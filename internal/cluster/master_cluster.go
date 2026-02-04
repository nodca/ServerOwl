package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// MasterRole 主节点角色
type MasterRole string

const (
	MasterRoleActive  MasterRole = "active"
	MasterRoleStandby MasterRole = "standby"
)

// MasterInfo 主节点信息
type MasterInfo struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Addr      string     `json:"addr"`
	Role      MasterRole `json:"role"`
	StartTime time.Time  `json:"start_time"`
	LastSeen  time.Time  `json:"last_seen"`
}

// AgentRecord Agent 记录
type AgentRecord struct {
	ID            string                 `json:"id"`
	Name          string                 `json:"name"`
	Hostname      string                 `json:"hostname"`
	IP            string                 `json:"ip"`
	Tags          []string               `json:"tags"`
	Labels        map[string]string      `json:"labels"`
	Status        string                 `json:"status"`
	Version       string                 `json:"version"`
	LastHeartbeat time.Time              `json:"last_heartbeat"`
	LastMetrics   *AgentMetrics          `json:"last_metrics,omitempty"`
	Environment   *AgentEnvironment      `json:"environment,omitempty"`
	RegisteredAt  time.Time              `json:"registered_at"`
}

// AgentEnvironment Agent 环境信息
type AgentEnvironment struct {
	UpdatedAt  string                    `json:"updated_at"`
	Host       EnvironmentHost           `json:"host"`
	Containers map[string]*ContainerInfo `json:"containers,omitempty"`
	Databases  map[string]*DatabaseInfo  `json:"databases,omitempty"`
	Proxies    map[string]*ProxyInfo     `json:"proxies,omitempty"`
}

// EnvironmentHost 主机信息
type EnvironmentHost struct {
	Hostname string `json:"hostname"`
	IP       string `json:"ip,omitempty"`
}

// ContainerInfo 容器信息
type ContainerInfo struct {
	Image   string            `json:"image"`
	Status  string            `json:"status"`
	Env     map[string]string `json:"env,omitempty"`
	Ports   []string          `json:"ports,omitempty"`
	Network string            `json:"network,omitempty"`
}

// DatabaseInfo 数据库信息
type DatabaseInfo struct {
	Type     string `json:"type"`
	Host     string `json:"host"`
	Port     string `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	Database string `json:"database,omitempty"`
	Source   string `json:"source"`
}

// ProxyInfo 代理信息
type ProxyInfo struct {
	Type       string     `json:"type"`
	ConfigFile string     `json:"config_file"`
	Sites      []SiteInfo `json:"sites,omitempty"`
	Source     string     `json:"source"`
}

// SiteInfo 站点信息
type SiteInfo struct {
	Domain  string `json:"domain"`
	Backend string `json:"backend,omitempty"`
	Root    string `json:"root,omitempty"`
}

// AgentMetrics Agent 上报的指标
type AgentMetrics struct {
	CPUUsage    float64   `json:"cpu_usage"`
	MemoryUsage float64   `json:"memory_usage"`
	DiskUsage   float64   `json:"disk_usage"`
	LoadAvg1    float64   `json:"load_avg_1"`
	Uptime      int64     `json:"uptime"`
	Timestamp   time.Time `json:"timestamp"`
}

// PendingCommand 待执行命令
type PendingCommand struct {
	ID        string        `json:"id"`
	AgentID   string        `json:"agent_id"`
	Type      string        `json:"type"`
	Payload   string        `json:"payload"`
	Timeout   time.Duration `json:"timeout"`
	CreatedAt time.Time     `json:"created_at"`
	ExpiresAt time.Time     `json:"expires_at"`
}

// CommandResult 命令结果
type CommandResult struct {
	CommandID string        `json:"command_id"`
	AgentID   string        `json:"agent_id"`
	Success   bool          `json:"success"`
	Output    string        `json:"output"`
	Error     string        `json:"error,omitempty"`
	ExitCode  int           `json:"exit_code"`
	Duration  time.Duration `json:"duration"`
	Timestamp time.Time     `json:"timestamp"`
}

// AgentEvent Agent 事件
type AgentEvent struct {
	AgentID   string    `json:"agent_id"`
	RuleID    string    `json:"rule_id"`
	RuleName  string    `json:"rule_name"`
	Condition string    `json:"condition"`
	Action    string    `json:"action"`
	Success   bool      `json:"success"`
	Output    string    `json:"output"`
	Error     string    `json:"error,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// MasterCluster 多主集群管理
type MasterCluster struct {
	self    *MasterInfo
	peers   map[string]*MasterInfo
	agents  map[string]*AgentRecord
	mu      sync.RWMutex

	// 待执行命令队列
	pendingCommands map[string][]*PendingCommand // agentID -> commands
	commandResults  map[string]*CommandResult    // commandID -> result
	commandMu       sync.RWMutex

	// 事件存储
	events   []*AgentEvent
	eventsMu sync.RWMutex

	// 状态同步
	syncInterval time.Duration
	client       *http.Client

	// 回调
	onAgentStatusChange func(agent *AgentRecord, oldStatus, newStatus string)
	onAgentEvent        func(event *AgentEvent)

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewMasterCluster 创建主节点集群
func NewMasterCluster(selfID, selfName, selfAddr string) *MasterCluster {
	return &MasterCluster{
		self: &MasterInfo{
			ID:        selfID,
			Name:      selfName,
			Addr:      selfAddr,
			Role:      MasterRoleActive,
			StartTime: time.Now(),
			LastSeen:  time.Now(),
		},
		peers:           make(map[string]*MasterInfo),
		agents:          make(map[string]*AgentRecord),
		pendingCommands: make(map[string][]*PendingCommand),
		commandResults:  make(map[string]*CommandResult),
		events:          make([]*AgentEvent, 0),
		syncInterval:    5 * time.Second,
		client:          &http.Client{Timeout: 10 * time.Second},
	}
}

// Start 启动集群管理
func (mc *MasterCluster) Start(ctx context.Context) {
	mc.ctx, mc.cancel = context.WithCancel(ctx)

	// 启动 Agent 健康检查
	mc.wg.Add(1)
	go mc.agentHealthCheckLoop()

	// 启动主节点同步
	mc.wg.Add(1)
	go mc.peerSyncLoop()

	// 启动命令过期清理
	mc.wg.Add(1)
	go mc.commandCleanupLoop()
}

// Stop 停止集群管理
func (mc *MasterCluster) Stop() {
	mc.cancel()
	mc.wg.Wait()
}

// AddPeer 添加对等主节点
func (mc *MasterCluster) AddPeer(peer *MasterInfo) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.peers[peer.ID] = peer
}

// RemovePeer 移除对等主节点
func (mc *MasterCluster) RemovePeer(peerID string) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	delete(mc.peers, peerID)
}

