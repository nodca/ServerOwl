package main

import (
	"encoding/json"
	"fmt"
	"serverowl/internal/agent"
	"serverowl/internal/llm"
)

func main() {
	fmt.Println("🔍 调试 Agent 请求体")
	fmt.Println("===================\n")

	// 创建工具注册表
	registry := agent.NewToolRegistry()
	registry.Register(agent.CreateDockerTool())

	// 构造消息
	messages := []llm.Message{
		{
			Role:    "system",
			Content: "你是运维助手",
		},
		{
			Role:    "user",
			Content: "重启 nginx",
		},
	}

	// 转换为 LLM Tools
	tools := registry.ToLLMTools()

	// 构造请求
	req := map[string]interface{}{
		"model":    "deepseek-ai/DeepSeek-V3.2",
		"messages": messages,
		"tools":    tools,
	}

	// 打印请求体
	reqJSON, _ := json.MarshalIndent(req, "", "  ")
	fmt.Println("请求体:")
	fmt.Println(string(reqJSON))
}
