package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"serverowl/internal/memory"
)

// MemoryHandler 记忆管理 API 处理器
type MemoryHandler struct {
	manager *memory.MemoryManager
}

// NewMemoryHandler 创建记忆处理器
func NewMemoryHandler(mgr *memory.MemoryManager) *MemoryHandler {
	return &MemoryHandler{
		manager: mgr,
	}
}

// EpisodeResponse Episode 响应
type EpisodeResponse struct {
	ID             string     `json:"id"`
	SessionID      string     `json:"session_id"`
	UserID         string     `json:"user_id"`
	TriggerType    string     `json:"trigger_type"`
	TriggerSummary string     `json:"trigger_summary"`
	Outcome        string     `json:"outcome"`
	OutcomeSummary string     `json:"outcome_summary"`
	Importance     float64    `json:"importance"`
	AccessCount    int        `json:"access_count"`
	LastAccessedAt *time.Time `json:"last_accessed_at,omitempty"`
	Pinned         bool       `json:"pinned"`
	Target         string     `json:"target,omitempty"`
	Tags           []string   `json:"tags,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

// KnowledgeResponse Knowledge 响应
type KnowledgeResponse struct {
	ID             string     `json:"id"`
	Topic          string     `json:"topic"`
	Content        string     `json:"content"`
	KeyPoints      []string   `json:"key_points,omitempty"`
	SourceEpisodes []string   `json:"source_episodes,omitempty"`
	Confidence     float64    `json:"confidence"`
	AccessCount    int        `json:"access_count"`
	LastAccessedAt *time.Time `json:"last_accessed_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// HandleEpisodes 处理 Episodes 列表请求
// GET /api/v1/memory/episodes - 列出 Episodes
func (h *MemoryHandler) HandleEpisodes(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		writeInternalError(w, "记忆管理器未初始化")
		return
	}

	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}

	// 获取查询参数
	query := r.URL.Query().Get("query")
	limitStr := r.URL.Query().Get("limit")
	target := r.URL.Query().Get("target")

	limit := 20
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	var episodes []memory.Episode
	var err error

	if query != "" {
		// 语义搜索
		episodes, err = h.manager.SearchEpisodes(query, limit)
	} else if target != "" {
		// 按目标搜索 - 需要通过 longTerm 访问
		// 这里简化处理，返回空列表
		episodes = []memory.Episode{}
	} else {
		// 返回最近的 Episodes - 需要扩展 MemoryManager
		// 这里简化处理，返回空列表
		episodes = []memory.Episode{}
	}

	if err != nil {
		writeInternalError(w, "查询 Episodes 失败: "+err.Error())
		return
	}

	responses := make([]EpisodeResponse, 0, len(episodes))
	for _, ep := range episodes {
		responses = append(responses, episodeToResponse(&ep))
	}

	writeSuccess(w, responses)
}

// HandleEpisodeByID 处理单个 Episode 请求
// GET /api/v1/memory/episodes/{id} - 获取 Episode 详情
// DELETE /api/v1/memory/episodes/{id} - 删除 Episode
// POST /api/v1/memory/episodes/{id}/pin - 标记为永不删除
// DELETE /api/v1/memory/episodes/{id}/pin - 取消永不删除标记
func (h *MemoryHandler) HandleEpisodeByID(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		writeInternalError(w, "记忆管理器未初始化")
		return
	}

	// 解析路径
	path := r.URL.Path
	prefix := "/api/v1/memory/episodes/"
	remaining := strings.TrimPrefix(path, prefix)

	// 检查是否是 pin 请求
	if strings.HasSuffix(remaining, "/pin") {
		episodeID := strings.TrimSuffix(remaining, "/pin")
		switch r.Method {
		case http.MethodPost:
			h.pinEpisode(w, r, episodeID)
		case http.MethodDelete:
			h.unpinEpisode(w, r, episodeID)
		default:
			writeMethodNotAllowed(w)
		}
		return
	}

	episodeID := remaining

	switch r.Method {
	case http.MethodGet:
		h.getEpisode(w, r, episodeID)
	case http.MethodDelete:
		h.deleteEpisode(w, r, episodeID)
	default:
		writeMethodNotAllowed(w)
	}
}

