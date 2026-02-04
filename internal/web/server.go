package web

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"runtime"
	"strings"
	"time"

	"serverowl/internal/agent"
	"serverowl/internal/cluster"
	"serverowl/internal/memory"
	"serverowl/internal/scheduler"
	"serverowl/internal/skills"
	"serverowl/internal/web/handlers"
	"serverowl/internal/web/middleware"
)

// Config Web 服务器配置
type Config struct {
	Port      int
	StaticDir string
	Auth      AuthConfig
	CORS      CORSConfig
}

// AuthConfig 认证配置
type AuthConfig struct {
	Enabled  bool
	Username string
	Password string
	Token    string
}

// CORSConfig CORS 配置
type CORSConfig struct {
	Enabled        bool
	AllowedOrigins []string
}

// Server HTTP 服务器
type Server struct {
	config          *Config
	server          *http.Server
	memoryManager   *memory.MemoryManager
	skillManager    *skills.Manager
	scheduler       *scheduler.Scheduler
	masterCluster   *cluster.MasterCluster
	agentExecutor   *agent.AgentExecutor
	actionLogger    *agent.ActionLogger
	envManager      *agent.EnvironmentManager
	agentAPIHandler *cluster.AgentAPIHandler
	startTime       time.Time
}

// NewServer 创建 Web 服务器
func NewServer(cfg *Config) *Server {
	return &Server{
		config:    cfg,
		startTime: time.Now(),
	}
}

// SetMemoryManager 设置记忆管理器
func (s *Server) SetMemoryManager(m *memory.MemoryManager) { s.memoryManager = m }

// SetSkillManager 设置技能管理器
func (s *Server) SetSkillManager(m *skills.Manager) { s.skillManager = m }

// SetScheduler 设置调度器
func (s *Server) SetScheduler(sch *scheduler.Scheduler) { s.scheduler = sch }

// SetMasterCluster 设置多主集群
func (s *Server) SetMasterCluster(mc *cluster.MasterCluster) { s.masterCluster = mc }

// SetAgentExecutor 设置 Agent 执行器
func (s *Server) SetAgentExecutor(e *agent.AgentExecutor) { s.agentExecutor = e }

// SetActionLogger 设置操作日志记录器
func (s *Server) SetActionLogger(l *agent.ActionLogger) { s.actionLogger = l }

// SetEnvironmentManager 设置环境管理器
func (s *Server) SetEnvironmentManager(e *agent.EnvironmentManager) { s.envManager = e }

// SetAgentAPIHandler 设置 Agent API 处理器
func (s *Server) SetAgentAPIHandler(h *cluster.AgentAPIHandler) { s.agentAPIHandler = h }

// Start 启动 HTTP 服务器
func (s *Server) Start() error {
	mux := http.NewServeMux()
	s.setupRoutes(mux)

	var handler http.Handler = mux

	if s.config.CORS.Enabled {
		handler = middleware.CORSMiddleware(handler)
	}

	if s.config.Auth.Enabled {
		handler = middleware.AuthMiddleware(handler, &middleware.AuthConfig{
			Username: s.config.Auth.Username,
			Password: s.config.Auth.Password,
			Token:    s.config.Auth.Token,
			SkipPaths: []string{
				"/",
				"/index.html",
				"/assets/",
				"/api/health",
			},
		})
	}

	handler = middleware.LoggingMiddleware(handler)

	addr := fmt.Sprintf(":%d", s.config.Port)
	s.server = &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("[Web] HTTP 服务器启动，监听地址: %s", addr)

	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("HTTP 服务器错误: %w", err)
	}
	return nil
}

// Stop 停止 HTTP 服务器
func (s *Server) Stop() error {
	if s.server == nil {
		return nil
	}

	log.Printf("[Web] 正在停止 HTTP 服务器...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := s.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("服务器关闭失败: %w", err)
	}

	log.Printf("[Web] HTTP 服务器已停止")
	return nil
}

