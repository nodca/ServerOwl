package memory

import "time"

//对话消息（短期记忆）
type ConversationMessage struct {
	ID         int64     `json:"id"`
	UserID     string    `json:"user_id"`
	SessionID  string    `json:"session_id"`
	Role       string    `json:"role"` // "user" | "assistant"
	Content    string    `json:"content"`
	IntentType string    `json:"intent_type"` // 可选：意图类型
	Target     string    `json:"target"`      // 可选：操作目标
	Metadata   string    `json:"metadata"`    // JSON 字符串
	CreatedAt  time.Time `json:"created_at"`
	ExpiresAt  time.Time `json:"expires_at"`
}

// PendingAction 待确认操作（用于“确认/取消”闭环）
type PendingAction struct {
	ID        string     `json:"id"`
	UserID    string     `json:"user_id"`
	SessionID string     `json:"session_id"`
	UserInput string     `json:"user_input"`
	ToolName  string     `json:"tool_name"`
	ArgsJSON  string     `json:"args_json"`
	Prompt    string     `json:"prompt"`
	Status    string     `json:"status"` // pending|confirmed|cancelled|expired|executed|failed
	ErrorMsg  string     `json:"error_msg"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt *time.Time `json:"expires_at"`
}

// Alert 告警记录（长期记忆）
type Alert struct {
	ID         int64      `json:"id"`
	AlertType  string     `json:"alert_type"` // "container" | "http" | "disk" | "memory"
	Target     string     `json:"target"`
	Status     string     `json:"status"`   // "triggered" | "resolved"
	Severity   string     `json:"severity"` // "high" | "medium" | "low"
	Message    string     `json:"message"`
	Details    string     `json:"details"`    // JSON 字符串
	AISummary  string     `json:"ai_summary"` // LLM 生成的摘要
	CreatedAt  time.Time  `json:"created_at"`
	ResolvedAt *time.Time `json:"resolved_at"` // 指针类型，可为 nil
}

// Operation 操作记录（长期记忆）
type Operation struct {
	ID            int64     `json:"id"`
	UserID        string    `json:"user_id"`
	OperationType string    `json:"operation_type"` // "restart" | "logs" | "status"
	Target        string    `json:"target"`
	Success       bool      `json:"success"`
	UserInput     string    `json:"user_input"`
	Result        string    `json:"result"`
	AISummary     string    `json:"ai_summary"`
	CreatedAt     time.Time `json:"created_at"`
	DurationMs    int64     `json:"duration_ms"`
}

// Summary LLM 生成的摘要
type Summary struct {
	ID          int64     `json:"id"`
	SummaryType string    `json:"summary_type"` // "daily" | "weekly" | "target"
	Target      string    `json:"target"`       // 可选：针对特定目标
	PeriodStart time.Time `json:"period_start"`
	PeriodEnd   time.Time `json:"period_end"`
	Content     string    `json:"content"`
	KeyFacts    string    `json:"key_facts"` // JSON 数组字符串
	CreatedAt   time.Time `json:"created_at"`
}

// MemoryContext 用于注入到 LLM 的上下文
type MemoryContext struct {
	RecentMessages []ConversationMessage `json:"recent_messages"`
	RecentAlerts   []Alert               `json:"recent_alerts"`
	RecentOps      []Operation           `json:"recent_ops"`
	Summaries      []Summary             `json:"summaries"`
}
