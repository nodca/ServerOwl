package handlers

import (
	"context"
	"net/http"
	"strings"
	"time"

	"serverowl/internal/skills"
)

// SkillsHandler 技能管理 API 处理器
type SkillsHandler struct {
	manager *skills.Manager
}

// NewSkillsHandler 创建技能处理器
func NewSkillsHandler(mgr *skills.Manager) *SkillsHandler {
	return &SkillsHandler{
		manager: mgr,
	}
}

// SkillResponse 技能响应
type SkillResponse struct {
	Name        string                  `json:"name"`
	Description string                  `json:"description"`
	Version     string                  `json:"version"`
	Author      string                  `json:"author"`
	Tags        []string                `json:"tags"`
	Parameters  []SkillParameterInfo    `json:"parameters"`
	StepCount   int                     `json:"step_count"`
	NeedConfirm bool                    `json:"need_confirm"`
	Timeout     string                  `json:"timeout,omitempty"`
}

// SkillParameterInfo 技能参数信息
type SkillParameterInfo struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
	Default     any    `json:"default,omitempty"`
}

// SkillExecuteRequest 执行技能请求
type SkillExecuteRequest struct {
	Params map[string]any `json:"params"`
}

// SkillExecuteResponse 执行技能响应
type SkillExecuteResponse struct {
	SkillName string              `json:"skill_name"`
	Success   bool                `json:"success"`
	Steps     []StepResultInfo    `json:"steps"`
	StartTime time.Time           `json:"start_time"`
	EndTime   time.Time           `json:"end_time"`
	Duration  string              `json:"duration"`
	Error     string              `json:"error,omitempty"`
}

// StepResultInfo 步骤结果信息
type StepResultInfo struct {
	StepName string `json:"step_name"`
	Tool     string `json:"tool"`
	Success  bool   `json:"success"`
	Output   string `json:"output,omitempty"`
	Error    string `json:"error,omitempty"`
	Duration string `json:"duration"`
}

// HandleSkills 处理技能列表请求
// GET /api/v1/skills - 列出所有技能
func (h *SkillsHandler) HandleSkills(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		writeInternalError(w, "技能管理器未初始化")
		return
	}

	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}

	// 获取查询参数
	tag := r.URL.Query().Get("tag")

	var skillList []*skills.Skill
	if tag != "" {
		skillList = h.manager.ListSkillsByTag(tag)
	} else {
		skillList = h.manager.ListSkills()
	}

	responses := make([]SkillResponse, 0, len(skillList))
	for _, s := range skillList {
		responses = append(responses, skillToResponse(s))
	}

	writeSuccess(w, responses)
}

// HandleSkillByName 处理单个技能请求
// GET /api/v1/skills/{name} - 获取技能详情
// POST /api/v1/skills/{name}/execute - 执行技能
func (h *SkillsHandler) HandleSkillByName(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		writeInternalError(w, "技能管理器未初始化")
		return
	}

	// 解析路径: /api/v1/skills/{name} 或 /api/v1/skills/{name}/execute
	path := r.URL.Path
	prefix := "/api/v1/skills/"
	remaining := strings.TrimPrefix(path, prefix)

	// 检查是否是 execute 请求
	if strings.HasSuffix(remaining, "/execute") {
		skillName := strings.TrimSuffix(remaining, "/execute")
		if r.Method == http.MethodPost {
			h.executeSkill(w, r, skillName)
		} else {
			writeMethodNotAllowed(w)
		}
		return
	}

	skillName := remaining

	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}

	h.getSkill(w, r, skillName)
}

// getSkill 获取技能详情
func (h *SkillsHandler) getSkill(w http.ResponseWriter, r *http.Request, name string) {
	skill, ok := h.manager.GetSkill(name)
	if !ok {
		writeNotFound(w, "技能不存在: "+name)
		return
	}

	writeSuccess(w, skillToResponse(skill))
}

// executeSkill 执行技能
func (h *SkillsHandler) executeSkill(w http.ResponseWriter, r *http.Request, name string) {
	skill, ok := h.manager.GetSkill(name)
	if !ok {
		writeNotFound(w, "技能不存在: "+name)
		return
	}

	// 解析请求参数
	var req SkillExecuteRequest
	if err := parseJSON(r, &req); err != nil {
		// 允许空请求体
		req.Params = make(map[string]any)
	}

	// 验证参数
	if err := h.manager.ValidateParams(name, req.Params); err != nil {
		writeBadRequest(w, "参数验证失败: "+err.Error())
		return
	}

	// 检查是否需要确认
	if skill.NeedConfirm {
		// 检查是否有确认标志
		confirmed := r.URL.Query().Get("confirmed")
		if confirmed != "true" {
			writeJSON(w, http.StatusAccepted, APIResponse{
				Success: false,
				Error:   "此技能需要确认才能执行",
				Data: map[string]any{
					"need_confirm": true,
					"skill_name":   name,
					"description":  skill.Description,
				},
			})
			return
		}
	}

	// 设置超时上下文
	timeout := 5 * time.Minute
	if skill.Timeout > 0 {
		timeout = skill.Timeout
	}
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	// 执行技能
	result, err := h.manager.Execute(ctx, name, req.Params)
	if err != nil {
		writeInternalError(w, "执行技能失败: "+err.Error())
		return
	}

	// 构建响应
	response := SkillExecuteResponse{
		SkillName: result.SkillName,
		Success:   result.Success,
		StartTime: result.StartTime,
		EndTime:   result.EndTime,
		Duration:  result.EndTime.Sub(result.StartTime).String(),
		Error:     result.Error,
	}

	for _, step := range result.Steps {
		response.Steps = append(response.Steps, StepResultInfo{
			StepName: step.StepName,
			Tool:     step.Tool,
			Success:  step.Success,
			Output:   step.Output,
			Error:    step.Error,
			Duration: step.Duration.String(),
		})
	}

	if result.Success {
		writeSuccess(w, response)
	} else {
		writeJSON(w, http.StatusOK, APIResponse{
			Success: false,
			Data:    response,
			Error:   result.Error,
		})
	}
}

// skillToResponse 将技能转换为响应格式
func skillToResponse(s *skills.Skill) SkillResponse {
	params := make([]SkillParameterInfo, 0, len(s.Parameters))
	for _, p := range s.Parameters {
		params = append(params, SkillParameterInfo{
			Name:        p.Name,
			Type:        p.Type,
			Description: p.Description,
			Required:    p.Required,
			Default:     p.Default,
		})
	}

	resp := SkillResponse{
		Name:        s.Name,
		Description: s.Description,
		Version:     s.Version,
		Author:      s.Author,
		Tags:        s.Tags,
		Parameters:  params,
		StepCount:   len(s.Steps),
		NeedConfirm: s.NeedConfirm,
	}

	if s.Timeout > 0 {
		resp.Timeout = s.Timeout.String()
	}

	return resp
}