// getEpisode 获取 Episode 详情
func (h *MemoryHandler) getEpisode(w http.ResponseWriter, r *http.Request, id string) {
	episode, err := h.manager.GetEpisode(id)
	if err != nil {
		writeInternalError(w, "获取 Episode 失败: "+err.Error())
		return
	}
	if episode == nil {
		writeNotFound(w, "Episode 不存在")
		return
	}

	writeSuccess(w, episodeToResponse(episode))
}

// deleteEpisode 删除 Episode
func (h *MemoryHandler) deleteEpisode(w http.ResponseWriter, r *http.Request, id string) {
	// 注意: MemoryManager 目前没有直接的删除方法
	// 这里返回未实现错误
	writeError(w, http.StatusNotImplemented, "删除 Episode 功能暂未实现")
}

// pinEpisode 标记 Episode 为永不删除
func (h *MemoryHandler) pinEpisode(w http.ResponseWriter, r *http.Request, id string) {
	if err := h.manager.PinEpisode(id); err != nil {
		writeInternalError(w, "标记失败: "+err.Error())
		return
	}

	writeSuccessMessage(w, "Episode 已标记为永不删除")
}

// unpinEpisode 取消永不删除标记
func (h *MemoryHandler) unpinEpisode(w http.ResponseWriter, r *http.Request, id string) {
	if err := h.manager.UnpinEpisode(id); err != nil {
		writeInternalError(w, "取消标记失败: "+err.Error())
		return
	}

	writeSuccessMessage(w, "已取消永不删除标记")
}

// HandleKnowledge 处理 Knowledge 列表请求
// GET /api/v1/memory/knowledge - 列出 Knowledge
func (h *MemoryHandler) HandleKnowledge(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		writeInternalError(w, "记忆管理器未初始化")
		return
	}

	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}

	// 获取查询参数
	query := r.URL.Query().Get("query")
	limitStr := r.URL.Query().Get("limit")

	limit := 20
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	var knowledge []memory.Knowledge
	var err error

	if query != "" {
		// 语义搜索
		knowledge, err = h.manager.SearchKnowledge(query, limit)
	} else {
		// 返回空列表 - 需要扩展 MemoryManager 支持列出所有 Knowledge
		knowledge = []memory.Knowledge{}
	}

	if err != nil {
		writeInternalError(w, "查询 Knowledge 失败: "+err.Error())
		return
	}

	responses := make([]KnowledgeResponse, 0, len(knowledge))
	for _, k := range knowledge {
		responses = append(responses, knowledgeToResponse(&k))
	}

	writeSuccess(w, responses)
}

// episodeToResponse 将 Episode 转换为响应格式
func episodeToResponse(ep *memory.Episode) EpisodeResponse {
	return EpisodeResponse{
		ID:             ep.ID,
		SessionID:      ep.SessionID,
		UserID:         ep.UserID,
		TriggerType:    ep.TriggerType,
		TriggerSummary: ep.TriggerSummary,
		Outcome:        ep.Outcome,
		OutcomeSummary: ep.OutcomeSummary,
		Importance:     ep.Importance,
		AccessCount:    ep.AccessCount,
		LastAccessedAt: ep.LastAccessedAt,
		Pinned:         ep.Pinned,
		Target:         ep.Target,
		Tags:           ep.Tags,
		CreatedAt:      ep.CreatedAt,
	}
}

// knowledgeToResponse 将 Knowledge 转换为响应格式
func knowledgeToResponse(k *memory.Knowledge) KnowledgeResponse {
	return KnowledgeResponse{
		ID:             k.ID,
		Topic:          k.Topic,
		Content:        k.Content,
		KeyPoints:      k.KeyPoints,
		SourceEpisodes: k.SourceEpisodes,
		Confidence:     k.Confidence,
		AccessCount:    k.AccessCount,
		LastAccessedAt: k.LastAccessedAt,
		CreatedAt:      k.CreatedAt,
		UpdatedAt:      k.UpdatedAt,
	}
}
