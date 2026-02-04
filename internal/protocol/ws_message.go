package protocol

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// WebSocket 消息类型常量
const (
	// Agent -> Master
	MsgTypeRegister      = "register"       // 注册
	MsgTypeHeartbeat     = "heartbeat"      // 心跳
	MsgTypeMetrics       = "metrics"        // 指标上报
	MsgTypeEvent         = "event"          // 事件上报
	MsgTypeCommandResult = "command_result" // 命令结果
	MsgTypeEnvironment   = "environment"    // 环境信息上报

	// Master -> Agent
	MsgTypeCommand         = "command"         // 命令下发
	MsgTypeConfigUpdate    = "config_update"   // 配置更新
	MsgTypeRefreshEnv      = "refresh_env"     // 刷新环境信息
	MsgTypePing            = "ping"            // 心跳检测
	MsgTypePong            = "pong"            // 心跳响应
	MsgTypeAck             = "ack"             // 消息确认
	MsgTypeError           = "error"           // 错误响应
)

// WSMessage WebSocket 消息结构
type WSMessage struct {
	ID        string          `json:"id"`
	Type      string          `json:"type"`
	Timestamp time.Time       `json:"timestamp"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

// NewMessage 创建新消息
func NewMessage(msgType string, payload any) (*WSMessage, error) {
	var payloadBytes json.RawMessage
	if payload != nil {
		var err error
		payloadBytes, err = json.Marshal(payload)
		if err != nil {
			return nil, err
		}
	}

	return &WSMessage{
		ID:        uuid.New().String(),
		Type:      msgType,
		Timestamp: time.Now(),
		Payload:   payloadBytes,
	}, nil
}

// NewMessageWithID 创建带指定 ID 的消息
func NewMessageWithID(id, msgType string, payload any) (*WSMessage, error) {
	msg, err := NewMessage(msgType, payload)
	if err != nil {
		return nil, err
	}
	msg.ID = id
	return msg, nil
}

// ParsePayload 解析消息载荷
func (m *WSMessage) ParsePayload(v any) error {
	if m.Payload == nil {
		return nil
	}
	return json.Unmarshal(m.Payload, v)
}

// Encode 编码消息为 JSON
func (m *WSMessage) Encode() ([]byte, error) {
	return json.Marshal(m)
}

// DecodeMessage 解码 JSON 为消息
func DecodeMessage(data []byte) (*WSMessage, error) {
	var msg WSMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

// RegisterPayload 注册消息载荷
type RegisterPayload struct {
	AgentID   string            `json:"agent_id"`
	Name      string            `json:"name"`
	Hostname  string            `json:"hostname"`
	IP        string            `json:"ip"`
	Tags      []string          `json:"tags"`
	Labels    map[string]string `json:"labels"`
	Version   string            `json:"version"`
	StartTime time.Time         `json:"start_time"`
}

// HeartbeatPayload 心跳消息载荷
type HeartbeatPayload struct {
	AgentID string `json:"agent_id"`
	Status  string `json:"status"`
}

// MetricsPayload 指标消息载荷
type MetricsPayload struct {
	AgentID     string  `json:"agent_id"`
	CPUUsage    float64 `json:"cpu_usage"`
	MemoryUsage float64 `json:"memory_usage"`
	MemoryTotal uint64  `json:"memory_total"`
	MemoryFree  uint64  `json:"memory_free"`
	DiskUsage   float64 `json:"disk_usage"`
	DiskTotal   uint64  `json:"disk_total"`
	DiskFree    uint64  `json:"disk_free"`
	LoadAvg1    float64 `json:"load_avg_1"`
	LoadAvg5    float64 `json:"load_avg_5"`
	LoadAvg15   float64 `json:"load_avg_15"`
	Uptime      int64   `json:"uptime"`
}

// EventPayload 事件消息载荷
type EventPayload struct {
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

// CommandPayload 命令消息载荷
type CommandPayload struct {
	CommandID string        `json:"command_id"`
	Type      string        `json:"type"` // shell, skill, config, shutdown
	Payload   string        `json:"payload"`
	Timeout   time.Duration `json:"timeout"`
	Priority  int           `json:"priority"`
}

// CommandResultPayload 命令结果载荷
type CommandResultPayload struct {
	CommandID string        `json:"command_id"`
	AgentID   string        `json:"agent_id"`
	Success   bool          `json:"success"`
	Output    string        `json:"output"`
	Error     string        `json:"error,omitempty"`
	ExitCode  int           `json:"exit_code"`
	Duration  time.Duration `json:"duration"`
}

// ConfigUpdatePayload 配置更新载荷
type ConfigUpdatePayload struct {
	Version   string         `json:"version"`
	AutoRules []AutoRuleData `json:"auto_rules,omitempty"`
	Settings  map[string]any `json:"settings,omitempty"`
}

// AutoRuleData 自治规则数据
type AutoRuleData struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Enabled     bool          `json:"enabled"`
	Condition   RuleCondition `json:"condition"`
	Actions     []RuleAction  `json:"actions"`
	Cooldown    time.Duration `json:"cooldown"`
	MaxRetries  int           `json:"max_retries"`
}

// RuleCondition 规则条件
type RuleCondition struct {
	Type      string        `json:"type"`
	Metric    string        `json:"metric"`
	Operator  string        `json:"operator"`
	Threshold float64       `json:"threshold"`
	Duration  time.Duration `json:"duration"`
}

// RuleAction 规则动作
type RuleAction struct {
	Type    string        `json:"type"`
	Command string        `json:"command"`
	Timeout time.Duration `json:"timeout"`
}

// AckPayload 确认消息载荷
type AckPayload struct {
	MessageID string `json:"message_id"`
	Success   bool   `json:"success"`
	Error     string `json:"error,omitempty"`
}

// ErrorPayload 错误消息载荷
type ErrorPayload struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// EnvironmentPayload 环境信息载荷
type EnvironmentPayload struct {
	AgentID    string                    `json:"agent_id"`
	UpdatedAt  string                    `json:"updated_at"`
	Host       HostInfo                  `json:"host"`
	Containers map[string]*ContainerInfo `json:"containers,omitempty"`
	Databases  map[string]*DatabaseInfo  `json:"databases,omitempty"`
	Proxies    map[string]*ProxyInfo     `json:"proxies,omitempty"`
}

// HostInfo 主机信息
type HostInfo struct {
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
