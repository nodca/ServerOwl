package scheduler

import (
  "time"

  "github.com/robfig/cron/v3"
)

type TaskStatus string

const (
  TaskStatusActive   TaskStatus = "active"
  TaskStatusPaused   TaskStatus = "paused"
  TaskStatusDisabled TaskStatus = "disabled"
)

type TaskType string

const (
  TaskTypeSkill    TaskType = "skill"
  TaskTypeCommand  TaskType = "command"
  TaskTypeWorkflow TaskType = "workflow"
)

type ScheduledTask struct {
  ID          string            `yaml:"id" json:"id"`
  Name        string            `yaml:"name" json:"name"`
  Description string            `yaml:"description" json:"description"`
  Schedule    string            `yaml:"schedule" json:"schedule"` // cron expression
  Type        TaskType          `yaml:"type" json:"type"`
  Config      TaskConfig        `yaml:"config" json:"config"`
  Status      TaskStatus        `yaml:"status" json:"status"`
  Tags        []string          `yaml:"tags" json:"tags"`

  // Runtime state
  EntryID    cron.EntryID `json:"-"`
  LastRun    time.Time    `json:"last_run"`
  NextRun    time.Time    `json:"next_run"`
  RunCount   int64        `json:"run_count"`
  FailCount  int64        `json:"fail_count"`
  LastResult string       `json:"last_result"`
  LastError  string       `json:"last_error"`
  CreatedAt  time.Time    `json:"created_at"`
  UpdatedAt  time.Time    `json:"updated_at"`
}

type TaskConfig struct {
  // For skill type
  SkillName   string         `yaml:"skill_name" json:"skill_name"`
  SkillParams map[string]any `yaml:"skill_params" json:"skill_params"`

  // For command type
  Command string `yaml:"command" json:"command"`

  // For workflow type
  WorkflowID string `yaml:"workflow_id" json:"workflow_id"`

  // Common options
  Timeout      time.Duration `yaml:"timeout" json:"timeout"`
  RetryCount   int           `yaml:"retry_count" json:"retry_count"`
  RetryDelay   time.Duration `yaml:"retry_delay" json:"retry_delay"`
  NotifyOnFail bool          `yaml:"notify_on_fail" json:"notify_on_fail"`
  NotifyUser   string        `yaml:"notify_user" json:"notify_user"`
}

type TaskResult struct {
  TaskID    string
  Success   bool
  Output    string
  Error     string
  StartTime time.Time
  EndTime   time.Time
  Duration  time.Duration
}
