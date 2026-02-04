package handlers

import (
	"net/http"
	"strconv"
	"time"

	"serverowl/internal/agent"
)

// LogsHandler 日志 API 处理器
type LogsHandler struct {
	logger *agent.ActionLogger
}

// NewLogsHandler 创建日志处理器
func NewLogsHandler(logger *agent.ActionLogger) *LogsHandler {
	return &LogsHandler{
		logger: logger,
	}
}

// ActionLogResponse 操作日志响应
type ActionLogResponse struct {
	ID         int64     `json:"id"`
	SessionID  string    `json:"session_id"`
	UserID     string    `json:"user_id"`
	RequestID  string    `json:"request_id,omitempty"`
	ToolName   string    `json:"tool_name"`
	Arguments  string    `json:"arguments"`
	Result     string    `json:"result,omitempty"`
	Success    bool      `json:"success"`
	ErrorMsg   string    `json:"error_msg,omitempty"`
	DurationMs int64     `json:"duration_ms"`
	CreatedAt  time.Time `json:"created_at"`
}

// LogAnalysis 日志分析结果
type LogAnalysis struct {
	TotalCount     int                `json:"total_count"`
	SuccessCount   int                `json:"success_count"`
	FailureCount   int                `json:"failure_count"`
	SuccessRate    float64            `json:"success_rate"`
	AvgDurationMs  float64            `json:"avg_duration_ms"`
	ToolUsage      map[string]int     `json:"tool_usage"`
	ErrorSummary   map[string]int     `json:"error_summary"`
	TimeRange      TimeRange          `json:"time_range"`
}

// TimeRange 时间范围
type TimeRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// GetActionLogs 获取操作日志
// GET /api/v1/logs/actions
// 查询参数:
//   - limit: 返回数量限制 (默认 50)
//   - user_id: 按用户过滤
//   - tool_name: 按工具过滤
//   - success: 按成功状态过滤 (true/false)
func (h *LogsHandler) GetActionLogs(w http.ResponseWriter, r *http.Request) {
	if h.logger == nil {
		writeInternalError(w, "日志记录器未初始化")
		return
	}

	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}

	// 获取查询参数
	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	// 获取日志
	logs, err := h.logger.GetRecentLogs(limit)
	if err != nil {
		writeInternalError(w, "获取日志失败: "+err.Error())
		return
	}

	// 应用过滤器
	userID := r.URL.Query().Get("user_id")
	toolName := r.URL.Query().Get("tool_name")
	successStr := r.URL.Query().Get("success")

	var filteredLogs []agent.ActionLog
	for _, log := range logs {
		// 用户过滤
		if userID != "" && log.UserID != userID {
			continue
		}
		// 工具过滤
		if toolName != "" && log.ToolName != toolName {
			continue
		}
		// 成功状态过滤
		if successStr != "" {
			success := successStr == "true"
			if log.Success != success {
				continue
			}
		}
		filteredLogs = append(filteredLogs, log)
	}

	// 转换为响应格式
	responses := make([]ActionLogResponse, 0, len(filteredLogs))
	for _, log := range filteredLogs {
		responses = append(responses, ActionLogResponse{
			ID:         log.ID,
			SessionID:  log.SessionID,
			UserID:     log.UserID,
			RequestID:  log.RequestID,
			ToolName:   log.ToolName,
			Arguments:  log.Arguments,
			Result:     log.Result,
			Success:    log.Success,
			ErrorMsg:   log.ErrorMsg,
			DurationMs: log.DurationMs,
			CreatedAt:  log.CreatedAt,
		})
	}

	writeSuccess(w, responses)
}

// AnalyzeLogs 分析日志
// GET /api/v1/logs/analyze
// 查询参数:
//   - limit: 分析的日志数量 (默认 1000)
//   - hours: 分析最近多少小时的日志 (默认 24)
func (h *LogsHandler) AnalyzeLogs(w http.ResponseWriter, r *http.Request) {
	if h.logger == nil {
		writeInternalError(w, "日志记录器未初始化")
		return
	}

	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}

	// 获取查询参数
	limitStr := r.URL.Query().Get("limit")
	limit := 1000
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	// 获取日志
	logs, err := h.logger.GetRecentLogs(limit)
	if err != nil {
		writeInternalError(w, "获取日志失败: "+err.Error())
		return
	}

	// 分析日志
	analysis := LogAnalysis{
		ToolUsage:    make(map[string]int),
		ErrorSummary: make(map[string]int),
	}

	if len(logs) == 0 {
		writeSuccess(w, analysis)
		return
	}

	var totalDuration int64
	var minTime, maxTime time.Time

	for i, log := range logs {
		analysis.TotalCount++

		if log.Success {
			analysis.SuccessCount++
		} else {
			analysis.FailureCount++
			if log.ErrorMsg != "" {
				// 简化错误消息用于统计
				errKey := truncateString(log.ErrorMsg, 50)
				analysis.ErrorSummary[errKey]++
			}
		}

		totalDuration += log.DurationMs
		analysis.ToolUsage[log.ToolName]++

		// 更新时间范围
		if i == 0 {
			minTime = log.CreatedAt
			maxTime = log.CreatedAt
		} else {
			if log.CreatedAt.Before(minTime) {
				minTime = log.CreatedAt
			}
			if log.CreatedAt.After(maxTime) {
				maxTime = log.CreatedAt
			}
		}
	}

	// 计算统计数据
	if analysis.TotalCount > 0 {
		analysis.SuccessRate = float64(analysis.SuccessCount) / float64(analysis.TotalCount) * 100
		analysis.AvgDurationMs = float64(totalDuration) / float64(analysis.TotalCount)
	}

	analysis.TimeRange = TimeRange{
		Start: minTime,
		End:   maxTime,
	}

	writeSuccess(w, analysis)
}

// truncateString 截断字符串
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
