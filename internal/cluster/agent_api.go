package cluster

import (
	"encoding/json"
	"net/http"
	"time"
)

// AgentAPIHandler Agent API 处理器
type AgentAPIHandler struct {
	cluster  *MasterCluster
	wsServer *WSServer
}

// NewAgentAPIHandler 创建 Agent API 处理器
func NewAgentAPIHandler(cluster *MasterCluster) *AgentAPIHandler {
	wsServer := NewWSServer(cluster)
	wsServer.Start()

	return &AgentAPIHandler{
		cluster:  cluster,
		wsServer: wsServer,
	}
}

// GetWSServer 获取 WebSocket 服务器
func (h *AgentAPIHandler) GetWSServer() *WSServer {
	return h.wsServer
}

// RegisterRoutes 注册路由
func (h *AgentAPIHandler) RegisterRoutes(mux *http.ServeMux) {
	// WebSocket 端点
	mux.HandleFunc("/ws/agent", h.handleWebSocket)

	// HTTP API（保留作为备选）
	mux.HandleFunc("/api/agent/heartbeat", h.handleHeartbeat)
	mux.HandleFunc("/api/agent/command/result", h.handleCommandResult)
	mux.HandleFunc("/api/agent/events", h.handleEvents)
	mux.HandleFunc("/api/health", h.handleHealth)

	// 主节点同步 API
	mux.HandleFunc("/api/master/heartbeat", h.handleMasterHeartbeat)
	mux.HandleFunc("/api/master/agents", h.handleMasterAgents)
	mux.HandleFunc("/api/master/forward-command", h.handleForwardCommand)
	mux.HandleFunc("/api/master/command-result", h.handlePeerCommandResult)

	// 管理 API
	mux.HandleFunc("/api/agents", h.handleListAgents)
	mux.HandleFunc("/api/agents/command", h.handleSendCommand)
	mux.HandleFunc("/api/agents/cleanup", h.handleCleanupOffline)
}

// handleWebSocket 处理 WebSocket 连接
func (h *AgentAPIHandler) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	h.wsServer.HandleConnection(w, r)
}

// HeartbeatRequest 心跳请求
type HeartbeatRequest struct {
	AgentID   string        `json:"agent_id"`
	Hostname  string        `json:"hostname"`
	IP        string        `json:"ip"`
	Tags      []string      `json:"tags"`
	Labels    map[string]string `json:"labels"`
	Status    string        `json:"status"`
	Metrics   *AgentMetrics `json:"metrics,omitempty"`
	Timestamp time.Time     `json:"timestamp"`
}

// HeartbeatResponse 心跳响应
type HeartbeatResponse struct {
	Success   bool              `json:"success"`
	MasterID  string            `json:"master_id"`
	Timestamp time.Time         `json:"timestamp"`
	Commands  []*PendingCommand `json:"commands,omitempty"`
}

func (h *AgentAPIHandler) handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req HeartbeatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	commands, err := h.cluster.HandleHeartbeat(
		req.AgentID,
		req.Hostname,
		req.IP,
		req.Tags,
		req.Labels,
		req.Status,
		req.Metrics,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp := HeartbeatResponse{
		Success:   true,
		MasterID:  h.cluster.GetSelf().ID,
		Timestamp: time.Now(),
		Commands:  commands,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *AgentAPIHandler) handleCommandResult(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var result CommandResult
	if err := json.NewDecoder(r.Body).Decode(&result); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.cluster.HandleCommandResult(&result)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// EventsRequest 事件请求
type EventsRequest struct {
	AgentID string        `json:"agent_id"`
	Events  []*AgentEvent `json:"events"`
}

func (h *AgentAPIHandler) handleEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req EventsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.cluster.HandleAgentEvents(req.AgentID, req.Events)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func (h *AgentAPIHandler) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "ok",
		"master_id": h.cluster.GetSelf().ID,
		"role":      h.cluster.GetSelf().Role,
		"timestamp": time.Now(),
	})
}

func (h *AgentAPIHandler) handleMasterHeartbeat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var peer MasterInfo
	if err := json.NewDecoder(r.Body).Decode(&peer); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	peer.LastSeen = time.Now()
	h.cluster.AddPeer(&peer)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(h.cluster.GetSelf())
}

func (h *AgentAPIHandler) handleMasterAgents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	agents := h.cluster.ListAgents()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(agents)
}

