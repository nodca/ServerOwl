package logging

// Standard field names for consistent logging
const (
  // Request fields
  FieldRequestID = "request_id"
  FieldSessionID = "session_id"
  FieldUserID    = "user_id"
  FieldTraceID   = "trace_id"

  // Operation fields
  FieldOperation = "operation"
  FieldTool      = "tool"
  FieldAction    = "action"
  FieldTarget    = "target"
  FieldDuration  = "duration_ms"
  FieldSuccess   = "success"

  // Error fields
  FieldError     = "error"
  FieldErrorCode = "error_code"
  FieldErrorType = "error_type"

  // System fields
  FieldComponent = "component"
  FieldVersion   = "version"
  FieldHost      = "host"
  FieldPID       = "pid"

  // Agent fields
  FieldIteration = "iteration"
  FieldToolCalls = "tool_calls"
  FieldTokens    = "tokens"

  // Memory fields
  FieldEpisodeID  = "episode_id"
  FieldImportance = "importance"

  // Scheduler fields
  FieldTaskID   = "task_id"
  FieldSchedule = "schedule"
  FieldNextRun  = "next_run"
)

// Component names
const (
  ComponentAgent     = "agent"
  ComponentScheduler = "scheduler"
  ComponentMemory    = "memory"
  ComponentMonitor   = "monitor"
  ComponentWeChat    = "wechat"
  ComponentSkills    = "skills"
  ComponentHTTP      = "http"
)

// Operation names
const (
  OpToolExecute    = "tool_execute"
  OpLLMCall        = "llm_call"
  OpMemorySave     = "memory_save"
  OpMemorySearch   = "memory_search"
  OpTaskRun        = "task_run"
  OpSkillExecute   = "skill_execute"
  OpConfirmRequest = "confirm_request"
  OpConfirmHandle  = "confirm_handle"
)

// WithRequestContext creates a logger with request context fields
func WithRequestContext(l *Logger, requestID, sessionID, userID string) *Logger {
  return l.With().
    Str(FieldRequestID, requestID).
    Str(FieldSessionID, sessionID).
    Str(FieldUserID, userID).
    Logger()
}

// WithComponent creates a logger with component field
func WithComponent(l *Logger, component string) *Logger {
  return l.With().
    Str(FieldComponent, component).
    Logger()
}

// WithOperation creates a logger with operation field
func WithOperation(l *Logger, operation string) *Logger {
  return l.With().
    Str(FieldOperation, operation).
    Logger()
}
