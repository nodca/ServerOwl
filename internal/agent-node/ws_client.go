package agentnode

import (
	"context"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"serverowl/internal/protocol"
)

// WSClientConfig WebSocket 客户端配置
type WSClientConfig struct {
	ServerURL            string
	PingInterval         time.Duration
	ReconnectInterval    time.Duration
	WriteTimeout         time.Duration
	ReadTimeout          time.Duration
	MaxReconnectAttempts int
}

// DefaultWSClientConfig 返回默认配置
func DefaultWSClientConfig() *WSClientConfig {
	return &WSClientConfig{
		PingInterval:         30 * time.Second,
		ReconnectInterval:    5 * time.Second,
		WriteTimeout:         10 * time.Second,
		ReadTimeout:          60 * time.Second,
		MaxReconnectAttempts: 0,
	}
}

// MessageHandler 消息处理函数
type MessageHandler func(msg *protocol.WSMessage)

// WSClient WebSocket 客户端
type WSClient struct {
	config    *WSClientConfig
	agentInfo *AgentInfo
	conn      *websocket.Conn
	sendCh    chan *protocol.WSMessage
	handlers  map[string]MessageHandler
	logger    Logger

	mu           sync.RWMutex
	connected    bool
	reconnecting bool
	reconnectCnt int

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewWSClient 创建 WebSocket 客户端
func NewWSClient(config *WSClientConfig, agentInfo *AgentInfo, logger Logger) *WSClient {
	if config == nil {
		config = DefaultWSClientConfig()
	}

	return &WSClient{
		config:    config,
		agentInfo: agentInfo,
		sendCh:    make(chan *protocol.WSMessage, 256),
		handlers:  make(map[string]MessageHandler),
		logger:    logger,
	}
}

// RegisterHandler 注册消息处理器
func (c *WSClient) RegisterHandler(msgType string, handler MessageHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.handlers[msgType] = handler
}

// Start 启动客户端
func (c *WSClient) Start(ctx context.Context) error {
	c.ctx, c.cancel = context.WithCancel(ctx)

	// 首次连接
	if err := c.connect(); err != nil {
		c.logger.Warn("首次连接失败，将在后台重试: %v", err)
	}

	// 启动重连循环
	c.wg.Add(1)
	go c.reconnectLoop()

	return nil
}

// Stop 停止客户端
func (c *WSClient) Stop() {
	c.cancel()
	c.wg.Wait()

	c.mu.Lock()
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
	c.connected = false
	c.mu.Unlock()
}

// Send 发送消息
func (c *WSClient) Send(msg *protocol.WSMessage) error {
	select {
	case c.sendCh <- msg:
		return nil
	case <-c.ctx.Done():
		return c.ctx.Err()
	default:
		return nil // 通道满时丢弃
	}
}

// IsConnected 检查是否已连接
func (c *WSClient) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// connect 建立连接
func (c *WSClient) connect() error {
	c.mu.Lock()
	if c.connected {
		c.mu.Unlock()
		return nil
	}
	c.mu.Unlock()

	// 建立 WebSocket 连接
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.DialContext(c.ctx, c.config.ServerURL, nil)
	if err != nil {
		return err
	}

	// 发送注册消息
	registerPayload := protocol.RegisterPayload{
		AgentID:   c.agentInfo.ID,
		Name:      c.agentInfo.Name,
		Hostname:  c.agentInfo.Hostname,
		IP:        c.agentInfo.IP,
		Tags:      c.agentInfo.Tags,
		Labels:    c.agentInfo.Labels,
		Version:   c.agentInfo.Version,
		StartTime: c.agentInfo.StartTime,
	}

	msg, err := protocol.NewMessage(protocol.MsgTypeRegister, registerPayload)
	if err != nil {
		conn.Close()
		return err
	}

	data, err := msg.Encode()
	if err != nil {
		conn.Close()
		return err
	}

	conn.SetWriteDeadline(time.Now().Add(c.config.WriteTimeout))
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		conn.Close()
		return err
	}

	// 等待确认
	conn.SetReadDeadline(time.Now().Add(c.config.ReadTimeout))
	_, respData, err := conn.ReadMessage()
	if err != nil {
		conn.Close()
		return err
	}

	respMsg, err := protocol.DecodeMessage(respData)
	if err != nil || respMsg.Type != protocol.MsgTypeAck {
		conn.Close()
		return err
	}

	c.mu.Lock()
	c.conn = conn
	c.connected = true
	c.reconnectCnt = 0
	c.mu.Unlock()

	c.logger.Info("WebSocket 连接成功: %s", c.config.ServerURL)

	// 启动读写循环
	c.wg.Add(2)
	go c.readLoop()
	go c.writeLoop()

	return nil
}

// reconnectLoop 重连循环
func (c *WSClient) reconnectLoop() {
	defer c.wg.Done()

	ticker := time.NewTicker(c.config.ReconnectInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.mu.RLock()
			connected := c.connected
			c.mu.RUnlock()

			if !connected {
				c.mu.Lock()
				c.reconnecting = true
				c.reconnectCnt++
				cnt := c.reconnectCnt
				c.mu.Unlock()

				if c.config.MaxReconnectAttempts > 0 && cnt > c.config.MaxReconnectAttempts {
					c.logger.Error("达到最大重连次数，停止重连")
					// 取消上下文以确保所有 goroutine 退出
					c.cancel()
					return
				}

				c.logger.Info("尝试重连 (第 %d 次)...", cnt)
				if err := c.connect(); err != nil {
					c.logger.Warn("重连失败: %v", err)
				}

				c.mu.Lock()
				c.reconnecting = false
				c.mu.Unlock()
			}
		}
	}
}

