package agentnode

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/google/uuid"

	"serverowl/internal/protocol"
)

// Agent 轻量级 Agent
type Agent struct {
	config     *AgentConfig
	info       *AgentInfo
	metrics    *MetricsCollector
	ruleEngine *RuleEngine
	envScanner *EnvironmentScanner
	client     *http.Client
	wsClient   *WSClient
	logger     Logger

	// 状态
	mu            sync.RWMutex
	status        AgentStatus
	activeMaster  *MasterNode
	lastHeartbeat time.Time
	pendingEvents []*RuleEvent // 待上报的事件

	// 控制
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewAgent 创建 Agent
func NewAgent(config *AgentConfig, logger Logger) (*Agent, error) {
	if config.Agent.ID == "" {
		config.Agent.ID = uuid.New().String()
	}

	hostname, _ := os.Hostname()
	info := &AgentInfo{
		ID:        config.Agent.ID,
		Name:      config.Agent.Name,
		Hostname:  hostname,
		Tags:      config.Agent.Tags,
		Labels:    config.Agent.Labels,
		Version:   "1.0.0",
		StartTime: time.Now(),
		Status:    AgentStatusOffline,
	}

	// 获取本机 IP
	info.IP = getLocalIP()

	agent := &Agent{
		config:        config,
		info:          info,
		metrics:       NewMetricsCollector(&config.Metrics),
		ruleEngine:    NewRuleEngine(logger),
		envScanner:    NewEnvironmentScanner(""),
		client:        &http.Client{Timeout: 30 * time.Second},
		logger:        logger,
		status:        AgentStatusOffline,
		pendingEvents: make([]*RuleEvent, 0),
	}

	// 设置自治规则
	agent.ruleEngine.SetRules(config.GetEnabledRules())

	return agent, nil
}

// Start 启动 Agent
func (a *Agent) Start(ctx context.Context) error {
	a.ctx, a.cancel = context.WithCancel(ctx)

	a.logger.Info("Agent 启动: ID=%s, Name=%s", a.info.ID, a.info.Name)

	// 启动 WebSocket 客户端（如果启用）
	if a.config.WebSocket.Enabled {
		if err := a.startWebSocket(); err != nil {
			a.logger.Warn("WebSocket 启动失败，将使用 HTTP 模式: %v", err)
		}
	}

	// 启动心跳（HTTP 备选）
	a.wg.Add(1)
	go a.heartbeatLoop()

	// 启动指标收集
	if a.config.Metrics.Enabled {
		a.wg.Add(1)
		go a.metricsLoop()
	}

	// 启动规则引擎
	a.wg.Add(1)
	go a.ruleEngineLoop()

	// 启动事件收集
	a.wg.Add(1)
	go a.eventCollectorLoop()

	return nil
}

// startWebSocket 启动 WebSocket 客户端
func (a *Agent) startWebSocket() error {
	master := a.getActiveMaster()
	if master == nil {
		return fmt.Errorf("无可用主节点")
	}

	// 构建 WebSocket URL
	wsURL := fmt.Sprintf("ws://%s/ws/agent", master.Addr)

	wsConfig := &WSClientConfig{
		ServerURL:         wsURL,
		PingInterval:      a.config.WebSocket.PingInterval,
		ReconnectInterval: a.config.WebSocket.ReconnectInterval,
		WriteTimeout:      a.config.WebSocket.WriteTimeout,
		ReadTimeout:       a.config.WebSocket.ReadTimeout,
	}

	a.wsClient = NewWSClient(wsConfig, a.info, a.logger)

	// 注册消息处理器
	a.wsClient.RegisterHandler(protocol.MsgTypeCommand, a.handleWSCommand)
	a.wsClient.RegisterHandler(protocol.MsgTypeConfigUpdate, a.handleWSConfigUpdate)
	a.wsClient.RegisterHandler(protocol.MsgTypeRefreshEnv, a.handleWSRefreshEnv)

	// 启动 WebSocket 客户端
	if err := a.wsClient.Start(a.ctx); err != nil {
		return err
	}

	// 启动后上报环境信息
	go func() {
		time.Sleep(2 * time.Second) // 等待连接稳定
		a.reportEnvironment()
	}()

	return nil
}

// handleWSCommand 处理 WebSocket 命令消息
func (a *Agent) handleWSCommand(msg *protocol.WSMessage) {
	var payload protocol.CommandPayload
	if err := msg.ParsePayload(&payload); err != nil {
		a.logger.Error("解析命令消息失败: %v", err)
		return
	}

	cmd := &Command{
		ID:        payload.CommandID,
		Type:      CommandType(payload.Type),
		Payload:   payload.Payload,
		Timeout:   payload.Timeout,
		Priority:  payload.Priority,
		CreatedAt: time.Now(),
	}

	go a.executeCommand(cmd)
}

// handleWSConfigUpdate 处理 WebSocket 配置更新消息
func (a *Agent) handleWSConfigUpdate(msg *protocol.WSMessage) {
	var payload protocol.ConfigUpdatePayload
	if err := msg.ParsePayload(&payload); err != nil {
		a.logger.Error("解析配置更新消息失败: %v", err)
		return
	}

	update := &ConfigUpdate{
		Version:   payload.Version,
		UpdatedAt: time.Now(),
	}

	// 转换规则
	for _, r := range payload.AutoRules {
		rule := AutoRule{
			ID:          r.ID,
			Name:        r.Name,
			Description: r.Description,
			Enabled:     r.Enabled,
			Cooldown:    r.Cooldown,
			MaxRetries:  r.MaxRetries,
			Condition: RuleCondition{
				Type:      ConditionType(r.Condition.Type),
				Metric:    r.Condition.Metric,
				Operator:  r.Condition.Operator,
				Threshold: r.Condition.Threshold,
				Duration:  r.Condition.Duration,
			},
		}
		for _, action := range r.Actions {
			rule.Actions = append(rule.Actions, RuleAction{
				Type:    ActionType(action.Type),
				Command: action.Command,
				Timeout: action.Timeout,
			})
		}
		update.AutoRules = append(update.AutoRules, rule)
	}

	a.applyConfigUpdate(update)
}

// handleWSRefreshEnv 处理环境刷新请求
func (a *Agent) handleWSRefreshEnv(msg *protocol.WSMessage) {
	a.logger.Info("收到环境刷新请求")
	a.reportEnvironment()
}

// reportEnvironment 上报环境信息
func (a *Agent) reportEnvironment() {
	if a.wsClient == nil || !a.wsClient.IsConnected() {
		a.logger.Warn("WebSocket 未连接，无法上报环境信息")
		return
	}

	// 扫描环境
	env := a.envScanner.Scan()
	env.AgentID = a.info.ID

	// 发送环境信息
	if err := a.wsClient.SendEnvironment(env); err != nil {
		a.logger.Error("上报环境信息失败: %v", err)
	} else {
		a.logger.Info("环境信息已上报")
	}
}

// Stop 停止 Agent
func (a *Agent) Stop() {
	a.logger.Info("Agent 停止中...")

	// 停止 WebSocket 客户端
	if a.wsClient != nil {
		a.wsClient.Stop()
	}

	a.cancel()
	a.wg.Wait()
	a.logger.Info("Agent 已停止")
}

// heartbeatLoop 心跳循环
func (a *Agent) heartbeatLoop() {
	defer a.wg.Done()

	ticker := time.NewTicker(a.config.Agent.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			a.sendHeartbeat()
		}
	}
}

