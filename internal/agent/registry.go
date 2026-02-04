package agent

import (
	"fmt"
	"serverowl/internal/llm"
)

func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]*Tool),
	}
}

func (r *ToolRegistry) Register(tool *Tool) error {
	if tool.Name == "" {
		return fmt.Errorf("tool name cannot be empty")
	}
	if _, ok := r.tools[tool.Name]; ok {
		return fmt.Errorf("tool %s already registered", tool.Name)
	}
	r.tools[tool.Name] = tool
	return nil
}
func (r *ToolRegistry) Get(name string) (*Tool, error) {
	tool, exists := r.tools[name]
	if !exists {
		return nil, fmt.Errorf("tool %s not found", name)
	}
	return tool, nil
}
func (r *ToolRegistry) GetAll() []*Tool {
	tools := make([]*Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, tool)
	}
	return tools
}

// 转换为 LLM 的 Tool 格式
func (r *ToolRegistry) ToLLMTools() []llm.Tool {
	llmTools := make([]llm.Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		llmTools = append(llmTools, llm.Tool{
			Type: "function",
			Function: llm.Function{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.Parameters,
			},
		})
	}
	return llmTools
}

// Count 返回已注册工具数量
func (r *ToolRegistry) Count() int {
	return len(r.tools)
}