// HandleHeartbeat 处理 Agent 心跳
func (mc *MasterCluster) HandleHeartbeat(agentID string, hostname, ip string, tags []string, labels map[string]string, status string, metrics *AgentMetrics) ([]*PendingCommand, error) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	agent, exists := mc.agents[agentID]
	oldStatus := ""
	if exists {
		oldStatus = agent.Status
	}

	if !exists {
		agent = &AgentRecord{
			ID:           agentID,
			Hostname:     hostname,
			IP:           ip,
			Tags:         tags,
			Labels:       labels,
			RegisteredAt: time.Now(),
		}
		mc.agents[agentID] = agent
	}

	agent.Hostname = hostname
	agent.IP = ip
	agent.Tags = tags
	agent.Labels = labels
	agent.Status = status
	agent.LastHeartbeat = time.Now()

	if metrics != nil {
		agent.LastMetrics = metrics
	}

	// 触发状态变化回调
	if mc.onAgentStatusChange != nil && oldStatus != "" && oldStatus != status {
		go mc.onAgentStatusChange(agent, oldStatus, status)
	}

	// 获取待执行命令
	mc.commandMu.Lock()
	commands := mc.pendingCommands[agentID]
	mc.pendingCommands[agentID] = nil
	mc.commandMu.Unlock()

	return commands, nil
}

// HandleCommandResult 处理命令结果
func (mc *MasterCluster) HandleCommandResult(result *CommandResult) {
	mc.commandMu.Lock()
	defer mc.commandMu.Unlock()
	mc.commandResults[result.CommandID] = result
}

// HandleAgentEvents 处理 Agent 事件
func (mc *MasterCluster) HandleAgentEvents(agentID string, events []*AgentEvent) {
	mc.eventsMu.Lock()
	defer mc.eventsMu.Unlock()

	for _, event := range events {
		event.AgentID = agentID
		mc.events = append(mc.events, event)

		// 触发事件回调
		if mc.onAgentEvent != nil {
			go mc.onAgentEvent(event)
		}
	}

	// 限制事件数量
	if len(mc.events) > 10000 {
		mc.events = mc.events[len(mc.events)-10000:]
	}
}

