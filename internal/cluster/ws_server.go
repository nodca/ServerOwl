package cluster

import (
	"context"
	"net/http"
	"time"

	"github.com/gorilla/websocket"

	"serverowl/internal/protocol"
)

const (
	// 写入超时
	writeWait = 10 * time.Second

	// 读取超时
	readWait = 60 * time.Second

	// Ping 间隔
	pingPeriod = 30 * time.Second

	// 最大消息大小
	maxMessageSize = 512 * 1024 // 512KB
)

// WSServer WebSocket 服务器
type WSServer struct {
	hub      *WSHub
	cluster  *MasterCluster
	upgrader websocket.Upgrader
}

// NewWSServer 创建 WebSocket 服务器
func NewWSServer(cluster *MasterCluster) *WSServer {
	hub := NewWSHub()

	server := &WSServer{
		hub:     hub,
		cluster: cluster,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true // 允许所有来源
			},
		},
	}

	// 设置回调
	hub.SetOnConnect(func(agentID string) {
		// Agent 连接时更新状态
		if agent, ok := cluster.GetAgent(agentID); ok {
			oldStatus := agent.Status
			agent.Status = "online"
			if cluster.onAgentStatusChange != nil && oldStatus != "online" {
				cluster.onAgentStatusChange(agent, oldStatus, "online")
			}
		}
	})

	hub.SetOnDisconnect(func(agentID string) {
		// Agent 断开时更新状态
		if agent, ok := cluster.GetAgent(agentID); ok {
			oldStatus := agent.Status
			agent.Status = "offline"
			if cluster.onAgentStatusChange != nil && oldStatus != "offline" {
				cluster.onAgentStatusChange(agent, oldStatus, "offline")
			}
		}
	})

	hub.SetOnMessage(func(agentID string, msg *protocol.WSMessage) {
		server.handleMessage(agentID, msg)
	})

	return server
}

// Start 启动 WebSocket 服务
func (s *WSServer) Start() {
	go s.hub.Run()
}

// GetHub 获取 Hub
func (s *WSServer) GetHub() *WSHub {
	return s.hub
}

// HandleConnection 处理 WebSocket 连接
func (s *WSServer) HandleConnection(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	// 等待注册消息
	conn.SetReadLimit(maxMessageSize)
	conn.SetReadDeadline(time.Now().Add(readWait))

	_, data, err := conn.ReadMessage()
	if err != nil {
		conn.Close()
		return
	}

	msg, err := protocol.DecodeMessage(data)
	if err != nil || msg.Type != protocol.MsgTypeRegister {
		conn.Close()
		return
	}

	var payload protocol.RegisterPayload
	if err := msg.ParsePayload(&payload); err != nil {
		conn.Close()
		return
	}

	// 创建客户端连接
	client := NewWSClientConn(payload.AgentID, conn, s.hub)

	// 注册到 Hub
	s.hub.Register(client)

	// 更新 Agent 信息到集群
	s.cluster.HandleHeartbeat(
		payload.AgentID,
		payload.Hostname,
		payload.IP,
		payload.Tags,
		payload.Labels,
		"online",
		nil,
	)

	// 发送确认消息
	ackMsg, _ := protocol.NewMessageWithID(msg.ID, protocol.MsgTypeAck, protocol.AckPayload{
		MessageID: msg.ID,
		Success:   true,
	})
	client.Send(ackMsg)

	// 启动读写协程
	go s.readPump(client)
	go s.writePump(client)
}

// readPump 读取消息循环
func (s *WSServer) readPump(client *WSClientConn) {
	defer func() {
		s.hub.Unregister(client)
	}()

	client.Conn.SetReadLimit(maxMessageSize)
	client.Conn.SetReadDeadline(time.Now().Add(readWait))
	client.Conn.SetPongHandler(func(string) error {
		client.Conn.SetReadDeadline(time.Now().Add(readWait))
		client.UpdateLastSeen()
		return nil
	})

	for {
		_, data, err := client.Conn.ReadMessage()
		if err != nil {
			break
		}

		client.UpdateLastSeen()

		msg, err := protocol.DecodeMessage(data)
		if err != nil {
			continue
		}

		s.hub.HandleMessage(client.AgentID, msg)
	}
}

