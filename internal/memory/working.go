package memory

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// 任务步骤
type TaskStep struct {
	Description string         `json:"description"`
	ToolName    string         `json:"tool_name,omitempty"`
	Args        map[string]any `json:"args,omitempty"`
	Result      string         `json:"result,omitempty"`
	Status      string         `json:"status"` // pending | in_progress | completed | failed
	StartedAt   *time.Time     `json:"started_at,omitempty"`
	CompletedAt *time.Time     `json:"completed_at,omitempty"`
}

// 任务状态
type TaskState struct {
	ID              string         `json:"id"`
	SessionID       string         `json:"session_id"`
	UserID          string         `json:"user_id"`
	OriginalRequest string         `json:"original_request"`
	Summary         string         `json:"summary"`
	Steps           []TaskStep     `json:"steps"`
	CurrentStep     int            `json:"current_step"`
	Context         map[string]any `json:"context"`
	Status          string         `json:"status"` // in_progress | completed | failed
	StartedAt       time.Time      `json:"started_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

// 工作记忆管理
type WorkingMemory struct {
	db *sql.DB
}

func NewWorkingMemory(db *sql.DB) *WorkingMemory {
	return &WorkingMemory{
		db: db,
	}
}

func (w *WorkingMemory) StartTask(sessionID, userID, request, summary string) (*TaskState, error) {
	task := &TaskState{
		ID:              uuid.New().String(),
		SessionID:       sessionID,
		UserID:          userID,
		OriginalRequest: request,
		Summary:         summary,
		Steps:           []TaskStep{},
		CurrentStep:     0,
		Context:         make(map[string]any),
		Status:          "in_progress",
		StartedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	stepsJSON, _ := json.Marshal(task.Steps)
	contextJSON, _ := json.Marshal(task.Context)
	_, err := w.db.Exec(`
	INSERT INTO task_states (id, session_id, user_id, original_request, summary, steps, current_step, context,status,started_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`, task.ID, task.SessionID, task.UserID, task.OriginalRequest, task.Summary, stepsJSON, task.CurrentStep, contextJSON, task.Status, task.StartedAt, task.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create task: %w", err)
	}
	return task, nil
}

// GetTask 获取任务
func (w *WorkingMemory) GetTask(taskID string) (*TaskState, error) {
	var task TaskState
	var stepsJSON, contextJSON []byte

	err := w.db.QueryRow(`
		SELECT id, session_id, user_id, original_request, summary, steps, current_step, context, status, started_at,updated_at FROM task_states WHERE id = $1
	`, taskID).Scan(
		&task.ID, &task.SessionID, &task.UserID, &task.OriginalRequest, &task.Summary,
		&stepsJSON, &task.CurrentStep, &contextJSON, &task.Status, &task.StartedAt, &task.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get task: %w", err)
	}

	json.Unmarshal(stepsJSON, &task.Steps)
	json.Unmarshal(contextJSON, &task.Context)

	return &task, nil
}

// GetCurrentTask 获取会话当前进行中的任务
func (w *WorkingMemory) GetCurrentTask(sessionID string) (*TaskState, error) {
	var task TaskState
	var stepsJSON, contextJSON []byte

	err := w.db.QueryRow(`
			SELECT id, session_id, user_id, original_request, summary, steps, current_step, context, status, started_at,updated_at
			FROM task_states
			WHERE session_id = $1 AND status = 'in_progress'
			ORDER BY started_at DESC
			LIMIT 1
	`, sessionID).Scan(
		&task.ID, &task.SessionID, &task.UserID, &task.OriginalRequest, &task.Summary,
		&stepsJSON, &task.CurrentStep, &contextJSON, &task.Status, &task.StartedAt, &task.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get current task: %w", err)
	}

	json.Unmarshal(stepsJSON, &task.Steps)
	json.Unmarshal(contextJSON, &task.Context)

	return &task, nil
}

// 添加任务步骤
func (w *WorkingMemory) AddStep(taskID string, step TaskStep) error {
	task, err := w.GetTask(taskID)
	if err != nil {
		return err
	}
	if task == nil {
		return fmt.Errorf("task not found: %s", taskID)
	}
	task.Steps = append(task.Steps, step)
	task.UpdatedAt = time.Now()

	setpsJSON, _ := json.Marshal(task.Steps)
	_, err = w.db.Exec(`
	UPDATE task_states SET steps=$1,updated_at=$2 WHERE id=$3
	`, setpsJSON, task.UpdatedAt, taskID)
	return err
}

// 更新指定步骤
func (w *WorkingMemory) UpdateStep(taskID string, stepIndex int, step TaskStep) error {
	task, err := w.GetTask(taskID)
	if err != nil {
		return err
	}
	if task == nil {
		return fmt.Errorf("task not found: %s", taskID)
	}
	if stepIndex < 0 || stepIndex >= len(task.Steps) {
		return fmt.Errorf("step index out of range: %d", stepIndex)
	}

	task.Steps[stepIndex] = step
	task.UpdatedAt = time.Now()
	stepsJSON, _ := json.Marshal(task.Steps)
	_, err = w.db.Exec(`
	UPDATE task_states SET steps = $1, current_step = $2, updated_at = $3 WHERE id = $4
`, stepsJSON, stepIndex, task.UpdatedAt, taskID)

	return err
}

// CompleteTask 完成任务
func (w *WorkingMemory) CompleteTask(taskID, status string) error {
	_, err := w.db.Exec(`
			UPDATE task_states SET status = $1, updated_at = $2 WHERE id = $3
	`, status, time.Now(), taskID)
	return err
}

// UpdateContext 更新任务上下文
func (w *WorkingMemory) UpdateContext(taskID string, context map[string]any) error {
	contextJSON, _ := json.Marshal(context)
	_, err := w.db.Exec(`
			UPDATE task_states SET context = $1, updated_at = $2 WHERE id = $3
	`, contextJSON, time.Now(), taskID)
	return err
}