// HandleAgentEnvironment 处理 Agent 环境信息
func (mc *MasterCluster) HandleAgentEnvironment(agentID string, env *AgentEnvironment) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	agent, exists := mc.agents[agentID]
	if !exists {
		// Agent 不存在，创建一个基本记录
		agent = &AgentRecord{
			ID:           agentID,
			Hostname:     env.Host.Hostname,
			IP:           env.Host.IP,
			Status:       "online",
			RegisteredAt: time.Now(),
		}
		mc.agents[agentID] = agent
	}

	agent.Environment = env
}

// GetAgentEnvironment 获取 Agent 环境信息
func (mc *MasterCluster) GetAgentEnvironment(agentID string) (*AgentEnvironment, bool) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	agent, exists := mc.agents[agentID]
	if !exists || agent.Environment == nil {
		return nil, false
	}
	return agent.Environment, true
}

// GetAllEnvironments 获取所有 Agent 的环境信息
func (mc *MasterCluster) GetAllEnvironments() map[string]*AgentEnvironment {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	envs := make(map[string]*AgentEnvironment)
	for agentID, agent := range mc.agents {
		if agent.Environment != nil {
			envs[agentID] = agent.Environment
		}
	}
	return envs
}

// SendCommand 发送命令到 Agent
func (mc *MasterCluster) SendCommand(agentID, cmdType, payload string, timeout time.Duration) (string, error) {
	mc.mu.RLock()
	_, exists := mc.agents[agentID]
	mc.mu.RUnlock()

	if !exists {
		return "", fmt.Errorf("agent not found: %s", agentID)
	}

	cmdID := fmt.Sprintf("cmd-%d", time.Now().UnixNano())
	cmd := &PendingCommand{
		ID:        cmdID,
		AgentID:   agentID,
		Type:      cmdType,
		Payload:   payload,
		Timeout:   timeout,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(5 * time.Minute),
	}

	mc.commandMu.Lock()
	mc.pendingCommands[agentID] = append(mc.pendingCommands[agentID], cmd)
	mc.commandMu.Unlock()

	return cmdID, nil
}

// GetCommandResult 获取命令结果
func (mc *MasterCluster) GetCommandResult(cmdID string) (*CommandResult, bool) {
	mc.commandMu.RLock()
	defer mc.commandMu.RUnlock()
	result, ok := mc.commandResults[cmdID]
	return result, ok
}

// WaitCommandResult 等待命令结果
func (mc *MasterCluster) WaitCommandResult(ctx context.Context, cmdID string) (*CommandResult, error) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			if result, ok := mc.GetCommandResult(cmdID); ok {
				return result, nil
			}
		}
	}
}

// ListAgents 列出所有 Agent
func (mc *MasterCluster) ListAgents() []*AgentRecord {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	agents := make([]*AgentRecord, 0, len(mc.agents))
	for _, agent := range mc.agents {
		agents = append(agents, agent)
	}
	return agents
}

// GetAgent 获取 Agent
func (mc *MasterCluster) GetAgent(agentID string) (*AgentRecord, bool) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	agent, ok := mc.agents[agentID]
	return agent, ok
}

// GetAgentsByTag 按标签获取 Agent
func (mc *MasterCluster) GetAgentsByTag(tag string) []*AgentRecord {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	var agents []*AgentRecord
	for _, agent := range mc.agents {
		for _, t := range agent.Tags {
			if t == tag {
				agents = append(agents, agent)
				break
			}
		}
	}
	return agents
}

// GetOnlineAgents 获取在线 Agent
func (mc *MasterCluster) GetOnlineAgents() []*AgentRecord {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	var agents []*AgentRecord
	for _, agent := range mc.agents {
		if agent.Status == "online" {
			agents = append(agents, agent)
		}
	}
	return agents
}

// SetAgentStatusChangeCallback 设置 Agent 状态变化回调
func (mc *MasterCluster) SetAgentStatusChangeCallback(callback func(agent *AgentRecord, oldStatus, newStatus string)) {
	mc.onAgentStatusChange = callback
}

// SetAgentEventCallback 设置 Agent 事件回调
func (mc *MasterCluster) SetAgentEventCallback(callback func(event *AgentEvent)) {
	mc.onAgentEvent = callback
}

// agentHealthCheckLoop Agent 健康检查循环
func (mc *MasterCluster) agentHealthCheckLoop() {
	defer mc.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-mc.ctx.Done():
			return
		case <-ticker.C:
			mc.checkAgentHealth()
		}
	}
}

// checkAgentHealth 检查 Agent 健康状态
func (mc *MasterCluster) checkAgentHealth() {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	timeout := 60 * time.Second
	for _, agent := range mc.agents {
		if agent.Status == "online" && time.Since(agent.LastHeartbeat) > timeout {
			oldStatus := agent.Status
			agent.Status = "offline"
			if mc.onAgentStatusChange != nil {
				go mc.onAgentStatusChange(agent, oldStatus, agent.Status)
			}
		}
	}
}

