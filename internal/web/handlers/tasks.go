package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"serverowl/internal/scheduler"

	"github.com/google/uuid"
)

// TasksHandler 任务管理 API 处理器
type TasksHandler struct {
	scheduler *scheduler.Scheduler
}

// NewTasksHandler 创建任务处理器
func NewTasksHandler(sched *scheduler.Scheduler) *TasksHandler {
	return &TasksHandler{
		scheduler: sched,
	}
}

// TaskCreateRequest 创建任务请求
type TaskCreateRequest struct {
	Name        string                `json:"name"`
	Description string                `json:"description"`
	Schedule    string                `json:"schedule"` // cron 表达式
	Type        string                `json:"type"`     // skill, command, workflow
	Config      scheduler.TaskConfig  `json:"config"`
	Tags        []string              `json:"tags"`
}

// TaskUpdateRequest 更新任务请求
type TaskUpdateRequest struct {
	Name        *string               `json:"name,omitempty"`
	Description *string               `json:"description,omitempty"`
	Schedule    *string               `json:"schedule,omitempty"`
	Config      *scheduler.TaskConfig `json:"config,omitempty"`
	Status      *string               `json:"status,omitempty"`
	Tags        []string              `json:"tags,omitempty"`
}

// TaskResponse 任务响应
type TaskResponse struct {
	ID          string               `json:"id"`
	Name        string               `json:"name"`
	Description string               `json:"description"`
	Schedule    string               `json:"schedule"`
	Type        string               `json:"type"`
	Config      scheduler.TaskConfig `json:"config"`
	Status      string               `json:"status"`
	Tags        []string             `json:"tags"`
	LastRun     *time.Time           `json:"last_run,omitempty"`
	NextRun     *time.Time           `json:"next_run,omitempty"`
	RunCount    int64                `json:"run_count"`
	FailCount   int64                `json:"fail_count"`
	LastResult  string               `json:"last_result,omitempty"`
	LastError   string               `json:"last_error,omitempty"`
	CreatedAt   time.Time            `json:"created_at"`
	UpdatedAt   time.Time            `json:"updated_at"`
}

// HandleTasks 处理任务列表请求
// GET /api/v1/tasks - 列出所有任务
// POST /api/v1/tasks - 创建新任务
func (h *TasksHandler) HandleTasks(w http.ResponseWriter, r *http.Request) {
	if h.scheduler == nil {
		writeInternalError(w, "调度器未初始化")
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.listTasks(w, r)
	case http.MethodPost:
		h.createTask(w, r)
	default:
		writeMethodNotAllowed(w)
	}
}

// HandleTaskByID 处理单个任务请求
// GET /api/v1/tasks/{id} - 获取任务详情
// PUT /api/v1/tasks/{id} - 更新任务
// DELETE /api/v1/tasks/{id} - 删除任务
// GET /api/v1/tasks/{id}/history - 获取任务执行历史
func (h *TasksHandler) HandleTaskByID(w http.ResponseWriter, r *http.Request) {
	if h.scheduler == nil {
		writeInternalError(w, "调度器未初始化")
		return
	}

	// 解析路径: /api/v1/tasks/{id} 或 /api/v1/tasks/{id}/history
	path := r.URL.Path
	prefix := "/api/v1/tasks/"
	remaining := strings.TrimPrefix(path, prefix)

	// 检查是否是 history 请求
	if strings.HasSuffix(remaining, "/history") {
		taskID := strings.TrimSuffix(remaining, "/history")
		if r.Method == http.MethodGet {
			h.getTaskHistory(w, r, taskID)
		} else {
			writeMethodNotAllowed(w)
		}
		return
	}

	taskID := remaining

	switch r.Method {
	case http.MethodGet:
		h.getTask(w, r, taskID)
	case http.MethodPut:
		h.updateTask(w, r, taskID)
	case http.MethodDelete:
		h.deleteTask(w, r, taskID)
	default:
		writeMethodNotAllowed(w)
	}
}

// listTasks 列出所有任务
func (h *TasksHandler) listTasks(w http.ResponseWriter, r *http.Request) {
	tasks := h.scheduler.ListTasks()

	responses := make([]TaskResponse, 0, len(tasks))
	for _, t := range tasks {
		responses = append(responses, taskToResponse(t))
	}

	writeSuccess(w, responses)
}