// setupRoutes 配置路由
func (s *Server) setupRoutes(mux *http.ServeMux) {
	// 创建 handlers
	dashboardHandler := handlers.NewDashboardHandler(nil, s.scheduler, s.skillManager, s.memoryManager)
	skillsHandler := handlers.NewSkillsHandler(s.skillManager)
	tasksHandler := handlers.NewTasksHandler(s.scheduler)
	memoryHandler := handlers.NewMemoryHandler(s.memoryManager)
	logsHandler := handlers.NewLogsHandler(s.actionLogger)

	// API v1 路由
	mux.HandleFunc("/api/health", s.handleHealth)
	mux.HandleFunc("/api/v1/dashboard/stats", dashboardHandler.GetStats)
	mux.HandleFunc("/api/v1/dashboard/health", dashboardHandler.GetHealth)

	// 技能 API
	mux.HandleFunc("/api/v1/skills", skillsHandler.HandleSkills)
	mux.HandleFunc("/api/v1/skills/", skillsHandler.HandleSkillByName)

	// 任务 API
	mux.HandleFunc("/api/v1/tasks", tasksHandler.HandleTasks)
	mux.HandleFunc("/api/v1/tasks/", tasksHandler.HandleTaskByID)

	// 记忆 API
	mux.HandleFunc("/api/v1/memory/episodes", memoryHandler.HandleEpisodes)
	mux.HandleFunc("/api/v1/memory/episodes/", memoryHandler.HandleEpisodeByID)
	mux.HandleFunc("/api/v1/memory/knowledge", memoryHandler.HandleKnowledge)

	// 日志 API
	mux.HandleFunc("/api/v1/logs/actions", logsHandler.GetActionLogs)
	mux.HandleFunc("/api/v1/logs/analyze", logsHandler.AnalyzeLogs)

	// 环境 API
	mux.HandleFunc("/api/v1/environment", s.handleEnvironment)
	mux.HandleFunc("/api/v1/environment/refresh", s.handleEnvironmentRefresh)

	// 集群 API
	mux.HandleFunc("/api/v1/cluster/nodes", s.handleClusterNodes)
	mux.HandleFunc("/api/v1/cluster/execute", s.handleClusterExecute)

	// Chat API - 网页端发送自然语言指令
	mux.HandleFunc("/api/v1/chat", s.handleChat)

	// 兼容旧 API 路径
	mux.HandleFunc("/api/dashboard", s.handleDashboardCompat)
	mux.HandleFunc("/api/skills", s.handleSkillsCompat)
	mux.HandleFunc("/api/tasks", s.handleTasksCompat)
	mux.HandleFunc("/api/memory/episodes", s.handleMemoryEpisodesCompat)
	mux.HandleFunc("/api/memory/knowledge", s.handleMemoryKnowledgeCompat)
	mux.HandleFunc("/api/cluster/nodes", s.handleClusterNodes)

	// 静态文件服务
	staticDir := s.config.StaticDir
	if staticDir == "" {
		staticDir = "web/static"
	}
	fs := http.FileServer(http.Dir(staticDir))
	mux.Handle("/", s.spaHandler(fs, staticDir))
}