func (h *AgentAPIHandler) handleListAgents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tag := r.URL.Query().Get("tag")
	status := r.URL.Query().Get("status")

	var agents []*AgentRecord
	if tag != "" {
		agents = h.cluster.GetAgentsByTag(tag)
	} else if status == "online" {
		agents = h.cluster.GetOnlineAgents()
	} else {
		agents = h.cluster.ListAgents()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(agents)
}

// SendCommandRequest 发送命令请求
type SendCommandRequest struct {
	AgentID string `json:"agent_id"`
	Tag     string `json:"tag"`     // 按标签发送
	Type    string `json:"type"`    // shell, skill
	Payload string `json:"payload"` // 命令内容
	Timeout int    `json:"timeout"` // 超时秒数
}

// SendCommandResponse 发送命令响应
type SendCommandResponse struct {
	Success    bool     `json:"success"`
	CommandIDs []string `json:"command_ids"`
	Error      string   `json:"error,omitempty"`
}

func (h *AgentAPIHandler) handleSendCommand(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req SendCommandRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	timeout := time.Duration(req.Timeout) * time.Second
	if timeout == 0 {
		timeout = 5 * time.Minute
	}

	var commandIDs []string
	var agents []*AgentRecord

	if req.AgentID != "" {
		agent, ok := h.cluster.GetAgent(req.AgentID)
		if ok {
			agents = []*AgentRecord{agent}
		}
	} else if req.Tag != "" {
		agents = h.cluster.GetAgentsByTag(req.Tag)
	}

	for _, agent := range agents {
		// 优先使用 WebSocket 发送命令（即时送达）
		if h.wsServer != nil && h.wsServer.GetHub().IsConnected(agent.ID) {
			cmdID, err := h.wsServer.SendCommand(agent.ID, req.Type, req.Payload, timeout)
			if err == nil && cmdID != "" {
				commandIDs = append(commandIDs, cmdID)
				continue
			}
		}

		// 回退到 HTTP 轮询方式
		cmdID, err := h.cluster.SendCommand(agent.ID, req.Type, req.Payload, timeout)
		if err == nil {
			commandIDs = append(commandIDs, cmdID)
		}
	}

	resp := SendCommandResponse{
		Success:    len(commandIDs) > 0,
		CommandIDs: commandIDs,
	}
	if len(commandIDs) == 0 {
		resp.Error = "no agents found or command send failed"
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// SendRefreshEnv 发送刷新环境请求到指定 Agent
func (h *AgentAPIHandler) SendRefreshEnv(agentID string) error {
	if h.wsServer == nil {
		return nil
	}
	return h.wsServer.SendRefreshEnv(agentID)
}

// BroadcastRefreshEnv 广播刷新环境请求到所有 Agent
func (h *AgentAPIHandler) BroadcastRefreshEnv() {
	if h.wsServer != nil {
		h.wsServer.BroadcastRefreshEnv()
	}
}

// handleCleanupOffline 清理离线节点
func (h *AgentAPIHandler) handleCleanupOffline(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	count := h.cluster.RemoveOfflineAgents()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"removed": count,
		"message": "离线节点已清理",
	})
}

// handleForwardCommand 处理来自对端 Master 的命令转发
func (h *AgentAPIHandler) handleForwardCommand(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ForwardCommandRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	timeout := time.Duration(req.Timeout) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	// 优先通过 WebSocket 发送
	if h.wsServer != nil && h.wsServer.GetHub().IsConnected(req.AgentID) {
		cmdID, err := h.wsServer.SendCommand(req.AgentID, req.Type, req.Payload, timeout)
		if err == nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(ForwardCommandResponse{Success: true, CommandID: cmdID})
			return
		}
	}

	// 回退到 pendingCommands
	cmdID, err := h.cluster.SendCommand(req.AgentID, req.Type, req.Payload, timeout)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ForwardCommandResponse{Success: false, Error: err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ForwardCommandResponse{Success: true, CommandID: cmdID})
}

// handlePeerCommandResult 返回指定 commandID 的结果（供对端 Master 轮询）
func (h *AgentAPIHandler) handlePeerCommandResult(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cmdID := r.URL.Query().Get("cmd_id")
	if cmdID == "" {
		http.Error(w, "cmd_id required", http.StatusBadRequest)
		return
	}

	result, ok := h.cluster.GetCommandResult(cmdID)
	if !ok {
		// 尚无结果，返回空对象
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(CommandResult{})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
