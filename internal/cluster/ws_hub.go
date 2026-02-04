package cluster

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"serverowl/internal/protocol"
)

// WSHub WebSocket 连接管理中心
type WSHub struct {
	// 已注册的客户端连接
	clients map[string]*WSClientConn

	// 注册请求通道
	register chan *WSClientConn

	// 注销请求通道
	unregister chan *WSClientConn

	// 广播消息通道
	broadcast chan *protocol.WSMessage

	// 互斥锁
	mu sync.RWMutex

	// 回调函数
	onConnect    func(agentID string)
	onDisconnect func(agentID string)
	onMessage    func(agentID string, msg *protocol.WSMessage)
}

// WSClientConn WebSocket 客户端连接
type WSClientConn struct {
	AgentID  string
	Conn     *websocket.Conn
	SendCh   chan *protocol.WSMessage
	LastSeen time.Time

	closed bool
	mu     sync.Mutex
}

// NewWSHub 创建 WebSocket Hub
func NewWSHub() *WSHub {
	return &WSHub{
		clients:    make(map[string]*WSClientConn),
		register:   make(chan *WSClientConn, 100),
		unregister: make(chan *WSClientConn, 100),
		broadcast:  make(chan *protocol.WSMessage, 100),
	}
}

// Run 运行 Hub 主循环
func (h *WSHub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			// 如果已存在旧连接，先关闭
			if old, ok := h.clients[client.AgentID]; ok {
				old.Close()
			}
			h.clients[client.AgentID] = client
			h.mu.Unlock()

			if h.onConnect != nil {
				go h.onConnect(client.AgentID)
			}

		case client := <-h.unregister:
			h.mu.Lock()
			if existing, ok := h.clients[client.AgentID]; ok && existing == client {
				delete(h.clients, client.AgentID)
				client.Close()
			}
			h.mu.Unlock()

			if h.onDisconnect != nil {
				go h.onDisconnect(client.AgentID)
			}

		case msg := <-h.broadcast:
			h.mu.RLock()
			for _, client := range h.clients {
				select {
				case client.SendCh <- msg:
				default:
					// 发送通道已满，跳过
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Register 注册客户端
func (h *WSHub) Register(client *WSClientConn) {
	h.register <- client
}

// Unregister 注销客户端
func (h *WSHub) Unregister(client *WSClientConn) {
	h.unregister <- client
}

// Broadcast 广播消息
func (h *WSHub) Broadcast(msg *protocol.WSMessage) {
	h.broadcast <- msg
}

// SendToAgent 发送消息到指定 Agent
func (h *WSHub) SendToAgent(agentID string, msg *protocol.WSMessage) bool {
	h.mu.RLock()
	client, ok := h.clients[agentID]
	h.mu.RUnlock()

	if !ok {
		return false
	}

	select {
	case client.SendCh <- msg:
		return true
	default:
		return false
	}
}

// GetClient 获取客户端连接
func (h *WSHub) GetClient(agentID string) (*WSClientConn, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	client, ok := h.clients[agentID]
	return client, ok
}

// GetConnectedAgents 获取所有已连接的 Agent ID
func (h *WSHub) GetConnectedAgents() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	agents := make([]string, 0, len(h.clients))
	for agentID := range h.clients {
		agents = append(agents, agentID)
	}
	return agents
}

// IsConnected 检查 Agent 是否已连接
func (h *WSHub) IsConnected(agentID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, ok := h.clients[agentID]
	return ok
}

// ClientCount 获取连接数
func (h *WSHub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// SetOnConnect 设置连接回调
func (h *WSHub) SetOnConnect(callback func(agentID string)) {
	h.onConnect = callback
}

// SetOnDisconnect 设置断开回调
func (h *WSHub) SetOnDisconnect(callback func(agentID string)) {
	h.onDisconnect = callback
}

// SetOnMessage 设置消息回调
func (h *WSHub) SetOnMessage(callback func(agentID string, msg *protocol.WSMessage)) {
	h.onMessage = callback
}

// HandleMessage 处理收到的消息
func (h *WSHub) HandleMessage(agentID string, msg *protocol.WSMessage) {
	if h.onMessage != nil {
		h.onMessage(agentID, msg)
	}
}

// NewWSClientConn 创建客户端连接
func NewWSClientConn(agentID string, conn *websocket.Conn, hub *WSHub) *WSClientConn {
	return &WSClientConn{
		AgentID:  agentID,
		Conn:     conn,
		SendCh:   make(chan *protocol.WSMessage, 256),
		LastSeen: time.Now(),
	}
}

// Close 关闭连接
func (c *WSClientConn) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return
	}
	c.closed = true

	close(c.SendCh)
	c.Conn.Close()
}

// IsClosed 检查是否已关闭
func (c *WSClientConn) IsClosed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed
}

// UpdateLastSeen 更新最后活跃时间
func (c *WSClientConn) UpdateLastSeen() {
	c.LastSeen = time.Now()
}

// Send 发送消息
func (c *WSClientConn) Send(msg *protocol.WSMessage) bool {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return false
	}
	c.mu.Unlock()

	select {
	case c.SendCh <- msg:
		return true
	default:
		return false
	}
}
