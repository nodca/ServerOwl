package main

import (
	"fmt"
	"serverowl/internal/agent"
	"serverowl/internal/llm"
)

func main() {
	fmt.Println("🧪 Agent 系统集成测试（模拟模式）")
	fmt.Println("================================\n")

	// 1. 初始化 LLM Client
	llmClient := llm.NewDeepSeekClient(
		"sk-bpbtodncdyqvavluhnxysqdxnxlnyzhzdpbjwxaxtuspybnz",
		"https://api.siliconflow.cn/v1",
		"deepseek-ai/DeepSeek-V3.2",
	)
	fmt.Println("✅ LLM Client 创建成功")

	// 2. 创建工具注册表
	registry := agent.NewToolRegistry()

	// 3. 注册模拟工具（不实际执行 docker 命令）
	mockDockerTool := &agent.Tool{
		Name:        "docker",
		Description: "Docker 容器操作，支持 restart、logs、status、exec 等操作",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"action": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"restart", "logs", "status", "exec"},
					"description": "操作类型",
				},
				"container": map[string]interface{}{
					"type":        "string",
					"description": "容器名称",
				},
				"lines": map[string]interface{}{
					"type":        "integer",
					"description": "日志行数（仅 logs 操作）",
				},
				"command": map[string]interface{}{
					"type":        "string",
					"description": "容器内执行的命令（仅 exec 操作）",
				},
			},
			"required": []string{"action", "container"},
		},
		Handler: func(args map[string]interface{}) (string, error) {
			action := args["action"].(string)
			container := args["container"].(string)

			switch action {
			case "restart":
				return fmt.Sprintf("✅ [模拟] 容器 %s 已重启", container), nil
			case "logs":
				lines := 50
				if l, ok := args["lines"].(float64); ok {
					lines = int(l)
				}
				return fmt.Sprintf("[模拟] %s 容器最近 %d 行日志:\n2024-01-22 10:00:00 INFO Server started\n2024-01-22 10:00:01 INFO Listening on port 8080", container, lines), nil
			case "status":
				return fmt.Sprintf("✅ [模拟] 容器 %s 状态: running", container), nil
			case "exec":
				command := args["command"].(string)
				return fmt.Sprintf("[模拟] 在 %s 中执行: %s\n输出: 命令执行成功", container, command), nil
			default:
				return "", fmt.Errorf("unknown action: %s", action)
			}
		},
		NeedConfirm: false,
	}

	if err := registry.Register(mockDockerTool); err != nil {
		fmt.Printf("❌ 注册工具失败: %v\n", err)
		return
	}
	fmt.Println("✅ 模拟 Docker 工具注册成功")

	// 4. 创建 Agent
	executor := agent.NewAgentExecutor(llmClient, registry, nil, nil, nil)
	fmt.Println("✅ Agent Executor 创建成功\n")

	// 5. 测试用例
	testCases := []struct {
		name  string
		input string
	}{
		{"简单对话", "你好，介绍一下你自己"},
		{"查看状态", "查看 nginx 容器状态"},
		{"重启容器", "重启 nginx"},
		{"查看日志", "看看 postgres 最近 20 行日志"},
		{"容器内执行命令", "在 redis 容器里执行 redis-cli INFO"},
	}

	successCount := 0
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
		successCount++
	}

	fmt.Printf("🎉 测试完成: %d/%d 通过\n", successCount, len(testCases))
}
