package memory

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"
)

// MemoryManager 统一的记忆管理入口
type MemoryManager struct {
	db          *sql.DB
	shortTerm   *ShortTermMemory
	working     *WorkingMemory
	longTerm    *LongTermStore
	embedding   *EmbeddingService
	forget      *ForgetGate
	consolidate *Consolidator
	config      *ManagerConfig
}

// ManagerConfig 记忆管理器配置
type ManagerConfig struct {
	// PostgreSQL
	PostgresDSN string

	// 短期记忆
	ShortTermMaxTurns int
	ShortTermTTL      time.Duration

	// 工作记忆
	TaskTimeout time.Duration

	// 长期记忆
	MaxEpisodes     int
	ForgetThreshold float64
	ForgetMinAge    time.Duration

	// Embedding
	EmbeddingAPIKey  string
	EmbeddingBaseURL string
	EmbeddingModel   string

	// 整合
	ConsolidateMinEpisodes int
}

// ShortTermConfig 短期记忆配置（用于 NewShortTermMemory）
type ShortTermCfg struct {
	MaxTurns int
	TTL      time.Duration
}

// NewMemoryManager 创建记忆管理器
func NewMemoryManager(cfg *ManagerConfig, llm LLMClient) (*MemoryManager, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is required")
	}

	// 1. 连接 PostgreSQL
	store, err := NewPostgresStore(cfg.PostgresDSN)
	if err != nil {
		return nil, fmt.Errorf("failed to connect postgres: %w", err)
	}
	db := store.db

	// 2. 初始化 Embedding 服务
	var embedding *EmbeddingService
	if cfg.EmbeddingAPIKey != "" {
		embedding = NewEmbeddingService(&EmbeddingConfig{
			APIKey:  cfg.EmbeddingAPIKey,
			BaseURL: cfg.EmbeddingBaseURL,
			Model:   cfg.EmbeddingModel,
		})
	}

	// 3. 初始化各组件
	shortTerm := NewShortTermMemory(cfg.ShortTermMaxTurns, cfg.ShortTermTTL)

	working := NewWorkingMemory(db)

	longTerm := NewLongTermStore(db, embedding)

	forget := NewForgetGate(db, &ForgetConfig{
		Threshold:   cfg.ForgetThreshold,
		MaxEpisodes: cfg.MaxEpisodes,
		MinAge:      cfg.ForgetMinAge,
	})

	var consolidate *Consolidator
	if llm != nil {
		consolidate = NewConsolidator(db, longTerm, llm, &ConsolidatorConfig{
			MinEpisodes: cfg.ConsolidateMinEpisodes,
		})
	}

	return &MemoryManager{
		db:          db,
		shortTerm:   shortTerm,
		working:     working,
		longTerm:    longTerm,
		embedding:   embedding,
		forget:      forget,
		consolidate: consolidate,
		config:      cfg,
	}, nil
}

// Close 关闭所有连接
func (m *MemoryManager) Close() error {
	if m.db != nil {
		return m.db.Close()
	}
	return nil
}

// ========== 短期记忆 ==========

// SaveMessage 保存对话消息
func (m *MemoryManager) SaveMessage(sessionID, userID, role, content string) {
	log.Printf("[DEBUG] MemoryManager.SaveMessage: sessionID=%q, role=%q, len=%d", sessionID, role, len(content))
	m.shortTerm.SaveMessage(sessionID, userID, role, content, nil)
}

// GetRecentMessages 获取最近的对话消息
func (m *MemoryManager) GetRecentMessages(sessionID string) []ChatMessage {
	return m.shortTerm.GetMessage(sessionID)
}

// ClearSession 清除会话
func (m *MemoryManager) ClearSession(sessionID string) {
	m.shortTerm.ClearSession(sessionID)
}

// ========== 工作记忆 ==========

// StartTask 开始新任务
func (m *MemoryManager) StartTask(sessionID, userID, request string) (*TaskState, error) {
	return m.working.StartTask(sessionID, userID, request, "")
}

// UpdateTaskStep 更新任务步骤
func (m *MemoryManager) UpdateTaskStep(taskID string, step TaskStep) error {
	return m.working.AddStep(taskID, step)
}

