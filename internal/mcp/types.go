package mcp

import "encoding/json"

// MCP 协议版本
const (
	MCPVersion     = "2024-11-05"
	JSONRPCVersion = "2.0"
)

// JSONRPCRequest JSON-RPC 2.0 请求结构
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// JSONRPCResponse JSON-RPC 2.0 响应结构
type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

// JSONRPCError JSON-RPC 2.0 错误结构
type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// JSON-RPC 标准错误码
const (
	ParseError     = -32700 // 解析错误
	InvalidRequest = -32600 // 无效请求
	MethodNotFound = -32601 // 方法不存在
	InvalidParams  = -32602 // 无效参数
	InternalError  = -32603 // 内部错误
)

// Tool MCP 工具定义
type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"inputSchema"`
}

// Resource MCP 资源定义
type Resource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

// ResourceTemplate 资源模板定义
type ResourceTemplate struct {
	URITemplate string `json:"uriTemplate"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

// ResourceContents 资源内容
type ResourceContents struct {
	URI      string `json:"uri"`
	MimeType string `json:"mimeType,omitempty"`
	Text     string `json:"text,omitempty"`
	Blob     string `json:"blob,omitempty"` // base64 编码
}

// Prompt MCP 提示词定义
type Prompt struct {
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	Arguments   []PromptArgument `json:"arguments,omitempty"`
}

// PromptArgument 提示词参数
type PromptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

// PromptMessage 提示词消息
type PromptMessage struct {
	Role    string  `json:"role"`
	Content Content `json:"content"`
}

// Content 内容结构
type Content struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
	Data     string `json:"data,omitempty"` // base64 编码的图片等
}

// InitializeParams 初始化请求参数
type InitializeParams struct {
	ProtocolVersion string           `json:"protocolVersion"`
	Capabilities    ClientCapability `json:"capabilities"`
	ClientInfo      Implementation   `json:"clientInfo"`
}

// InitializeResult 初始化响应结果
type InitializeResult struct {
	ProtocolVersion string           `json:"protocolVersion"`
	Capabilities    ServerCapability `json:"capabilities"`
	ServerInfo      Implementation   `json:"serverInfo"`
	Instructions    string           `json:"instructions,omitempty"`
}

// Implementation 实现信息
type Implementation struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ClientCapability 客户端能力
type ClientCapability struct {
	Roots    *RootsCapability    `json:"roots,omitempty"`
	Sampling *SamplingCapability `json:"sampling,omitempty"`
}

// ServerCapability 服务端能力
type ServerCapability struct {
	Tools     *ToolsCapability     `json:"tools,omitempty"`
	Resources *ResourcesCapability `json:"resources,omitempty"`
	Prompts   *PromptsCapability   `json:"prompts,omitempty"`
	Logging   *LoggingCapability   `json:"logging,omitempty"`
}

// RootsCapability 根目录能力
type RootsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// SamplingCapability 采样能力
type SamplingCapability struct{}

// ToolsCapability 工具能力
type ToolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// ResourcesCapability 资源能力
type ResourcesCapability struct {
	Subscribe   bool `json:"subscribe,omitempty"`
	ListChanged bool `json:"listChanged,omitempty"`
}

// PromptsCapability 提示词能力
type PromptsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// LoggingCapability 日志能力
type LoggingCapability struct{}

// ToolsListResult 工具列表响应
type ToolsListResult struct {
	Tools []Tool `json:"tools"`
}

// ToolsCallParams 工具调用参数
type ToolsCallParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

// ToolsCallResult 工具调用结果
type ToolsCallResult struct {
	Content []Content `json:"content"`
	IsError bool      `json:"isError,omitempty"`
}

// ResourcesListResult 资源列表响应
type ResourcesListResult struct {
	Resources []Resource `json:"resources"`
}

// ResourcesReadParams 资源读取参数
type ResourcesReadParams struct {
	URI string `json:"uri"`
}

// ResourcesReadResult 资源读取结果
type ResourcesReadResult struct {
	Contents []ResourceContents `json:"contents"`
}

// PromptsListResult 提示词列表响应
type PromptsListResult struct {
	Prompts []Prompt `json:"prompts"`
}

// PromptsGetParams 获取提示词参数
type PromptsGetParams struct {
	Name      string            `json:"name"`
	Arguments map[string]string `json:"arguments,omitempty"`
}

// PromptsGetResult 获取提示词结果
type PromptsGetResult struct {
	Description string          `json:"description,omitempty"`
	Messages    []PromptMessage `json:"messages"`
}

// MCPError MCP 错误
type MCPError struct {
	Code    int
	Message string
	Data    any
}

// Error 实现 error 接口
func (e *MCPError) Error() string {
	return e.Message
}

// NewMCPError 创建 MCP 错误
func NewMCPError(code int, message string, data any) *MCPError {
	return &MCPError{
		Code:    code,
		Message: message,
		Data:    data,
	}
}

// ToJSONRPCError 转换为 JSON-RPC 错误
func (e *MCPError) ToJSONRPCError() *JSONRPCError {
	return &JSONRPCError{
		Code:    e.Code,
		Message: e.Message,
		Data:    e.Data,
	}
}