// createTask 创建新任务
func (h *TasksHandler) createTask(w http.ResponseWriter, r *http.Request) {
	var req TaskCreateRequest
	if err := parseJSON(r, &req); err != nil {
		writeBadRequest(w, "无效的请求体: "+err.Error())
		return
	}

	// 验证必填字段
	if req.Name == "" {
		writeBadRequest(w, "任务名称不能为空")
		return
	}
	if req.Schedule == "" {
		writeBadRequest(w, "调度表达式不能为空")
		return
	}
	if req.Type == "" {
		req.Type = "skill"
	}

	// 创建任务
	task := &scheduler.ScheduledTask{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Description: req.Description,
		Schedule:    req.Schedule,
		Type:        scheduler.TaskType(req.Type),
		Config:      req.Config,
		Status:      scheduler.TaskStatusActive,
		Tags:        req.Tags,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := h.scheduler.AddTask(task); err != nil {
		writeBadRequest(w, "创建任务失败: "+err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, APIResponse{
		Success: true,
		Data:    taskToResponse(task),
		Message: "任务创建成功",
	})
}

// getTask 获取任务详情
func (h *TasksHandler) getTask(w http.ResponseWriter, r *http.Request, taskID string) {
	task, ok := h.scheduler.GetTask(taskID)
	if !ok {
		writeNotFound(w, "任务不存在")
		return
	}

	writeSuccess(w, taskToResponse(task))
}

// updateTask 更新任务
func (h *TasksHandler) updateTask(w http.ResponseWriter, r *http.Request, taskID string) {
	task, ok := h.scheduler.GetTask(taskID)
	if !ok {
		writeNotFound(w, "任务不存在")
		return
	}

	var req TaskUpdateRequest
	if err := parseJSON(r, &req); err != nil {
		writeBadRequest(w, "无效的请求体: "+err.Error())
		return
	}

	// 更新字段
	needReschedule := false

	if req.Name != nil {
		task.Name = *req.Name
	}
	if req.Description != nil {
		task.Description = *req.Description
	}
	if req.Schedule != nil && *req.Schedule != task.Schedule {
		task.Schedule = *req.Schedule
		needReschedule = true
	}
	if req.Config != nil {
		task.Config = *req.Config
	}
	if req.Status != nil {
		task.Status = scheduler.TaskStatus(*req.Status)
	}
	if req.Tags != nil {
		task.Tags = req.Tags
	}

	task.UpdatedAt = time.Now()

	// 如果调度表达式变更，需要重新调度
	if needReschedule {
		// 先删除旧任务
		if err := h.scheduler.RemoveTask(taskID); err != nil {
			writeInternalError(w, "更新任务失败: "+err.Error())
			return
		}
		// 重新添加
		if err := h.scheduler.AddTask(task); err != nil {
			writeInternalError(w, "更新任务失败: "+err.Error())
			return
		}
	}

	writeSuccess(w, taskToResponse(task))
}

// deleteTask 删除任务
func (h *TasksHandler) deleteTask(w http.ResponseWriter, r *http.Request, taskID string) {
	if err := h.scheduler.RemoveTask(taskID); err != nil {
		writeNotFound(w, "任务不存在或删除失败: "+err.Error())
		return
	}

	writeSuccessMessage(w, "任务删除成功")
}

// getTaskHistory 获取任务执行历史
func (h *TasksHandler) getTaskHistory(w http.ResponseWriter, r *http.Request, taskID string) {
	// 检查任务是否存在
	_, ok := h.scheduler.GetTask(taskID)
	if !ok {
		writeNotFound(w, "任务不存在")
		return
	}

	// 获取 limit 参数（预留给后续扩展使用）
	_ = 20
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			_ = l
		}
	}

	// 注意: 这里需要通过 store 获取历史记录
	// 由于 Scheduler 没有直接暴露 store，这里返回空列表
	// 实际使用时需要扩展 Scheduler 接口
	history := make([]scheduler.TaskResult, 0)

	writeSuccess(w, history)
}

// taskToResponse 将任务转换为响应格式
func taskToResponse(t *scheduler.ScheduledTask) TaskResponse {
	resp := TaskResponse{
		ID:          t.ID,
		Name:        t.Name,
		Description: t.Description,
		Schedule:    t.Schedule,
		Type:        string(t.Type),
		Config:      t.Config,
		Status:      string(t.Status),
		Tags:        t.Tags,
		RunCount:    t.RunCount,
		FailCount:   t.FailCount,
		LastResult:  t.LastResult,
		LastError:   t.LastError,
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
	}

	if !t.LastRun.IsZero() {
		resp.LastRun = &t.LastRun
	}
	if !t.NextRun.IsZero() {
		resp.NextRun = &t.NextRun
	}

	return resp
}
