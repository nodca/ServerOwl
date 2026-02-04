package mcp

import (
	"encoding/json"
	"fmt"
)

// handleInitialize 处理初始化请求
func (s *MCPServer) handleInitialize(params json.RawMessage) (any, *MCPError) {
	var initParams InitializeParams
	if params != nil {
		if err := json.Unmarshal(params, &initParams); err != nil {
			return nil, NewMCPError(InvalidParams, "无效的初始化参数", err.Error())
		}
	}

	// 标记为已初始化
	s.mu.Lock()
	s.initialized = true
	s.mu.Unlock()

	// 返回初始化结果
	result := InitializeResult{
		ProtocolVersion: MCPVersion,
		Capabilities:    s.GetCapabilities(),
		ServerInfo:      s.GetServerInfo(),
		Instructions:    "ServerOwl 是一个智能运维助手，提供服务器监控、容器管理、命令执行等功能。",
	}

	return result, nil
}

// handleToolsList 处理工具列表请求
func (s *MCPServer) handleToolsList(params json.RawMessage) (any, *MCPError) {
	// 检查是否已初始化
	if !s.IsInitialized() {
		return nil, NewMCPError(InvalidRequest, "服务器未初始化", nil)
	}

	// 从 ToolRegistry 获取工具并转换为 MCP 格式
	tools := s.convertToolsToMCP()

	return ToolsListResult{
		Tools: tools,
	}, nil
}

// handleToolsCall 处理工具调用请求
func (s *MCPServer) handleToolsCall(params json.RawMessage) (any, *MCPError) {
	// 检查是否已初始化
	if !s.IsInitialized() {
		return nil, NewMCPError(InvalidRequest, "服务器未初始化", nil)
	}

	// 解析参数
	var callParams ToolsCallParams
	if err := json.Unmarshal(params, &callParams); err != nil {
		return nil, NewMCPError(InvalidParams, "无效的工具调用参数", err.Error())
	}

	// 验证工具名称
	if callParams.Name == "" {
		return nil, NewMCPError(InvalidParams, "工具名称不能为空", nil)
	}

	// 从注册表获取工具
	tool, err := s.toolRegistry.Get(callParams.Name)
	if err != nil {
		return nil, NewMCPError(InvalidParams, fmt.Sprintf("工具不存在: %s", callParams.Name), nil)
	}

	// 执行工具
	result, err := tool.Handler(callParams.Arguments)
	if err != nil {
		// 工具执行失败，返回错误内容
		return ToolsCallResult{
			Content: []Content{
				{
					Type: "text",
					Text: fmt.Sprintf("工具执行失败: %s", err.Error()),
				},
			},
			IsError: true,
		}, nil
	}

	// 返回成功结果
	return ToolsCallResult{
		Content: []Content{
			{
				Type: "text",
				Text: result,
			},
		},
		IsError: false,
	}, nil
}

// handleResourcesList 处理资源列表请求
func (s *MCPServer) handleResourcesList(params json.RawMessage) (any, *MCPError) {
	// 检查是否已初始化
	if !s.IsInitialized() {
		return nil, NewMCPError(InvalidRequest, "服务器未初始化", nil)
	}

	// 如果有资源提供者，使用它获取资源
	if s.resourceProvider != nil {
		resources := s.resourceProvider.ListResources()
		return ResourcesListResult{
			Resources: resources,
		}, nil
	}

	// 返回内置资源列表
	return ResourcesListResult{
		Resources: s.resources,
	}, nil
}

// handleResourcesRead 处理资源读取请求
func (s *MCPServer) handleResourcesRead(params json.RawMessage) (any, *MCPError) {
	// 检查是否已初始化
	if !s.IsInitialized() {
		return nil, NewMCPError(InvalidRequest, "服务器未初始化", nil)
	}

	// 解析参数
	var readParams ResourcesReadParams
	if err := json.Unmarshal(params, &readParams); err != nil {
		return nil, NewMCPError(InvalidParams, "无效的资源读取参数", err.Error())
	}

	// 验证 URI
	if readParams.URI == "" {
		return nil, NewMCPError(InvalidParams, "资源 URI 不能为空", nil)
	}

	// 如果有资源提供者，使用它读取资源
	if s.resourceProvider != nil {
		contents, err := s.resourceProvider.ReadResource(readParams.URI)
		if err != nil {
			return nil, NewMCPError(InvalidParams, fmt.Sprintf("读取资源失败: %s", err.Error()), nil)
		}
		return ResourcesReadResult{
			Contents: []ResourceContents{*contents},
		}, nil
	}

	// 默认返回空内容
	return nil, NewMCPError(InvalidParams, fmt.Sprintf("资源不存在: %s", readParams.URI), nil)
}

// handlePromptsList 处理提示词列表请求
func (s *MCPServer) handlePromptsList(params json.RawMessage) (any, *MCPError) {
	// 检查是否已初始化
	if !s.IsInitialized() {
		return nil, NewMCPError(InvalidRequest, "服务器未初始化", nil)
	}

	// 如果有提示词提供者，使用它获取提示词
	if s.promptProvider != nil {
		prompts := s.promptProvider.ListPrompts()
		return PromptsListResult{
			Prompts: prompts,
		}, nil
	}

	// 返回内置提示词列表
	return PromptsListResult{
		Prompts: s.prompts,
	}, nil
}

// handlePromptsGet 处理获取提示词请求
func (s *MCPServer) handlePromptsGet(params json.RawMessage) (any, *MCPError) {
	// 检查是否已初始化
	if !s.IsInitialized() {
		return nil, NewMCPError(InvalidRequest, "服务器未初始化", nil)
	}

	// 解析参数
	var getParams PromptsGetParams
	if err := json.Unmarshal(params, &getParams); err != nil {
		return nil, NewMCPError(InvalidParams, "无效的提示词获取参数", err.Error())
	}

	// 验证名称
	if getParams.Name == "" {
		return nil, NewMCPError(InvalidParams, "提示词名称不能为空", nil)
	}

	// 如果有提示词提供者，使用它获取提示词
	if s.promptProvider != nil {
		result, err := s.promptProvider.GetPrompt(getParams.Name, getParams.Arguments)
		if err != nil {
			return nil, NewMCPError(InvalidParams, fmt.Sprintf("获取提示词失败: %s", err.Error()), nil)
		}
		return result, nil
	}

	return nil, NewMCPError(InvalidParams, fmt.Sprintf("提示词不存在: %s", getParams.Name), nil)
}

// convertToolsToMCP 将 agent.Tool 转换为 MCP Tool 格式
func (s *MCPServer) convertToolsToMCP() []Tool {
	if s.toolRegistry == nil {
		return []Tool{}
	}

	agentTools := s.toolRegistry.GetAll()
	mcpTools := make([]Tool, 0, len(agentTools))

	for _, t := range agentTools {
		mcpTool := Tool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: convertParametersToInputSchema(t.Parameters),
		}
		mcpTools = append(mcpTools, mcpTool)
	}

	return mcpTools
}

// convertParametersToInputSchema 将参数定义转换为 JSON Schema 格式
func convertParametersToInputSchema(params map[string]any) map[string]any {
	if params == nil {
		return map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		}
	}

	// 确保有 type 字段
	schema := make(map[string]any)
	for k, v := range params {
		schema[k] = v
	}

	if _, ok := schema["type"]; !ok {
		schema["type"] = "object"
	}

	return schema
}
