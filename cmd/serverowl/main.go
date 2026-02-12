package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"serverowl/internal/agent"
	"serverowl/internal/cluster"
	"serverowl/internal/config"
	"serverowl/internal/llm"
	"serverowl/internal/logging"
	"serverowl/internal/mcp"
	"serverowl/internal/memory"
	"serverowl/internal/monitor"
	"serverowl/internal/notifier"
	"serverowl/internal/scheduler"
	"serverowl/internal/skills"
	"serverowl/internal/skills/builtin"
	"serverowl/internal/web"
	"serverowl/internal/wecom"

	_ "github.com/lib/pq"
)

// llmAdapter 适配 llm.Client 到 memory.LLMClient
type llmAdapter struct {
	client llm.Client
}

func (a *llmAdapter) Chat(prompt string) (string, error) {
	return a.client.Chat(prompt)
}

func main() {
	configPath := flag.String("config", "configs/config.yaml", "path to config yaml")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	// 初始化结构化日志系统
	logging.Init(&logging.Config{
		Level:    cfg.Logging.Level,
		Format:   cfg.Logging.Format,
		Output:   cfg.Logging.Output,
		Sanitize: cfg.Logging.Sanitize,
	})
	slog := logging.Default()
	slog.Info().Str("config", *configPath).Msg("configuration loaded")

	// 保留旧的 logger 用于兼容
	logger := log.New(os.Stdout, "serverowl ", log.LstdFlags|log.Lmicroseconds)

	// 初始化 LLM 客户端
	var llmClient llm.Client
	if cfg.LLM.ApiKey != "" {
		ds := llm.NewDeepSeekClient(cfg.LLM.ApiKey, cfg.LLM.BaseURL, cfg.LLM.Model)
		ds.SetTimeout(cfg.LLM.Timeout)
		ds.SetRetries(cfg.LLM.Retries)
		llmClient = ds
		logger.Printf("LLM client initialized: %s", cfg.LLM.Model)
	}

	// 初始化 MemoryManager (PostgreSQL)
	var memManager *memory.MemoryManager
	if cfg.Memory.Postgres.DSN != "" {
		// 解析配置
		shortTermTTL, _ := time.ParseDuration(cfg.Memory.ShortTerm.MaxAge)
		if shortTermTTL == 0 {
			shortTermTTL = 30 * time.Minute
		}
		forgetMinAge, _ := time.ParseDuration(cfg.Memory.Forget.MinAge)
		if forgetMinAge == 0 {
			forgetMinAge = 7 * 24 * time.Hour
		}

		managerCfg := &memory.ManagerConfig{
			PostgresDSN:            cfg.Memory.Postgres.DSN,
			ShortTermMaxTurns:      cfg.Memory.ShortTerm.MaxTurns,
			ShortTermTTL:           shortTermTTL,
			MaxEpisodes:            cfg.Memory.LongTerm.MaxEpisodes,
			ForgetThreshold:        cfg.Memory.Forget.Threshold,
			ForgetMinAge:           forgetMinAge,
			EmbeddingAPIKey:        cfg.Memory.Embedding.APIKey,
			EmbeddingBaseURL:       cfg.Memory.Embedding.BaseURL,
			EmbeddingModel:         cfg.Memory.Embedding.Model,
			ConsolidateMinEpisodes: cfg.Memory.Summary.ConsolidateMinEpisodes,
		}

		// 创建 LLM 适配器用于记忆整合
		var memLLM memory.LLMClient
		if llmClient != nil {
			memLLM = &llmAdapter{client: llmClient}
		}

		memManager, err = memory.NewMemoryManager(managerCfg, memLLM)
		if err != nil {
			logger.Printf("memory manager init failed: %v", err)
		} else {
			logger.Printf("memory manager initialized (PostgreSQL)")
			defer memManager.Close()

			// 启动定期维护任务
			go func() {
				ticker := time.NewTicker(24 * time.Hour)
				defer ticker.Stop()
				for range ticker.C {
					// 执行遗忘清理
					if deleted, err := memManager.RunForgetGate(); err != nil {
						logger.Printf("forget gate error: %v", err)
					} else if deleted > 0 {
						logger.Printf("forget gate cleaned %d episodes", deleted)
					}
					// 执行记忆整合
					if consolidated, err := memManager.RunConsolidation(); err != nil {
						logger.Printf("consolidation error: %v", err)
					} else if consolidated > 0 {
						logger.Printf("consolidated %d knowledge entries", consolidated)
					}
				}
			}()
		}
	}

	var n *notifier.WeChatNotifier
	if cfg.WeChat.CorpID != "" && cfg.WeChat.AgentID != 0 && cfg.WeChat.Secret != "" {
		n = notifier.NewWeChatNotifier(cfg.WeChat.CorpID, cfg.WeChat.AgentID, cfg.WeChat.Secret)
	}

	if n != nil {
		// 创建监控器
		mon := monitor.NewMonitor(&cfg.Monitor, n, cfg.WeChat.DefaultUser)

		// 启动前先执行一次检查，确认功能正常
		logger.Printf("running initial health check...")
		if err := mon.RunChecks(); err != nil {
			logger.Printf("initial check error: %v", err)
		}

		// 启动定时任务
		logger.Printf("starting monitor with interval: %s", cfg.Monitor.Interval)
		if err := mon.Start(); err != nil {
			logger.Fatalf("start monitor: %v", err)
		}
	}

	// 创建 ActionLogger
	actionLogger, err := agent.NewActionLogger(cfg.Memory.Postgres.DSN)
	if err != nil {
		log.Fatal(err)
	}
	// 启动自动清理
	if cfg.Agent.LogCleanupSchedule != "" {
		cronJob, err := actionLogger.StartAutoCleanup(
			cfg.Agent.LogRetentionDays,
			cfg.Agent.LogCleanupSchedule,
		)
		if err != nil {
			log.Printf("警告: 无法启动日志自动清理: %v", err)
		} else {
			defer cronJob.Stop()
			log.Printf("✅ 日志自动清理已启动: 保留 %d 天，计划: %s",
				cfg.Agent.LogRetentionDays, cfg.Agent.LogCleanupSchedule)
		}
	}

	// 初始化技能系统
	var skillManager *skills.Manager
	if cfg.Skills.Enabled {
		// 创建技能管理器
		skillDirs := cfg.Skills.Dirs
		if len(skillDirs) == 0 {
			skillDirs = []string{"skills"} // 默认目录
		}
		skillManager = skills.NewManager(skillDirs...)

		// 注册内置技能
		builtin.RegisterAll(skillManager.Registry())
		slog.Info().Int("builtin_count", skillManager.SkillCount()).Msg("built-in skills registered")

		// 自动加载 YAML 技能
		if cfg.Skills.AutoLoad {
			if err := skillManager.LoadSkills(); err != nil {
				slog.Warn().Err(err).Msg("failed to load skills from directories")
			} else {
				slog.Info().Int("total_count", skillManager.SkillCount()).Msg("skills loaded")
			}
		}
	}

	// 初始化调度器
	var taskScheduler *scheduler.Scheduler
	if cfg.Scheduler.Enabled && cfg.Memory.Postgres.DSN != "" {
		// 打开数据库连接
		schedulerDB, err := sql.Open("postgres", cfg.Memory.Postgres.DSN)
		if err != nil {
			slog.Warn().Err(err).Msg("failed to open scheduler database")
		} else {
			store, err := scheduler.NewPostgresStore(schedulerDB)
			if err != nil {
				slog.Warn().Err(err).Msg("failed to create scheduler store")
				schedulerDB.Close()
			} else {
				// 创建技能执行器适配器
				var executor scheduler.TaskExecutor
				if skillManager != nil {
					executor = &skillExecutorAdapter{manager: skillManager}
				}

				taskScheduler = scheduler.New(store, executor)

				// 加载预定义任务
				for _, task := range cfg.Scheduler.Tasks {
					if !task.Enabled {
						continue
					}
					scheduledTask := &scheduler.ScheduledTask{
						ID:          task.ID,
						Name:        task.Name,
						Description: task.Description,
						Schedule:    task.Schedule,
						Type:        scheduler.TaskType(task.Type),
						Config: scheduler.TaskConfig{
							SkillName:   task.SkillName,
							SkillParams: task.SkillParams,
							Command:     task.Command,
						},
						Status: scheduler.TaskStatusActive,
					}
					if err := taskScheduler.AddTask(scheduledTask); err != nil {
						slog.Warn().Str("task_id", task.ID).Err(err).Msg("failed to add scheduled task")
					}
				}

				// 启动调度器
				taskScheduler.Start()
				defer taskScheduler.Stop()
				slog.Info().Int("task_count", len(cfg.Scheduler.Tasks)).Msg("scheduler started")
			}
		}
	}

	// 初始化多主集群（Agent 模式）
	var masterCluster *cluster.MasterCluster
	if cfg.MasterCluster.Enabled {
		masterCluster = cluster.NewMasterCluster(
			cfg.MasterCluster.ID,
			cfg.MasterCluster.Name,
			cfg.MasterCluster.Addr,
		)

		// 添加对等主节点
		for _, peerCfg := range cfg.MasterCluster.Peers {
			peer := &cluster.MasterInfo{
				ID:   peerCfg.ID,
				Name: peerCfg.Name,
				Addr: peerCfg.Addr,
				Role: cluster.MasterRoleStandby,
			}
			masterCluster.AddPeer(peer)
		}

		// 设置回调
		masterCluster.SetAgentStatusChangeCallback(func(agent *cluster.AgentRecord, oldStatus, newStatus string) {
			slog.Info().
				Str("agent_id", agent.ID).
				Str("hostname", agent.Hostname).
				Str("old_status", oldStatus).
				Str("new_status", newStatus).
				Msg("agent status changed")

			// 可以在这里发送通知
			if n != nil && newStatus == "offline" {
				_ = n.SendText(cfg.WeChat.DefaultUser, fmt.Sprintf("⚠️ Agent 离线: %s (%s)", agent.Hostname, agent.IP))
			}
		})

		masterCluster.SetAgentEventCallback(func(event *cluster.AgentEvent) {
			slog.Info().
				Str("agent_id", event.AgentID).
				Str("rule_name", event.RuleName).
				Bool("success", event.Success).
				Msg("agent event received")
		})

		// 启动集群
		masterCluster.Start(context.Background())
		defer masterCluster.Stop()
		slog.Info().
			Str("master_id", cfg.MasterCluster.ID).
			Int("peer_count", len(cfg.MasterCluster.Peers)).
			Msg("master cluster started")
	}

	// MCP 服务器变量（在 Agent 初始化后创建）
	var mcpServer *mcp.MCPServer

	// 初始化环境管理器
	var envManager *agent.EnvironmentManager
	envFilePath := filepath.Join(filepath.Dir(*configPath), "environment.yaml")
	envManager = agent.NewEnvironmentManager(envFilePath, *configPath)
	if err := envManager.Load(); err != nil {
		slog.Warn().Err(err).Msg("failed to load environment info")
	} else {
		slog.Info().Str("file", envFilePath).Msg("environment info loaded")
	}

	// 关联 MasterCluster 到环境管理器（支持多节点环境查询）
	if masterCluster != nil && envManager != nil {
		envManager.SetMasterCluster(masterCluster, cfg.MasterCluster.ID)
		slog.Info().Msg("environment manager linked to master cluster")
	}

	// 初始化 Agent 系统
	var agentExecutor *agent.AgentExecutor
	var cmdAdapter *clusterCommanderAdapter
	if masterCluster != nil {
		cmdAdapter = &clusterCommanderAdapter{mc: masterCluster}
	}
	if llmClient != nil && cfg.Agent.WhitelistPath != "" {
		// 创建 SafetyChecker
		safetyChecker, err := agent.NewSafetyChecker(cfg.Agent.WhitelistPath)
		if err != nil {
			logger.Printf("⚠️  SafetyChecker 初始化失败: %v", err)
		} else {
			// 创建工具注册表
			registry := agent.NewToolRegistry()
			registry.Register(agent.CreateDockerTool())
			registry.Register(agent.CreateLocalShellTool())
			registry.Register(agent.CreateRemoteShellTool(cmdAdapter))
			// 注册日志分析工具
			registry.Register(agent.CreateAnalyzeLogsTool())
			// 注册环境查询工具
			if envManager != nil {
				registry.Register(agent.CreateEnvironmentTool(envManager))
			}
			// 注册记忆检索工具（如果记忆系统已启用）
			if memManager != nil {
				registry.Register(agent.CreateRecallMemoryTool(memManager))
			}
			// 注册技能工具（如果技能系统已启用）
			if skillManager != nil {
				registry.Register(agent.CreateSkillTool(skillManager))
				registry.Register(agent.CreateListSkillsTool(skillManager))
			}

			toolCount := registry.Count()
			slog.Info().Int("tool_count", toolCount).Msg("agent system initialized")

			// 创建 AgentExecutor
			agentExecutor = agent.NewAgentExecutor(llmClient, registry, safetyChecker, actionLogger, memManager)
			if cmdAdapter != nil {
				agentExecutor.SetClusterCommander(cmdAdapter)
			}

			// 创建 MCP 服务器（需要 registry）
			if cfg.MCP.Enabled {
				mcpServer = mcp.NewMCPServer(registry, mcp.WithServerInfo("serverowl", "1.0.0"))
				slog.Info().Msg("MCP server initialized with tool registry")
			}
		}
	}

	// 启动 MCP 服务器
	if mcpServer != nil {
		switch cfg.MCP.Transport {
		case "http":
			httpTransport := mcp.NewHTTPTransport(mcpServer, fmt.Sprintf(":%d", cfg.MCP.HTTPPort))
			go func() {
				if err := httpTransport.Start(context.Background()); err != nil {
					slog.Error().Err(err).Msg("MCP HTTP server error")
				}
			}()
			slog.Info().Int("port", cfg.MCP.HTTPPort).Msg("MCP HTTP server started")
		case "stdio":
			// stdio 模式需要单独启动，这里只记录
			slog.Info().Msg("MCP stdio transport configured (start with --mcp-stdio flag)")
		}
	}

	// 启动 Web 管理面板
	var webServer *web.Server
	var agentAPI *cluster.AgentAPIHandler

	// 创建 Agent API 处理器（需要在 Web 服务器之前创建）
	if masterCluster != nil {
		agentAPI = cluster.NewAgentAPIHandler(masterCluster)
		if cmdAdapter != nil {
			cmdAdapter.agentAPI = agentAPI
		}
	}

	if cfg.Web.Enabled {
		webCfg := &web.Config{
			Port:      cfg.Web.Port,
			StaticDir: cfg.Web.StaticDir,
			Auth: web.AuthConfig{
				Enabled:  cfg.Web.Auth.Enabled,
				Username: cfg.Web.Auth.Username,
				Password: cfg.Web.Auth.Password,
				Token:    cfg.Web.Auth.Token,
			},
			CORS: web.CORSConfig{
				Enabled:        cfg.Web.CORS.Enabled,
				AllowedOrigins: cfg.Web.CORS.AllowedOrigins,
			},
		}

		webServer = web.NewServer(webCfg)

		// 注入依赖
		webServer.SetMemoryManager(memManager)
		webServer.SetSkillManager(skillManager)
		webServer.SetScheduler(taskScheduler)
		webServer.SetMasterCluster(masterCluster)
		webServer.SetAgentExecutor(agentExecutor)
		webServer.SetActionLogger(actionLogger)
		webServer.SetEnvironmentManager(envManager)
		webServer.SetAgentAPIHandler(agentAPI)

		// 启动 Web 服务器
		go func() {
			if err := webServer.Start(); err != nil {
				slog.Error().Err(err).Msg("Web server error")
			}
		}()
		slog.Info().Int("port", cfg.Web.Port).Msg("Web management panel started")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// 注册 Agent API 路由（多主集群模式）
	if agentAPI != nil {
		agentAPI.RegisterRoutes(mux)
		slog.Info().Msg("agent API routes registered")
	}

	if cfg.WeChat.CorpID != "" && cfg.WeChat.Token != "" && cfg.WeChat.EncodingAESKey != "" {
		crypto, err := wecom.NewCrypto(cfg.WeChat.Token, cfg.WeChat.EncodingAESKey, cfg.WeChat.CorpID)
		if err != nil {
			logger.Printf("wecom callback disabled (invalid wechat settings): %v", err)
		} else {
			cb := &wecom.CallbackHandler{
				Crypto: crypto,
				Logf:   logger.Printf,
				OnText: func(fromUserID, content string) {
					requestID := newRequestID()
					startTime := time.Now()
					logger.Printf("wecom text from=%s content=%q", fromUserID, content)
					if n == nil {
						return
					}

					// 使用 userID 作为 sessionID
					sessionID := fromUserID

					// 保存用户消息到记忆（跳过确认/取消指令，避免污染历史）
					if memManager != nil && !agent.IsConfirmCancelOnly(content) {
						memManager.SaveMessage(sessionID, fromUserID, "user", content)
					}

					// 判断使用哪种模式
					// 目前统一使用 Agent（LLM）+ 快速路径（无需 LLM）
					var result string

					// 确认/取消待执行动作（不走 LLM）
					if agentExecutor != nil {
						handled, reply, _, err := agentExecutor.TryHandleConfirmCancel(fromUserID, sessionID, requestID, content)
						if err != nil {
							logger.Printf("confirm/cancel handler error: %v", err)
						} else if handled {
							result = reply
							logger.Printf("request_id=%s handled=confirm_cancel cost_ms=%d", requestID, time.Since(startTime).Milliseconds())
						}
					}

					useAgent := (agentExecutor != nil)

					if result == "" && useAgent {
						// 使用 Agent 模式
						logger.Printf("request_id=%s mode=agent", requestID)
						agentStart := time.Now()
						res, err := agentExecutor.Execute(sessionID, content, requestID)
						if err != nil {
							logger.Printf("agent execute error: %v", err)
							result = fmt.Sprintf("❌ 处理失败: %v", err)
						} else {
							result = res
						}
						logger.Printf("request_id=%s mode=agent cost_ms=%d", requestID, time.Since(agentStart).Milliseconds())
					} else {
						if result == "" {
							// 降级：简单回显
							result = fmt.Sprintf("收到：%s", content)
						}
					}
					// 保存助手回复
					if memManager != nil {
						memManager.SaveMessage(sessionID, fromUserID, "assistant", result)
					}

					// 返回结果
					if err := n.SendText(fromUserID, result); err != nil {
						logger.Printf("wecom reply failed to=%s err=%v", fromUserID, err)
					} else {
						logger.Printf("request_id=%s total_cost_ms=%d", requestID, time.Since(startTime).Milliseconds())
					}
				},
			}
			mux.Handle("/wecom/callback-ccgj123123", cb)
			logger.Printf("wecom callback enabled at %s", "/wecom/callback")
		}
	} else {
		logger.Printf("wecom callback disabled (missing wechat.corp_id / wechat.token / wechat.encoding_aes_key)")
	}

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	logger.Printf("listening on %s", srv.Addr)
	logger.Fatal(srv.ListenAndServe())
}

func newRequestID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// skillExecutorAdapter 适配 skills.Manager 到 scheduler.TaskExecutor
type skillExecutorAdapter struct {
	manager *skills.Manager
}

func (a *skillExecutorAdapter) ExecuteTask(task *scheduler.ScheduledTask) (*scheduler.TaskResult, error) {
	startTime := time.Now()
	result := &scheduler.TaskResult{
		TaskID:    task.ID,
		StartTime: startTime,
	}

	var err error
	var output string

	switch task.Type {
	case scheduler.TaskTypeSkill:
		if a.manager == nil {
			err = fmt.Errorf("skill manager not available")
		} else {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			skillResult, execErr := a.manager.Execute(ctx, task.Config.SkillName, task.Config.SkillParams)
			if execErr != nil {
				err = execErr
			} else if skillResult != nil {
				if skillResult.Success {
					output = fmt.Sprintf("Skill %s completed successfully", task.Config.SkillName)
				} else {
					err = fmt.Errorf("skill failed: %s", skillResult.Error)
				}
			}
		}
	case scheduler.TaskTypeCommand:
		err = fmt.Errorf("command execution not implemented")
	case scheduler.TaskTypeWorkflow:
		err = fmt.Errorf("workflow execution not implemented")
	default:
		err = fmt.Errorf("unknown task type: %s", task.Type)
	}

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(startTime)

	if err != nil {
		result.Success = false
		result.Error = err.Error()
		return result, err
	}

	result.Success = true
	result.Output = output
	return result, nil
}

type clusterCommanderAdapter struct {
	mc       *cluster.MasterCluster
	agentAPI *cluster.AgentAPIHandler
}

func (a *clusterCommanderAdapter) ExecuteOnAgent(host, command string) (string, error) {
	agentID, found := a.mc.FindAgentByHost(host)
	if !found {
		// 本地找不到，尝试转发到 peer
		return a.forwardToPeer(host, command)
	}

	// 检查本地是否有 WebSocket 连接，没有则转发到 peer
	localConnected := a.agentAPI != nil &&
		a.agentAPI.GetWSServer() != nil &&
		a.agentAPI.GetWSServer().GetHub().IsConnected(agentID)
	if !localConnected {
		return a.forwardToPeer(agentID, command)
	}

	cmdID, err := a.agentAPI.GetWSServer().SendCommand(agentID, "shell", command, 30*time.Second)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	result, err := a.mc.WaitCommandResult(ctx, cmdID)
	if err != nil {
		return "", err
	}
	if !result.Success {
		return "", fmt.Errorf("%s", result.Error)
	}
	return result.Output, nil
}

func (a *clusterCommanderAdapter) forwardToPeer(agentID, command string) (string, error) {
	peerAddr, remoteCmdID, err := a.mc.ForwardCommandToPeers(agentID, "shell", command, 30*time.Second)
	if err != nil {
		return "", fmt.Errorf("agent not found or no peer available: %s", agentID)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	result, err := a.mc.PollPeerCommandResult(ctx, peerAddr, remoteCmdID)
	if err != nil {
		return "", err
	}
	if !result.Success {
		return "", fmt.Errorf("%s", result.Error)
	}
	return result.Output, nil
}

func (a *clusterCommanderAdapter) ListOnlineAgents() []agent.AgentInfo {
	agents := a.mc.GetOnlineAgents()
	infos := make([]agent.AgentInfo, len(agents))
	for i, ag := range agents {
		infos[i] = agent.AgentInfo{
			ID:       ag.ID,
			Hostname: ag.Hostname,
			IP:       ag.IP,
			Status:   ag.Status,
		}
	}
	return infos
}
