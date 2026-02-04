// Package agentnode 提供轻量级 Agent 节点功能
package agentnode

import (
	"time"
)

// AgentStatus Agent 状态
type AgentStatus string

const (
	AgentStatusOnline      AgentStatus = "online"
	AgentStatusOffline     AgentStatus = "offline"
	AgentStatusAutonomous  AgentStatus = "autonomous" // 自治模式（与主节点断开）
)

// AgentInfo Agent 基本信息
type AgentInfo struct {
	ID        string            `json:"id" yaml:"id"`
	Name      string            `json:"name" yaml:"name"`
	Hostname  string            `json:"hostname" yaml:"hostname"`
	IP        string            `json:"ip" yaml:"ip"`
	Tags      []string          `json:"tags" yaml:"tags"`
	Version   string            `json:"version" yaml:"version"`
	StartTime time.Time         `json:"start_time" yaml:"start_time"`
	Status    AgentStatus       `json:"status" yaml:"status"`
	Labels    map[string]string `json:"labels" yaml:"labels"`
}

// MasterNode 主节点信息
type MasterNode struct {
	ID       string `json:"id" yaml:"id"`
	Name     string `json:"name" yaml:"name"`
	Addr     string `json:"addr" yaml:"addr"` // host:port
	Priority int    `json:"priority" yaml:"priority"`
	IsActive bool   `json:"is_active" yaml:"is_active"`
}

// HeartbeatRequest 心跳请求
type HeartbeatRequest struct {
	AgentID   string        `json:"agent_id"`
	Timestamp time.Time     `json:"timestamp"`
	Status    AgentStatus   `json:"status"`
	Metrics   *SystemMetrics `json:"metrics,omitempty"`
}

// HeartbeatResponse 心跳响应
type HeartbeatResponse struct {
	Success      bool           `json:"success"`
	MasterID     string         `json:"master_id"`
	Timestamp    time.Time      `json:"timestamp"`
	Commands     []*Command     `json:"commands,omitempty"`     // 待执行命令
	ConfigUpdate *ConfigUpdate  `json:"config_update,omitempty"` // 配置更新
}

// SystemMetrics 系统指标
type SystemMetrics struct {
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
	Timestamp   time.Time `json:"timestamp"`
}

// Command 命令
type Command struct {
	ID        string         `json:"id"`
	Type      CommandType    `json:"type"`
	Payload   string         `json:"payload"`
	Timeout   time.Duration  `json:"timeout"`
	Priority  int            `json:"priority"`
	CreatedAt time.Time      `json:"created_at"`
}

// CommandType 命令类型
type CommandType string

const (
	CommandTypeShell    CommandType = "shell"
	CommandTypeSkill    CommandType = "skill"
	CommandTypeConfig   CommandType = "config"
	CommandTypeShutdown CommandType = "shutdown"
)

// CommandResult 命令执行结果
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

// ConfigUpdate 配置更新
type ConfigUpdate struct {
	Version    string                 `json:"version"`
	AutoRules  []AutoRule             `json:"auto_rules,omitempty"`
	Settings   map[string]interface{} `json:"settings,omitempty"`
	UpdatedAt  time.Time              `json:"updated_at"`
}

// AutoRule 自治规则
type AutoRule struct {
	ID          string        `json:"id" yaml:"id"`
	Name        string        `json:"name" yaml:"name"`
	Description string        `json:"description" yaml:"description"`
	Enabled     bool          `json:"enabled" yaml:"enabled"`
	Condition   RuleCondition `json:"condition" yaml:"condition"`
	Actions     []RuleAction  `json:"actions" yaml:"actions"`
	Cooldown    time.Duration `json:"cooldown" yaml:"cooldown"`       // 冷却时间
	MaxRetries  int           `json:"max_retries" yaml:"max_retries"` // 最大重试次数
	LastTriggered time.Time   `json:"last_triggered" yaml:"-"`
	TriggerCount  int         `json:"trigger_count" yaml:"-"`
}

// RuleCondition 规则条件
type RuleCondition struct {
	Type      ConditionType `json:"type" yaml:"type"`
	Metric    string        `json:"metric" yaml:"metric"`       // cpu_usage, memory_usage, disk_usage, service:xxx
	Operator  string        `json:"operator" yaml:"operator"`   // >, <, >=, <=, ==, !=
	Threshold float64       `json:"threshold" yaml:"threshold"` // 阈值
	Duration  time.Duration `json:"duration" yaml:"duration"`   // 持续时间
}

// ConditionType 条件类型
type ConditionType string

const (
	ConditionTypeMetric  ConditionType = "metric"  // 指标阈值
	ConditionTypeService ConditionType = "service" // 服务状态
	ConditionTypeProcess ConditionType = "process" // 进程状态
	ConditionTypeFile    ConditionType = "file"    // 文件存在
)

// RuleAction 规则动作
type RuleAction struct {
	Type    ActionType `json:"type" yaml:"type"`
	Command string     `json:"command" yaml:"command"`
	Timeout time.Duration `json:"timeout" yaml:"timeout"`
}

// ActionType 动作类型
type ActionType string

const (
	ActionTypeShell   ActionType = "shell"   // 执行 shell 命令
	ActionTypeRestart ActionType = "restart" // 重启服务
	ActionTypeNotify  ActionType = "notify"  // 发送通知（记录日志，等主节点恢复后上报）
)
