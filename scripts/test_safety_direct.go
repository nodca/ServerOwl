package main

import (
	"fmt"
	"serverowl/internal/agent"
)

func main() {
	fmt.Println("🔒 安全检查层直接测试")
	fmt.Println("======================\n")

	// 创建 SafetyChecker
	checker, err := agent.NewSafetyChecker("configs/tool_whitelist.yaml")
	if err != nil {
		fmt.Printf("❌ 创建 SafetyChecker 失败: %v\n", err)
		return
	}
	fmt.Println("✅ SafetyChecker 创建成功\n")

	// 测试用例
	testCases := []struct {
		name      string
		testFunc  func() error
		shouldFail bool
	}{
		// 黑名单测试
		{
			name: "黑名单 - rm -rf",
			testFunc: func() error {
				_, err := checker.CheckLocalShell("rm -rf /tmp/test")
				return err
			},
			shouldFail: true,
		},
		{
			name: "黑名单 - DROP TABLE",
			testFunc: func() error {
				_, err := checker.CheckDockerExec("postgres", "psql -c 'DROP TABLE users'")
				return err
			},
			shouldFail: true,
		},
		{
			name: "黑名单 - TRUNCATE",
			testFunc: func() error {
				_, err := checker.CheckDockerExec("postgres", "psql -c 'TRUNCATE users'")
				return err
			},
			shouldFail: true,
		},
		{
			name: "黑名单 - FLUSHALL",
			testFunc: func() error {
				_, err := checker.CheckDockerExec("redis", "redis-cli FLUSHALL")
				return err
			},
			shouldFail: true,
		},

		// 白名单测试
		{
			name: "白名单 - ls",
			testFunc: func() error {
				_, err := checker.CheckLocalShell("ls -la")
				return err
			},
			shouldFail: false,
		},
		{
			name: "白名单 - cat",
			testFunc: func() error {
				_, err := checker.CheckLocalShell("cat config.yaml")
				return err
			},
			shouldFail: false,
		},
		{
			name: "白名单 - docker exec ls",
			testFunc: func() error {
				_, err := checker.CheckDockerExec("postgres", "ls -la")
				return err
			},
			shouldFail: false,
		},
		{
			name: "白名单 - SELECT 查询",
			testFunc: func() error {
				_, err := checker.CheckDockerExec("postgres", "psql -c 'SELECT * FROM users'")
				return err
			},
			shouldFail: false,
		},

		// 不在白名单的命令
		{
			name: "不在白名单 - python",
			testFunc: func() error {
				_, err := checker.CheckLocalShell("python script.py")
				return err
			},
			shouldFail: true,
		},
	}

	successCount := 0
	for i, tc := range testCases {
		fmt.Printf("📝 测试 %d: %s\n", i+1, tc.name)

		err := tc.testFunc()

		if tc.shouldFail {
			// 应该失败
			if err != nil {
				fmt.Printf("   ✅ 正确拦截: %v\n\n", err)
				successCount++
			} else {
				fmt.Printf("   ❌ 应该被拦截但通过了\n\n")
			}
		} else {
			// 应该通过
			if err == nil {
				fmt.Printf("   ✅ 正确通过\n\n")
				successCount++
			} else {
				fmt.Printf("   ❌ 应该通过但被拦截: %v\n\n", err)
			}
		}
	}

	fmt.Printf("🎉 测试完成: %d/%d 通过\n", successCount, len(testCases))
}