// sendHeartbeat 发送心跳
func (a *Agent) sendHeartbeat() {
	// 优先使用 WebSocket
	if a.wsClient != nil && a.wsClient.IsConnected() {
		status := string(a.status)
		if err := a.wsClient.SendHeartbeat(status); err != nil {
			a.logger.Warn("WebSocket 心跳发送失败: %v", err)
		} else {
			a.setStatus(AgentStatusOnline)
			a.lastHeartbeat = time.Now()
			return
		}
	}

	// 回退到 HTTP
	master := a.getActiveMaster()
	if master == nil {
		a.setStatus(AgentStatusAutonomous)
		a.logger.Warn("无可用主节点，进入自治模式")
		return
	}

	a.mu.RLock()
	metrics := a.ruleEngine.metrics
	a.mu.RUnlock()

	req := &HeartbeatRequest{
		AgentID:   a.info.ID,
		Timestamp: time.Now(),
		Status:    a.status,
		Metrics:   metrics,
	}

	resp, err := a.doHeartbeat(master, req)
	if err != nil {
		a.logger.Error("心跳失败: %v", err)
		a.tryFailover()
		return
	}

	a.setStatus(AgentStatusOnline)
	a.lastHeartbeat = time.Now()

	// 处理响应中的命令
	if len(resp.Commands) > 0 {
		for _, cmd := range resp.Commands {
			go a.executeCommand(cmd)
		}
	}

	// 处理配置更新
	if resp.ConfigUpdate != nil {
		a.applyConfigUpdate(resp.ConfigUpdate)
	}
}