// spaHandler 处理 SPA 路由，对于不存在的路径返回 index.html
func (s *Server) spaHandler(fs http.Handler, staticDir string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// API 路径不处理
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		fs.ServeHTTP(w, r)
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.jsonResponse(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleChat 处理聊天请求 - 网页端发送自然语言指令
func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.jsonError(w, http.StatusMethodNotAllowed, "方法不允许")
		return
	}

	if s.agentExecutor == nil {
		s.jsonError(w, http.StatusServiceUnavailable, "Agent 系统未初始化")
		return
	}

	var req struct {
		Message   string `json:"message"`
		SessionID string `json:"session_id,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, http.StatusBadRequest, "无效的请求体")
		return
	}

	if req.Message == "" {
		s.jsonError(w, http.StatusBadRequest, "消息不能为空")
		return
	}

	// 生成 session ID 和 request ID
	sessionID := req.SessionID
	if sessionID == "" {
		sessionID = "web-" + generateID()
	}
	requestID := generateID()

	// 保存用户消息到记忆
	if s.memoryManager != nil && !agent.IsConfirmCancelOnly(req.Message) {
		s.memoryManager.SaveMessage(sessionID, "web-user", "user", req.Message)
	}

	// 执行 Agent
	startTime := time.Now()
	result, err := s.agentExecutor.Execute(sessionID, req.Message, requestID)
	duration := time.Since(startTime)

	if err != nil {
		log.Printf("[Chat] request_id=%s error=%v", requestID, err)
		s.jsonError(w, http.StatusInternalServerError, fmt.Sprintf("处理失败: %v", err))
		return
	}

	// 保存助手回复
	if s.memoryManager != nil {
		s.memoryManager.SaveMessage(sessionID, "web-user", "assistant", result)
	}

	log.Printf("[Chat] request_id=%s duration=%v", requestID, duration)

	s.jsonResponse(w, http.StatusOK, map[string]any{
		"success":    true,
		"message":    result,
		"session_id": sessionID,
		"request_id": requestID,
		"duration":   duration.String(),
	})
}

// 兼容旧 API - Dashboard
func (s *Server) handleDashboardCompat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.jsonError(w, http.StatusMethodNotAllowed, "方法不允许")
		return
	}

	// 计算运行时间
	uptime := time.Since(s.startTime)

	// 获取内存统计
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// 节点统计
	nodeStats := map[string]int{
		"total":   0,
		"online":  0,
		"offline": 0,
	}
	if s.masterCluster != nil {
		agents := s.masterCluster.ListAgents()
		nodeStats["total"] = len(agents)
		for _, a := range agents {
			if a.Status == "online" {
				nodeStats["online"]++
			} else {
				nodeStats["offline"]++
			}
		}
	}

	// 任务统计
	taskStats := map[string]int{
		"total":  0,
		"active": 0,
		"paused": 0,
	}
	if s.scheduler != nil {
		tasks := s.scheduler.ListTasks()
		taskStats["total"] = len(tasks)
		for _, t := range tasks {
			if t.Status == scheduler.TaskStatusActive {
				taskStats["active"]++
			} else if t.Status == scheduler.TaskStatusPaused {
				taskStats["paused"]++
			}
		}
	}

	// 技能统计
	skillCount := 0
	if s.skillManager != nil {
		skillCount = s.skillManager.SkillCount()
	}

	s.jsonResponse(w, http.StatusOK, map[string]any{
		"success": true,
		"data": map[string]any{
			"uptime":         uptime.String(),
			"uptime_seconds": int64(uptime.Seconds()),
			"go_version":     runtime.Version(),
			"num_goroutine":  runtime.NumGoroutine(),
			"num_cpu":        runtime.NumCPU(),
			"memory": map[string]any{
				"alloc":       memStats.Alloc,
				"total_alloc": memStats.TotalAlloc,
				"sys":         memStats.Sys,
				"num_gc":      memStats.NumGC,
			},
			"nodes":  nodeStats,
			"tasks":  taskStats,
			"skills": skillCount,
		},
	})
}

// 兼容旧 API - Skills
func (s *Server) handleSkillsCompat(w http.ResponseWriter, r *http.Request) {
	if s.skillManager == nil {
		s.jsonError(w, http.StatusServiceUnavailable, "技能管理器未初始化")
		return
	}

	skillList := s.skillManager.ListSkills()
	skills := make([]map[string]any, 0, len(skillList))
	for _, sk := range skillList {
		skills = append(skills, map[string]any{
			"name":        sk.Name,
			"description": sk.Description,
			"tags":        sk.Tags,
		})
	}

	s.jsonResponse(w, http.StatusOK, map[string]any{
		"success": true,
		"data":    skills,
	})
}

// 兼容旧 API - Tasks
func (s *Server) handleTasksCompat(w http.ResponseWriter, r *http.Request) {
	if s.scheduler == nil {
		s.jsonError(w, http.StatusServiceUnavailable, "调度器未初始化")
		return
	}

	taskList := s.scheduler.ListTasks()
	tasks := make([]map[string]any, 0, len(taskList))
	for _, t := range taskList {
		tasks = append(tasks, map[string]any{
			"id":          t.ID,
			"name":        t.Name,
			"description": t.Description,
			"schedule":    t.Schedule,
			"status":      string(t.Status),
			"last_run":    t.LastRun,
			"next_run":    t.NextRun,
		})
	}

	s.jsonResponse(w, http.StatusOK, map[string]any{
		"success": true,
		"data":    tasks,
	})
}

// 兼容旧 API - Memory Episodes
func (s *Server) handleMemoryEpisodesCompat(w http.ResponseWriter, r *http.Request) {
	if s.memoryManager == nil {
		s.jsonError(w, http.StatusServiceUnavailable, "记忆管理器未初始化")
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]any{
		"success": true,
		"data":    []any{},
	})
}

// 兼容旧 API - Memory Knowledge
func (s *Server) handleMemoryKnowledgeCompat(w http.ResponseWriter, r *http.Request) {
	if s.memoryManager == nil {
		s.jsonError(w, http.StatusServiceUnavailable, "记忆管理器未初始化")
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]any{
		"success": true,
		"data":    []any{},
	})
}

// 集群节点
func (s *Server) handleClusterNodes(w http.ResponseWriter, r *http.Request) {
	// 优先使用 MasterCluster（Agent WebSocket 模式）
	if s.masterCluster != nil {
		agents := s.masterCluster.ListAgents()
		nodeList := make([]map[string]any, 0, len(agents))
		for _, agent := range agents {
			node := map[string]any{
				"id":             agent.ID,
				"name":           agent.Hostname,
				"host":           agent.IP,
				"hostname":       agent.Hostname,
				"status":         agent.Status,
				"tags":           agent.Tags,
				"labels":         agent.Labels,
				"last_heartbeat": agent.LastHeartbeat,
				"registered_at":  agent.RegisteredAt,
			}
			// 添加指标数据
			if agent.LastMetrics != nil {
				node["metrics"] = map[string]any{
					"cpu":      agent.LastMetrics.CPUUsage,
					"memory":   agent.LastMetrics.MemoryUsage,
					"disk":     agent.LastMetrics.DiskUsage,
					"load_avg": agent.LastMetrics.LoadAvg1,
					"uptime":   agent.LastMetrics.Uptime,
				}
			} else {
				node["metrics"] = map[string]any{
					"cpu":    0,
					"memory": 0,
					"disk":   0,
				}
			}
			nodeList = append(nodeList, node)
		}

		s.jsonResponse(w, http.StatusOK, map[string]any{
			"success": true,
			"data":    nodeList,
		})
		return
	}

	// 没有集群管理器
	s.jsonError(w, http.StatusServiceUnavailable, "集群管理器未初始化")
}

// 集群执行命令
func (s *Server) handleClusterExecute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.jsonError(w, http.StatusMethodNotAllowed, "方法不允许")
		return
	}
	if s.masterCluster == nil {
		s.jsonError(w, http.StatusServiceUnavailable, "集群管理器未初始化")
		return
	}

	var req struct {
		NodeID  string `json:"node_id"`
		Command string `json:"command"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, http.StatusBadRequest, "无效的请求体")
		return
	}

	if req.NodeID == "" || req.Command == "" {
		s.jsonError(w, http.StatusBadRequest, "node_id 和 command 不能为空")
		return
	}

	// 通过 WebSocket 向 Agent 发送命令
	cmdID, err := s.masterCluster.SendCommand(req.NodeID, "shell", req.Command, 30*time.Second)
	if err != nil {
		s.jsonError(w, http.StatusInternalServerError, fmt.Sprintf("发送命令失败: %v", err))
		return
	}

	// 等待命令结果
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	result, err := s.masterCluster.WaitCommandResult(ctx, cmdID)
	if err != nil {
		s.jsonError(w, http.StatusInternalServerError, fmt.Sprintf("等待结果超时: %v", err))
		return
	}

	output := result.Output
	if !result.Success {
		output = result.Error
	}

	s.jsonResponse(w, http.StatusOK, map[string]any{
		"success": result.Success,
		"data": map[string]any{
			"node_id": req.NodeID,
			"command": req.Command,
			"output":  output,
		},
	})
}