// writePump 写入消息循环
func (s *WSServer) writePump(client *WSClientConn) {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		client.Close()
	}()

	for {
		select {
		case msg, ok := <-client.SendCh:
			client.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				client.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			data, err := msg.Encode()
			if err != nil {
				continue
			}

			if err := client.Conn.WriteMessage(websocket.TextMessage, data); err != nil {
				return
			}

		case <-ticker.C:
			client.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := client.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// handleMessage 处理收到的消息
func (s *WSServer) handleMessage(agentID string, msg *protocol.WSMessage) {
	switch msg.Type {
	case protocol.MsgTypeHeartbeat:
		s.handleHeartbeat(agentID, msg)

	case protocol.MsgTypeMetrics:
		s.handleMetrics(agentID, msg)

	case protocol.MsgTypeEvent:
		s.handleEvent(agentID, msg)

	case protocol.MsgTypeCommandResult:
		s.handleCommandResult(agentID, msg)

	case protocol.MsgTypeEnvironment:
		s.handleEnvironment(agentID, msg)

	case protocol.MsgTypePong:
		// Pong 消息，更新最后活跃时间
		if client, ok := s.hub.GetClient(agentID); ok {
			client.UpdateLastSeen()
		}
	}
}

// handleHeartbeat 处理心跳消息
func (s *WSServer) handleHeartbeat(agentID string, msg *protocol.WSMessage) {
	var payload protocol.HeartbeatPayload
	if err := msg.ParsePayload(&payload); err != nil {
		return
	}

	// 更新 Agent 状态
	if agent, ok := s.cluster.GetAgent(agentID); ok {
		agent.LastHeartbeat = time.Now()
		agent.Status = payload.Status
	}

	// 发送确认
	if client, ok := s.hub.GetClient(agentID); ok {
		ackMsg, _ := protocol.NewMessageWithID(msg.ID, protocol.MsgTypeAck, protocol.AckPayload{
			MessageID: msg.ID,
			Success:   true,
		})
		client.Send(ackMsg)
	}
}

// handleMetrics 处理指标消息
func (s *WSServer) handleMetrics(agentID string, msg *protocol.WSMessage) {
	var payload protocol.MetricsPayload
	if err := msg.ParsePayload(&payload); err != nil {
		return
	}

	// 更新 Agent 指标
	if agent, ok := s.cluster.GetAgent(agentID); ok {
		agent.LastMetrics = &AgentMetrics{
			CPUUsage:    payload.CPUUsage,
			MemoryUsage: payload.MemoryUsage,
			DiskUsage:   payload.DiskUsage,
			LoadAvg1:    payload.LoadAvg1,
			Uptime:      payload.Uptime,
			Timestamp:   time.Now(),
		}
		agent.LastHeartbeat = time.Now()
	}
}

// handleEvent 处理事件消息
func (s *WSServer) handleEvent(agentID string, msg *protocol.WSMessage) {
	var payload protocol.EventPayload
	if err := msg.ParsePayload(&payload); err != nil {
		return
	}

	event := &AgentEvent{
		AgentID:   agentID,
		RuleID:    payload.RuleID,
		RuleName:  payload.RuleName,
		Condition: payload.Condition,
		Action:    payload.Action,
		Success:   payload.Success,
		Output:    payload.Output,
		Error:     payload.Error,
		Timestamp: payload.Timestamp,
	}

	s.cluster.HandleAgentEvents(agentID, []*AgentEvent{event})
}

// handleCommandResult 处理命令结果消息
func (s *WSServer) handleCommandResult(agentID string, msg *protocol.WSMessage) {
	var payload protocol.CommandResultPayload
	if err := msg.ParsePayload(&payload); err != nil {
		return
	}

	result := &CommandResult{
		CommandID: payload.CommandID,
		AgentID:   agentID,
		Success:   payload.Success,
		Output:    payload.Output,
		Error:     payload.Error,
		ExitCode:  payload.ExitCode,
		Duration:  payload.Duration,
		Timestamp: time.Now(),
	}

	s.cluster.HandleCommandResult(result)
}

// SendCommand 发送命令到 Agent
func (s *WSServer) SendCommand(agentID string, cmdType, payload string, timeout time.Duration) (string, error) {
	cmdPayload := protocol.CommandPayload{
		CommandID: "cmd-" + time.Now().Format("20060102150405.000"),
		Type:      cmdType,
		Payload:   payload,
		Timeout:   timeout,
	}

	msg, err := protocol.NewMessage(protocol.MsgTypeCommand, cmdPayload)
	if err != nil {
		return "", err
	}

	if s.hub.SendToAgent(agentID, msg) {
		return cmdPayload.CommandID, nil
	}

	return "", nil
}

// SendCommandToAll 发送命令到所有 Agent
func (s *WSServer) SendCommandToAll(cmdType, payload string, timeout time.Duration) []string {
	var cmdIDs []string

	agents := s.hub.GetConnectedAgents()
	for _, agentID := range agents {
		cmdID, err := s.SendCommand(agentID, cmdType, payload, timeout)
		if err == nil && cmdID != "" {
			cmdIDs = append(cmdIDs, cmdID)
		}
	}

	return cmdIDs
}

// SendConfigUpdate 发送配置更新
func (s *WSServer) SendConfigUpdate(agentID string, config *protocol.ConfigUpdatePayload) error {
	msg, err := protocol.NewMessage(protocol.MsgTypeConfigUpdate, config)
	if err != nil {
		return err
	}

	s.hub.SendToAgent(agentID, msg)
	return nil
}

// BroadcastConfigUpdate 广播配置更新
func (s *WSServer) BroadcastConfigUpdate(config *protocol.ConfigUpdatePayload) error {
	msg, err := protocol.NewMessage(protocol.MsgTypeConfigUpdate, config)
	if err != nil {
		return err
	}

	s.hub.Broadcast(msg)
	return nil
}

// WaitCommandResult 等待命令结果
func (s *WSServer) WaitCommandResult(ctx context.Context, cmdID string) (*CommandResult, error) {
	return s.cluster.WaitCommandResult(ctx, cmdID)
}

// handleEnvironment 处理环境信息消息
func (s *WSServer) handleEnvironment(agentID string, msg *protocol.WSMessage) {
	var payload protocol.EnvironmentPayload
	if err := msg.ParsePayload(&payload); err != nil {
		return
	}

	env := &AgentEnvironment{
		UpdatedAt: payload.UpdatedAt,
		Host: EnvironmentHost{
			Hostname: payload.Host.Hostname,
			IP:       payload.Host.IP,
		},
		Containers: make(map[string]*ContainerInfo),
		Databases:  make(map[string]*DatabaseInfo),
		Proxies:    make(map[string]*ProxyInfo),
	}

	// 转换容器信息
	for name, c := range payload.Containers {
		env.Containers[name] = &ContainerInfo{
			Image:   c.Image,
			Status:  c.Status,
			Env:     c.Env,
			Ports:   c.Ports,
			Network: c.Network,
		}
	}

	// 转换数据库信息
	for name, db := range payload.Databases {
		env.Databases[name] = &DatabaseInfo{
			Type:     db.Type,
			Host:     db.Host,
			Port:     db.Port,
			User:     db.User,
			Password: db.Password,
			Database: db.Database,
			Source:   db.Source,
		}
	}

	// 转换代理信息
	for name, p := range payload.Proxies {
		proxy := &ProxyInfo{
			Type:       p.Type,
			ConfigFile: p.ConfigFile,
			Source:     p.Source,
		}
		for _, site := range p.Sites {
			proxy.Sites = append(proxy.Sites, SiteInfo{
				Domain:  site.Domain,
				Backend: site.Backend,
				Root:    site.Root,
			})
		}
		env.Proxies[name] = proxy
	}

	s.cluster.HandleAgentEnvironment(agentID, env)
}

// SendRefreshEnv 发送刷新环境请求到 Agent
func (s *WSServer) SendRefreshEnv(agentID string) error {
	msg, err := protocol.NewMessage(protocol.MsgTypeRefreshEnv, nil)
	if err != nil {
		return err
	}

	if s.hub.SendToAgent(agentID, msg) {
		return nil
	}
	return nil
}

// BroadcastRefreshEnv 广播刷新环境请求
func (s *WSServer) BroadcastRefreshEnv() {
	msg, _ := protocol.NewMessage(protocol.MsgTypeRefreshEnv, nil)
	s.hub.Broadcast(msg)
}
