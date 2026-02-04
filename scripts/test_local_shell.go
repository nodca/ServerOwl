package main

import (
	"fmt"
	"serverowl/internal/agent"
)

func main() {
	fmt.Println("🧪 测试 local_shell 工具")
	fmt.Println("========================\n")

	// 创建 SafetyChecker
	checker, err := agent.NewSafetyChecker("/opt/serverowl/configs/tool_whitelist.yaml")
	if err != nil {
		fmt.Printf("❌ SafetyChecker 失败: %v\n", err)
		return
	}

	// 创建 local_shell 工具
	tool := agent.CreateLocalShellTool()

	// 测试用例
	testCases := []struct {
		name    string
		command string
	}{
		{"列出当前目录", "ls -la"},
		{"查看 /opt/serverowl", "ls -la /opt/serverowl"},
		{"查看 .sh 文件", "ls /opt/serverowl/*.sh"},
		{"读取文件", "cat /opt/serverowl/update.sh"},
	}

	for i, tc := range testCases {
		fmt.Printf("📝 测试 %d: %s\n", i+1, tc.name)
		fmt.Printf("   命令: %s\n", tc.command)

		// 构造参数
		args := map[string]interface{}{
			"command":         tc.command,
			"_safety_checker": checker,
		}

		// 执行
		result, err := tool.Handler(args)
		if err != nil {
			fmt.Printf("   ❌ 失败: %v\n\n", err)
		} else {
			fmt.Printf("   ✅ 成功\n")
			fmt.Printf("   输出:\n%s\n\n", result)
		}
	}
}
