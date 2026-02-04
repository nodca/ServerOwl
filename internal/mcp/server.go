package mcp

import (
	"encoding/json"
	"fmt"
	"serverowl/internal/agent"
	"sync"
)

// MCPServer MCP 服务器
type MCPServer struct {
	toolRegistry *agent.ToolRegistry
	resources    []Resource
	prompts      []Prompt
	initialized  bool
	mu           sync.RWMutex

	// 服务器信息
	serverName    string
	serverVersion string

	// 资源提供者（可选）
	resourceProvider ResourceProvider

	// 提示词提供者（可选）
	promptProvider PromptProvider
}

// ResourceProvider 资源提供者接口
type ResourceProvider interface {
	ListResources() []Resource
	ReadResource(uri string) (*ResourceContents, error)
}

// PromptProvider 提示词提供者接口
type PromptProvider interface {
	ListPrompts() []Prompt
	GetPrompt(name string, arguments map[string]string) (*PromptsGetResult, error)
}

// MCPServerOption 服务器配置选项
type MCPServerOption func(*MCPServer)

// WithServerInfo 设置服务器信息
func WithServerInfo(name, version string) MCPServerOption {
	return func(s *MCPServer) {
		s.serverName = name
		s.serverVersion = version
	}
}

// WithResourceProvider 设置资源提供者
func WithResourceProvider(provider ResourceProvider) MCPServerOption {
	return func(s *MCPServer) {
		s.resourceProvider = provider
	}
}

// WithPromptProvider 设置提示词提供者
func WithPromptProvider(provider PromptProvider) MCPServerOption {
	return func(s *MCPServer) {
		s.promptProvider = provider
	}
}

// NewMCPServer 创建 MCP 服务器
func NewMCPServer(toolRegistry *agent.ToolRegistry, opts ...MCPServerOption) *MCPServer {
	s := &MCPServer{
		toolRegistry:  toolRegistry,
		resources:     make([]Resource, 0),
		prompts:       make([]Prompt, 0),
		serverName:    "ServerOwl",
		serverVersion: "1.0.0",
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// HandleRequest 处理 JSON-RPC 请求
func (s *MCPServer) HandleRequest(request []byte) ([]byte, error) {
	// 解析请求
	var req JSONRPCRequest
	if err := json.Unmarshal(request, &req); err != nil {
		return s.errorResponse(nil, ParseError, "解析请求失败", err.Error())
	}

	// 验证 JSON-RPC 版本
	if req.JSONRPC != JSONRPCVersion {
		return s.errorResponse(req.ID, InvalidRequest, "无效的 JSON-RPC 版本", nil)
	}

	// 路由请求到对应处理器
	var result any
	var mcpErr *MCPError

	switch req.Method {
	case "initialize":
		result, mcpErr = s.handleInitialize(req.Params)
	case "initialized":
		// 客户端确认初始化完成，无需响应
		return nil, nil
	case "ping":
		result = map[string]any{}
	case "tools/list":
		result, mcpErr = s.handleToolsList(req.Params)
	case "tools/call":
		result, mcpErr = s.handleToolsCall(req.Params)
	case "resources/list":
		result, mcpErr = s.handleResourcesList(req.Params)
	case "resources/read":
		result, mcpErr = s.handleResourcesRead(req.Params)
	case "prompts/list":
		result, mcpErr = s.handlePromptsList(req.Params)
	case "prompts/get":
		result, mcpErr = s.handlePromptsGet(req.Params)
	default:
		return s.errorResponse(req.ID, MethodNotFound, fmt.Sprintf("方法不存在: %s", req.Method), nil)
	}

	// 处理错误
	if mcpErr != nil {
		return s.errorResponse(req.ID, mcpErr.Code, mcpErr.Message, mcpErr.Data)
	}

	// 构建成功响应
	return s.successResponse(req.ID, result)
}

// HandleBatchRequest 处理批量请求
func (s *MCPServer) HandleBatchRequest(requests []byte) ([]byte, error) {
	// 尝试解析为数组
	var batch []json.RawMessage
	if err := json.Unmarshal(requests, &batch); err != nil {
		// 不是数组，作为单个请求处理
		return s.HandleRequest(requests)
	}

	// 处理批量请求
	responses := make([]json.RawMessage, 0, len(batch))
	for _, reqBytes := range batch {
		resp, err := s.HandleRequest(reqBytes)
		if err != nil {
			continue
		}
		if resp != nil {
			responses = append(responses, resp)
		}
	}

	if len(responses) == 0 {
		return nil, nil
	}

	return json.Marshal(responses)
}

// IsInitialized 检查是否已初始化
func (s *MCPServer) IsInitialized() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.initialized
}

// GetServerInfo 获取服务器信息
func (s *MCPServer) GetServerInfo() Implementation {
	return Implementation{
		Name:    s.serverName,
		Version: s.serverVersion,
	}
}

// GetCapabilities 获取服务器能力
func (s *MCPServer) GetCapabilities() ServerCapability {
	caps := ServerCapability{
		Tools: &ToolsCapability{
			ListChanged: false,
		},
		Logging: &LoggingCapability{},
	}

	// 如果有资源提供者，启用资源能力
	if s.resourceProvider != nil {
		caps.Resources = &ResourcesCapability{
			Subscribe:   false,
			ListChanged: false,
		}
	}

	// 如果有提示词提供者，启用提示词能力
	if s.promptProvider != nil {
		caps.Prompts = &PromptsCapability{
			ListChanged: false,
		}
	}

	return caps
}

// successResponse 构建成功响应
func (s *MCPServer) successResponse(id json.RawMessage, result any) ([]byte, error) {
	resp := JSONRPCResponse{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Result:  result,
	}
	return json.Marshal(resp)
}

// errorResponse 构建错误响应
func (s *MCPServer) errorResponse(id json.RawMessage, code int, message string, data any) ([]byte, error) {
	resp := JSONRPCResponse{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Error: &JSONRPCError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
	return json.Marshal(resp)
}