// doHeartbeat 执行心跳请求
func (a *Agent) doHeartbeat(master *MasterNode, req *HeartbeatRequest) (*HeartbeatResponse, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("http://%s/api/agent/heartbeat", master.Addr)
	httpReq, err := http.NewRequestWithContext(a.ctx, "POST", url, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Agent-ID", a.info.ID)

	httpResp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(httpResp.Body)
		return nil, fmt.Errorf("心跳响应错误: %d, %s", httpResp.StatusCode, string(body))
	}

	var resp HeartbeatResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

// tryFailover 尝试故障转移
func (a *Agent) tryFailover() {
	a.mu.Lock()
	defer a.mu.Unlock()

	// 标记当前主节点为不活跃
	if a.activeMaster != nil {
		for i := range a.config.Masters {
			if a.config.Masters[i].ID == a.activeMaster.ID {
				a.config.Masters[i].IsActive = false
				break
			}
		}
	}

	// 尝试连接其他主节点
	for i := range a.config.Masters {
		master := &a.config.Masters[i]
		if master == a.activeMaster {
			continue
		}

		// 尝试连接
		url := fmt.Sprintf("http://%s/api/health", master.Addr)
		resp, err := a.client.Get(url)
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			master.IsActive = true
			a.activeMaster = master
			a.logger.Info("故障转移成功，切换到主节点: %s", master.Name)
			return
		}
		if resp != nil {
			resp.Body.Close()
		}
	}

	// 所有主节点都不可用，进入自治模式
	a.activeMaster = nil
	a.status = AgentStatusAutonomous
	a.logger.Warn("所有主节点不可用，进入自治模式")
}

// getActiveMaster 获取活跃主节点
func (a *Agent) getActiveMaster() *MasterNode {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.activeMaster != nil {
		return a.activeMaster
	}
	return a.config.GetActiveMaster()
}

// setStatus 设置状态
func (a *Agent) setStatus(status AgentStatus) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.status = status
	a.info.Status = status
}

// metricsLoop 指标收集循环
func (a *Agent) metricsLoop() {
	defer a.wg.Done()

	ticker := time.NewTicker(a.config.Metrics.CollectInterval)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			metrics := a.metrics.Collect()
			a.ruleEngine.UpdateMetrics(metrics)

			// 通过 WebSocket 发送指标
			if a.wsClient != nil && a.wsClient.IsConnected() {
				if err := a.wsClient.SendMetrics(metrics); err != nil {
					a.logger.Warn("WebSocket 指标发送失败: %v", err)
				}
			}
		}
	}
}

// ruleEngineLoop 规则引擎循环
func (a *Agent) ruleEngineLoop() {
	defer a.wg.Done()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			a.ruleEngine.Evaluate(a.ctx)
		}
	}
}

// eventCollectorLoop 事件收集循环
func (a *Agent) eventCollectorLoop() {
	defer a.wg.Done()

	for {
		select {
		case <-a.ctx.Done():
			return
		case event := <-a.ruleEngine.Events():
			a.mu.Lock()
			a.pendingEvents = append(a.pendingEvents, event)
			// 限制待上报事件数量
			if len(a.pendingEvents) > 1000 {
				a.pendingEvents = a.pendingEvents[len(a.pendingEvents)-1000:]
			}
			a.mu.Unlock()

			// 尝试立即上报
			go a.reportEvents()
		}
	}
}

