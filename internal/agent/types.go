package agent

import (
	"serverowl/internal/llm"
	"time"
)

type Tool struct {
	Name          string
	Description   string
	Parameters    map[string]any // JSON Schema
	Handler       func(args map[string]any) (string, error)
	NeedConfirm   bool
	Timeout       time.Duration
	RetryCount    int
	SafetyChecker *SafetyChecker
}

type ToolRegistry struct {
	tools map[string]*Tool
}

type ExecutionContext struct {
	SessionID string
	UserID    string
	Messages  []llm.Message
	Iteration int
}