// peerSyncLoop 主节点同步循环
func (mc *MasterCluster) peerSyncLoop() {
	defer mc.wg.Done()

	ticker := time.NewTicker(mc.syncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-mc.ctx.Done():
			return
		case <-ticker.C:
			mc.syncWithPeers()
		}
	}
}

// syncWithPeers 与对等节点同步
func (mc *MasterCluster) syncWithPeers() {
	mc.mu.RLock()
	peers := make([]*MasterInfo, 0, len(mc.peers))
	for _, peer := range mc.peers {
		peers = append(peers, peer)
	}
	mc.mu.RUnlock()

	for _, peer := range peers {
		mc.syncWithPeer(peer)
	}
}

// syncWithPeer 与单个对等节点同步
func (mc *MasterCluster) syncWithPeer(peer *MasterInfo) {
	// 发送心跳
	url := fmt.Sprintf("http://%s/api/master/heartbeat", peer.Addr)
	data, _ := json.Marshal(mc.self)

	req, err := http.NewRequestWithContext(mc.ctx, "POST", url, nil)
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := mc.client.Do(req)
	if err != nil {
		mc.mu.Lock()
		peer.LastSeen = time.Time{} // 标记为不可达
		mc.mu.Unlock()
		return
	}
	resp.Body.Close()

	mc.mu.Lock()
	peer.LastSeen = time.Now()
	mc.mu.Unlock()

	// 同步 Agent 列表
	mc.syncAgentsFromPeer(peer, data)
}

// syncAgentsFromPeer 从对等节点同步 Agent 列表
func (mc *MasterCluster) syncAgentsFromPeer(peer *MasterInfo, _ []byte) {
	url := fmt.Sprintf("http://%s/api/master/agents", peer.Addr)
	req, err := http.NewRequestWithContext(mc.ctx, "GET", url, nil)
	if err != nil {
		return
	}

	resp, err := mc.client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	var agents []*AgentRecord
	if err := json.NewDecoder(resp.Body).Decode(&agents); err != nil {
		return
	}

	// 合并 Agent 信息（以最新心跳为准）
	mc.mu.Lock()
	defer mc.mu.Unlock()

	for _, remoteAgent := range agents {
		localAgent, exists := mc.agents[remoteAgent.ID]
		if !exists || remoteAgent.LastHeartbeat.After(localAgent.LastHeartbeat) {
			mc.agents[remoteAgent.ID] = remoteAgent
		}
	}
}

// commandCleanupLoop 命令清理循环
func (mc *MasterCluster) commandCleanupLoop() {
	defer mc.wg.Done()

	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-mc.ctx.Done():
			return
		case <-ticker.C:
			mc.cleanupExpiredCommands()
		}
	}
}

// cleanupExpiredCommands 清理过期命令
func (mc *MasterCluster) cleanupExpiredCommands() {
	mc.commandMu.Lock()
	defer mc.commandMu.Unlock()

	now := time.Now()

	// 清理待执行命令
	for agentID, commands := range mc.pendingCommands {
		var valid []*PendingCommand
		for _, cmd := range commands {
			if cmd.ExpiresAt.After(now) {
				valid = append(valid, cmd)
			}
		}
		mc.pendingCommands[agentID] = valid
	}

	// 清理旧结果（保留 1 小时）
	for cmdID, result := range mc.commandResults {
		if time.Since(result.Timestamp) > time.Hour {
			delete(mc.commandResults, cmdID)
		}
	}
}

// GetSelf 获取自身信息
func (mc *MasterCluster) GetSelf() *MasterInfo {
	return mc.self
}

// GetPeers 获取对等节点
func (mc *MasterCluster) GetPeers() []*MasterInfo {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	peers := make([]*MasterInfo, 0, len(mc.peers))
	for _, peer := range mc.peers {
		peers = append(peers, peer)
	}
	return peers
}

// GetRecentEvents 获取最近事件
func (mc *MasterCluster) GetRecentEvents(limit int) []*AgentEvent {
	mc.eventsMu.RLock()
	defer mc.eventsMu.RUnlock()

	if limit <= 0 || limit > len(mc.events) {
		limit = len(mc.events)
	}

	// 返回最新的事件
	start := len(mc.events) - limit
	if start < 0 {
		start = 0
	}

	events := make([]*AgentEvent, limit)
	copy(events, mc.events[start:])
	return events
}
