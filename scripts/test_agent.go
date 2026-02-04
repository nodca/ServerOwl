package main

import (
	"fmt"
	"serverowl/internal/agent"
	"serverowl/internal/llm"
)

func main() {
	fmt.Println("🧪 Agent 系统集成测试")
	fmt.Println("======================\n")

	// 1. 初始化 LLM Client
	llmClient := llm.NewDeepSeekClient(
		"sk-bpbtodncdyqvavluhnxysqdxnxlnyzhzdpbjwxaxtuspybnz",
		"https://api.siliconflow.cn/v1",
		"deepseek-ai/DeepSeek-V3.2",
	)
	fmt.Println("✅ LLM Client 创建成功")

	// 2. 创建工具注册表
	registry := agent.NewToolRegistry()

	// 3. 注册 docker 工具
	if err := registry.Register(agent.CreateDockerTool()); err != nil {
		fmt.Printf("❌ 注册工具失败: %v\n", err)
		return
	}
	fmt.Println("✅ Docker 工具注册成功")

	// 4. 创建 Agent
	executor := agent.NewAgentExecutor(llmClient, registry, nil, nil, nil)
	fmt.Println("✅ Agent Executor 创建成功\n")

	// 5. 测试用例
	testCases := []struct {
		name  string
		input string
	}{
		{"简单对话", "你好，介绍一下你自己"},
		{"查看状态", "查看 postgres 容器状态"},
		{"重启容器", "重启 postgres"},
	}

	for i, tc := range testCases {
		fmt.Printf("📝 测试 %d: %s\n", i+1, tc.name)
		fmt.Printf("   输入: %s\n", tc.input)

		result, err := executor.Execute("test_session", tc.input, "test_request")
		if err != nil {
			fmt.Printf("   ❌ 失败: %v\n\n", err)
			continue
		}

		fmt.Printf("   ✅ 成功\n")
		fmt.Printf("   输出: %s\n\n", result)
	}

	fmt.Println("🎉 测试完成")
}