// CompleteTask 完成任务并归档为 Episode
func (m *MemoryManager) CompleteTask(taskID, outcome, summary string) (*Episode, error) {
	task, err := m.working.GetTask(taskID)
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}

	// 更新任务状态
	if err := m.working.CompleteTask(taskID, outcome); err != nil {
		return nil, err
	}

	// 创建 Episode
	episode := &Episode{
		SessionID:      task.SessionID,
		UserID:         task.UserID,
		TriggerType:    "user_request",
		TriggerSummary: task.OriginalRequest,
		Steps:          task.Steps,
		Outcome:        outcome,
		OutcomeSummary: summary,
		Target:         extractTarget(task.Steps),
	}

	if err := m.longTerm.SaveEpisode(episode); err != nil {
		return nil, err
	}

	return episode, nil
}

// GetCurrentTask 获取当前任务
func (m *MemoryManager) GetCurrentTask(sessionID string) (*TaskState, error) {
	return m.working.GetCurrentTask(sessionID)
}

// ========== 长期记忆 ==========

// SearchEpisodes 语义检索 Episodes
func (m *MemoryManager) SearchEpisodes(query string, limit int) ([]Episode, error) {
	return m.longTerm.SearchEpisodes(query, limit)
}

// SearchKnowledge 语义检索 Knowledge
func (m *MemoryManager) SearchKnowledge(query string, limit int) ([]Knowledge, error) {
	return m.longTerm.SearchKnowledge(query, limit)
}

// SaveEpisode 保存 Episode
func (m *MemoryManager) SaveEpisode(episode *Episode) error {
	return m.longTerm.SaveEpisode(episode)
}

// GetEpisode 获取单个 Episode
func (m *MemoryManager) GetEpisode(id string) (*Episode, error) {
	return m.longTerm.GetEpisode(id)
}

// ========== 待确认操作 ==========

// CreatePendingAction 创建待确认操作
func (m *MemoryManager) CreatePendingAction(action *PendingAction) error {
	return m.createPendingAction(action)
}

// GetPendingAction 获取待确认操作
func (m *MemoryManager) GetPendingAction(id string) (*PendingAction, error) {
	return m.getPendingAction(id)
}

// GetLatestPendingAction 获取最新的待确认操作
func (m *MemoryManager) GetLatestPendingAction(sessionID, userID string) (*PendingAction, error) {
	return m.getLatestPendingAction(sessionID, userID)
}

// UpdatePendingActionStatus 更新待确认操作状态
func (m *MemoryManager) UpdatePendingActionStatus(id, status, errorMsg string) error {
	return m.updatePendingActionStatus(id, status, errorMsg)
}