// readLoop 读取消息循环
func (c *WSClient) readLoop() {
	defer c.wg.Done()
	defer func() {
		c.mu.Lock()
		c.connected = false
		if c.conn != nil {
			c.conn.Close()
		}
		c.mu.Unlock()
	}()

	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()

	if conn == nil {
		return
	}

	conn.SetReadDeadline(time.Now().Add(c.config.ReadTimeout))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(c.config.ReadTimeout))
		return nil
	})

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		_, data, err := conn.ReadMessage()
		if err != nil {
			c.logger.Warn("读取消息失败: %v", err)
			return
		}

		msg, err := protocol.DecodeMessage(data)
		if err != nil {
			c.logger.Warn("解析消息失败: %v", err)
			continue
		}

		c.handleMessage(msg)
	}
}

// writeLoop 写入消息循环
func (c *WSClient) writeLoop() {
	defer c.wg.Done()

	ticker := time.NewTicker(c.config.PingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return

		case msg := <-c.sendCh:
			c.mu.RLock()
			conn := c.conn
			connected := c.connected
			c.mu.RUnlock()

			if !connected || conn == nil {
				continue
			}

			data, err := msg.Encode()
			if err != nil {
				continue
			}

			conn.SetWriteDeadline(time.Now().Add(c.config.WriteTimeout))
			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				c.logger.Warn("发送消息失败: %v", err)
				return
			}

		case <-ticker.C:
			c.mu.RLock()
			conn := c.conn
			connected := c.connected
			c.mu.RUnlock()

			if !connected || conn == nil {
				continue
			}

			conn.SetWriteDeadline(time.Now().Add(c.config.WriteTimeout))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				c.logger.Warn("发送 Ping 失败: %v", err)
				return
			}
		}
	}
}

// handleMessage 处理收到的消息
func (c *WSClient) handleMessage(msg *protocol.WSMessage) {
	c.mu.RLock()
	handler, ok := c.handlers[msg.Type]
	c.mu.RUnlock()

	if ok {
		handler(msg)
		return
	}

	// 默认处理
	switch msg.Type {
	case protocol.MsgTypePing:
		// 响应 Ping
		pongMsg, _ := protocol.NewMessageWithID(msg.ID, protocol.MsgTypePong, nil)
		c.Send(pongMsg)

	case protocol.MsgTypeAck:
		// 确认消息，忽略

	default:
		c.logger.Warn("未处理的消息类型: %s", msg.Type)
	}
}

// SendHeartbeat 发送心跳
func (c *WSClient) SendHeartbeat(status string) error {
	payload := protocol.HeartbeatPayload{
		AgentID: c.agentInfo.ID,
		Status:  status,
	}

	msg, err := protocol.NewMessage(protocol.MsgTypeHeartbeat, payload)
	if err != nil {
		return err
	}

	return c.Send(msg)
}

// SendMetrics 发送指标
func (c *WSClient) SendMetrics(metrics *SystemMetrics) error {
	payload := protocol.MetricsPayload{
		AgentID:     c.agentInfo.ID,
		CPUUsage:    metrics.CPUUsage,
		MemoryUsage: metrics.MemoryUsage,
		MemoryTotal: metrics.MemoryTotal,
		MemoryFree:  metrics.MemoryFree,
		DiskUsage:   metrics.DiskUsage,
		DiskTotal:   metrics.DiskTotal,
		DiskFree:    metrics.DiskFree,
		LoadAvg1:    metrics.LoadAvg1,
		LoadAvg5:    metrics.LoadAvg5,
		LoadAvg15:   metrics.LoadAvg15,
		Uptime:      metrics.Uptime,
	}

	msg, err := protocol.NewMessage(protocol.MsgTypeMetrics, payload)
	if err != nil {
		return err
	}

	return c.Send(msg)
}

// SendEvent 发送事件
func (c *WSClient) SendEvent(event *RuleEvent) error {
	payload := protocol.EventPayload{
		AgentID:   c.agentInfo.ID,
		RuleID:    event.RuleID,
		RuleName:  event.RuleName,
		Condition: event.Condition,
		Action:    event.Action,
		Success:   event.Success,
		Output:    event.Output,
		Error:     event.Error,
		Timestamp: event.Timestamp,
	}

	msg, err := protocol.NewMessage(protocol.MsgTypeEvent, payload)
	if err != nil {
		return err
	}

	return c.Send(msg)
}

// SendCommandResult 发送命令结果
func (c *WSClient) SendCommandResult(result *CommandResult) error {
	payload := protocol.CommandResultPayload{
		CommandID: result.CommandID,
		AgentID:   c.agentInfo.ID,
		Success:   result.Success,
		Output:    result.Output,
		Error:     result.Error,
		ExitCode:  result.ExitCode,
		Duration:  result.Duration,
	}

	msg, err := protocol.NewMessage(protocol.MsgTypeCommandResult, payload)
	if err != nil {
		return err
	}

	return c.Send(msg)
}

// SendEnvironment 发送环境信息
func (c *WSClient) SendEnvironment(env *protocol.EnvironmentPayload) error {
	msg, err := protocol.NewMessage(protocol.MsgTypeEnvironment, env)
	if err != nil {
		return err
	}

	return c.Send(msg)
}
