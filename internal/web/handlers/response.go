package handlers

import (
	"encoding/json"
	"net/http"
)

// APIResponse 统一的 API 响应格式
type APIResponse struct {
	Success bool   `json:"success"`
	Data    any    `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
	Message string `json:"message,omitempty"`
}

// PaginatedResponse 分页响应
type PaginatedResponse struct {
	Items      any `json:"items"`
	Total      int `json:"total"`
	Page       int `json:"page"`
	PageSize   int `json:"page_size"`
	TotalPages int `json:"total_pages"`
}

// writeJSON 写入 JSON 响应
func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// writeSuccess 写入成功响应
func writeSuccess(w http.ResponseWriter, data any) {
	writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: data})
}

// writeSuccessMessage 写入成功消息响应
func writeSuccessMessage(w http.ResponseWriter, message string) {
	writeJSON(w, http.StatusOK, APIResponse{Success: true, Message: message})
}

// writeError 写入错误响应
func writeError(w http.ResponseWriter, status int, err string) {
	writeJSON(w, status, APIResponse{Success: false, Error: err})
}

// 常用错误响应的便捷函数
func writeBadRequest(w http.ResponseWriter, err string)    { writeError(w, http.StatusBadRequest, err) }
func writeNotFound(w http.ResponseWriter, err string)      { writeError(w, http.StatusNotFound, err) }
func writeInternalError(w http.ResponseWriter, err string) { writeError(w, http.StatusInternalServerError, err) }
func writeMethodNotAllowed(w http.ResponseWriter)          { writeError(w, http.StatusMethodNotAllowed, "方法不允许") }

// parseJSON 解析 JSON 请求体
func parseJSON(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}

// getPathParam 从路径中提取参数
func getPathParam(path, prefix string) string {
	if len(path) <= len(prefix) {
		return ""
	}
	return path[len(prefix):]
}