// pending_actions 的数据库操作
func (m *MemoryManager) createPendingAction(action *PendingAction) error {
	_, err := m.db.Exec(`
		INSERT INTO pending_actions (id, user_id, session_id, user_input, tool_name, args_json, prompt, status, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, action.ID, action.UserID, action.SessionID, action.UserInput, action.ToolName, action.ArgsJSON, action.Prompt, action.Status, action.ExpiresAt)
	return err
}

func (m *MemoryManager) getPendingAction(id string) (*PendingAction, error) {
	var action PendingAction
	var errorMsg sql.NullString
	err := m.db.QueryRow(`
		SELECT id, user_id, session_id, user_input, tool_name, args_json, prompt, status, error_msg, created_at, expires_at
		FROM pending_actions WHERE id = $1
	`, id).Scan(&action.ID, &action.UserID, &action.SessionID, &action.UserInput, &action.ToolName, &action.ArgsJSON, &action.Prompt, &action.Status, &errorMsg, &action.CreatedAt, &action.ExpiresAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	action.ErrorMsg = errorMsg.String
	return &action, nil
}

func (m *MemoryManager) getLatestPendingAction(sessionID, userID string) (*PendingAction, error) {
	var action PendingAction
	var errorMsg sql.NullString
	err := m.db.QueryRow(`
		SELECT id, user_id, session_id, user_input, tool_name, args_json, prompt, status, error_msg, created_at, expires_at
		FROM pending_actions
		WHERE session_id = $1 AND user_id = $2 AND status = 'pending' AND (expires_at IS NULL OR expires_at > NOW())
		ORDER BY created_at DESC
		LIMIT 1
	`, sessionID, userID).Scan(&action.ID, &action.UserID, &action.SessionID, &action.UserInput, &action.ToolName, &action.ArgsJSON, &action.Prompt, &action.Status, &errorMsg, &action.CreatedAt, &action.ExpiresAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	action.ErrorMsg = errorMsg.String
	return &action, nil
}

func (m *MemoryManager) updatePendingActionStatus(id, status, errorMsg string) error {
	_, err := m.db.Exec(`
		UPDATE pending_actions SET status = $1, error_msg = $2 WHERE id = $3
	`, status, errorMsg, id)
	return err
}

// ========== 上下文构建 ==========

// NewMemoryContext 记忆上下文
type NewMemoryContext struct {
	RecentMessages    []ChatMessage
	CurrentTask       *TaskState
	RelevantEpisodes  []Episode
	RelevantKnowledge []Knowledge
}

// BuildContext 构建记忆上下文
func (m *MemoryManager) BuildContext(sessionID, query string) (*NewMemoryContext, error) {
	ctx := &NewMemoryContext{}

	// 1. 短期记忆
	ctx.RecentMessages = m.shortTerm.GetMessage(sessionID)

	// 2. 工作记忆
	task, _ := m.working.GetCurrentTask(sessionID)
	ctx.CurrentTask = task

	// 3. 长期记忆 - Episodes
	if query != "" && m.embedding != nil {
		episodes, err := m.longTerm.SearchEpisodes(query, 3)
		if err == nil {
			ctx.RelevantEpisodes = episodes
		}

		// 4. 长期记忆 - Knowledge
		knowledge, err := m.longTerm.SearchKnowledge(query, 2)
		if err == nil {
			ctx.RelevantKnowledge = knowledge
		}
	}

	return ctx, nil
}

// FormatContextForLLM 格式化上下文供 LLM 使用
func (m *MemoryManager) FormatContextForLLM(ctx *NewMemoryContext) string {
	if ctx == nil {
		return ""
	}

	var sb strings.Builder

	// 最近对话
	if len(ctx.RecentMessages) > 0 {
		sb.WriteString("## 最近对话\n")
		for _, msg := range ctx.RecentMessages {
			role := "用户"
			if msg.Role == "assistant" {
				role = "助手"
			}
			sb.WriteString(fmt.Sprintf("%s: %s\n", role, msg.Content))
		}
		sb.WriteString("\n")
	}

	// 当前任务
	if ctx.CurrentTask != nil {
		sb.WriteString("## 当前任务\n")
		sb.WriteString(fmt.Sprintf("请求: %s\n", ctx.CurrentTask.OriginalRequest))
		sb.WriteString(fmt.Sprintf("状态: %s\n", ctx.CurrentTask.Status))
		if len(ctx.CurrentTask.Steps) > 0 {
			sb.WriteString("已执行步骤:\n")
			for i, step := range ctx.CurrentTask.Steps {
				sb.WriteString(fmt.Sprintf("  %d. %s\n", i+1, step.ToolName))
			}
		}
		sb.WriteString("\n")
	}

	// 相关经验
	if len(ctx.RelevantEpisodes) > 0 {
		sb.WriteString("## 相关经验\n")
		for _, ep := range ctx.RelevantEpisodes {
			sb.WriteString(fmt.Sprintf("- %s → %s\n", ep.TriggerSummary, ep.OutcomeSummary))
		}
		sb.WriteString("\n")
	}

	// 相关知识
	if len(ctx.RelevantKnowledge) > 0 {
		sb.WriteString("## 相关知识\n")
		for _, k := range ctx.RelevantKnowledge {
			sb.WriteString(fmt.Sprintf("### %s\n%s\n", k.Topic, k.Content))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// ========== 维护任务 ==========

// RunForgetGate 执行遗忘清理
func (m *MemoryManager) RunForgetGate() (int, error) {
	if m.forget == nil {
		return 0, nil
	}
	return m.forget.Cleanup()
}

// RunConsolidation 执行记忆整合
func (m *MemoryManager) RunConsolidation() (int, error) {
	if m.consolidate == nil {
		return 0, nil
	}
	return m.consolidate.Run()
}

// PinEpisode 标记 Episode 永不删除
func (m *MemoryManager) PinEpisode(episodeID string) error {
	return m.forget.PinEpisode(episodeID)
}

// UnpinEpisode 取消永不删除标记
func (m *MemoryManager) UnpinEpisode(episodeID string) error {
	return m.forget.UnpinEpisode(episodeID)
}

// ========== 辅助函数 ==========

// extractTarget 从步骤中提取目标
func extractTarget(steps []TaskStep) string {
	for _, step := range steps {
		if step.ToolName == "docker" {
			if args := step.Args; args != nil {
				if container, ok := args["container"].(string); ok {
					return container
				}
			}
		}
	}
	return ""
}