func (s *Server) jsonResponse(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (s *Server) jsonError(w http.ResponseWriter, status int, message string) {
	s.jsonResponse(w, status, map[string]any{
		"success": false,
		"error":   message,
	})
}

func generateID() string {
	var b [8]byte
	rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// 环境信息 API
func (s *Server) handleEnvironment(w http.ResponseWriter, r *http.Request) {
	if s.envManager == nil {
		s.jsonError(w, http.StatusServiceUnavailable, "环境管理器未初始化")
		return
	}

	switch r.Method {
	case http.MethodGet:
		// 支持查询参数: ?all=true 返回所有节点, ?node=xxx 返回指定节点
		query := r.URL.Query()
		if query.Get("all") == "true" {
			// 返回所有节点的环境信息
			allNodes := s.envManager.GetAllNodes()
			s.jsonResponse(w, http.StatusOK, map[string]any{
				"success": true,
				"data":    allNodes,
			})
			return
		}

		// 默认返回本地节点
		env := s.envManager.Get()
		if env == nil {
			s.jsonError(w, http.StatusNotFound, "环境信息未加载")
			return
		}
		s.jsonResponse(w, http.StatusOK, map[string]any{
			"success": true,
			"data":    env,
		})

	case http.MethodPut:
		var req struct {
			Content string `json:"content"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.jsonError(w, http.StatusBadRequest, "无效的请求体")
			return
		}
		if err := s.envManager.SaveRaw(req.Content); err != nil {
			s.jsonError(w, http.StatusInternalServerError, fmt.Sprintf("保存失败: %v", err))
			return
		}
		s.jsonResponse(w, http.StatusOK, map[string]any{
			"success": true,
			"message": "环境配置已保存",
		})

	default:
		s.jsonError(w, http.StatusMethodNotAllowed, "方法不允许")
	}
}

// 刷新环境信息 API
func (s *Server) handleEnvironmentRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.jsonError(w, http.StatusMethodNotAllowed, "方法不允许")
		return
	}

	if s.envManager == nil {
		s.jsonError(w, http.StatusServiceUnavailable, "环境管理器未初始化")
		return
	}

	// 支持查询参数: ?node=xxx 刷新指定远程节点, ?all=true 刷新所有节点
	query := r.URL.Query()
	nodeID := query.Get("node")
	refreshAll := query.Get("all") == "true"

	if refreshAll {
		// 刷新本地
		if err := s.envManager.Refresh(); err != nil {
			s.jsonError(w, http.StatusInternalServerError, fmt.Sprintf("本地刷新失败: %v", err))
			return
		}
		// 广播刷新到所有远程节点
		if s.agentAPIHandler != nil {
			s.agentAPIHandler.BroadcastRefreshEnv()
		}
		s.jsonResponse(w, http.StatusOK, map[string]any{
			"success": true,
			"message": "已向所有节点发送刷新请求",
		})
		return
	}

	if nodeID != "" {
		// 刷新指定远程节点
		if s.agentAPIHandler == nil {
			s.jsonError(w, http.StatusServiceUnavailable, "Agent API 未初始化")
			return
		}
		if err := s.agentAPIHandler.SendRefreshEnv(nodeID); err != nil {
			s.jsonError(w, http.StatusInternalServerError, fmt.Sprintf("发送刷新请求失败: %v", err))
			return
		}
		s.jsonResponse(w, http.StatusOK, map[string]any{
			"success": true,
			"message": fmt.Sprintf("已向节点 %s 发送刷新请求", nodeID),
		})
		return
	}

	// 默认刷新本地
	if err := s.envManager.Refresh(); err != nil {
		s.jsonError(w, http.StatusInternalServerError, fmt.Sprintf("刷新失败: %v", err))
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "环境信息已刷新",
		"data":    s.envManager.Get(),
	})
}