// reportEvents 上报事件
func (a *Agent) reportEvents() {
	master := a.getActiveMaster()
	if master == nil {
		return
	}

	a.mu.Lock()
	if len(a.pendingEvents) == 0 {
		a.mu.Unlock()
		return
	}
	events := a.pendingEvents
	a.pendingEvents = make([]*RuleEvent, 0)
	a.mu.Unlock()

	data, err := json.Marshal(map[string]interface{}{
		"agent_id": a.info.ID,
		"events":   events,
	})
	if err != nil {
		a.mu.Lock()
		a.pendingEvents = append(events, a.pendingEvents...)
		a.mu.Unlock()
		return
	}

	url := fmt.Sprintf("http://%s/api/agent/events", master.Addr)
	req, err := http.NewRequestWithContext(a.ctx, "POST", url, bytes.NewReader(data))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Agent-ID", a.info.ID)

	resp, err := a.client.Do(req)
	if err != nil {
		// 上报失败，放回队列
		a.mu.Lock()
		a.pendingEvents = append(events, a.pendingEvents...)
		a.mu.Unlock()
		return
	}
	resp.Body.Close()
}

// executeCommand 执行命令
func (a *Agent) executeCommand(cmd *Command) {
	a.logger.Info("执行命令: ID=%s, Type=%s", cmd.ID, cmd.Type)

	result := &CommandResult{
		CommandID: cmd.ID,
		AgentID:   a.info.ID,
		Timestamp: time.Now(),
	}

	startTime := time.Now()
	timeout := cmd.Timeout
	if timeout == 0 {
		timeout = a.config.Agent.CommandTimeout
	}

	ctx, cancel := context.WithTimeout(a.ctx, timeout)
	defer cancel()

	switch cmd.Type {
	case CommandTypeShell:
		execCmd := exec.CommandContext(ctx, "sh", "-c", cmd.Payload)
		output, err := execCmd.CombinedOutput()
		result.Output = string(output)
		if err != nil {
			result.Success = false
			result.Error = err.Error()
			if exitErr, ok := err.(*exec.ExitError); ok {
				result.ExitCode = exitErr.ExitCode()
			}
		} else {
			result.Success = true
			result.ExitCode = 0
		}

	case CommandTypeSkill:
		// TODO: 集成 skill 执行
		result.Success = false
		result.Error = "skill 执行暂未实现"

	case CommandTypeShutdown:
		a.logger.Info("收到关闭命令")
		result.Success = true
		go func() {
			time.Sleep(time.Second)
			a.Stop()
		}()

	default:
		result.Success = false
		result.Error = fmt.Sprintf("未知命令类型: %s", cmd.Type)
	}

	result.Duration = time.Since(startTime)

	// 上报结果
	a.reportCommandResult(result)
}

// reportCommandResult 上报命令结果
func (a *Agent) reportCommandResult(result *CommandResult) {
	master := a.getActiveMaster()
	if master == nil {
		a.logger.Warn("无法上报命令结果，主节点不可用")
		return
	}

	data, err := json.Marshal(result)
	if err != nil {
		return
	}

	url := fmt.Sprintf("http://%s/api/agent/command/result", master.Addr)
	req, err := http.NewRequestWithContext(a.ctx, "POST", url, bytes.NewReader(data))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Agent-ID", a.info.ID)

	resp, err := a.client.Do(req)
	if err != nil {
		a.logger.Error("上报命令结果失败: %v", err)
		return
	}
	resp.Body.Close()
}

// applyConfigUpdate 应用配置更新
func (a *Agent) applyConfigUpdate(update *ConfigUpdate) {
	a.logger.Info("应用配置更新: version=%s", update.Version)

	if len(update.AutoRules) > 0 {
		a.config.Rules = update.AutoRules
		a.ruleEngine.SetRules(a.config.GetEnabledRules())
	}

	// 保存配置
	if a.config.Agent.DataDir != "" {
		configPath := a.config.Agent.DataDir + "/config.yaml"
		if err := a.config.Save(configPath); err != nil {
			a.logger.Error("保存配置失败: %v", err)
		}
	}
}

// GetInfo 获取 Agent 信息
func (a *Agent) GetInfo() *AgentInfo {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.info
}

// GetStatus 获取状态
func (a *Agent) GetStatus() AgentStatus {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.status
}

// getLocalIP 获取本机 IP
func getLocalIP() string {
	// 简单实现，获取第一个非回环 IP
	cmd := exec.Command("hostname", "-I")
	output, err := cmd.Output()
	if err != nil {
		return "127.0.0.1"
	}
	fields := bytes.Fields(output)
	if len(fields) > 0 {
		return string(fields[0])
	}
	return "127.0.0.1"
}
