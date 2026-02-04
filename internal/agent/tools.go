package agent

import (
	"fmt"
	"os/exec"
	"runtime"
	"serverowl/internal/memory"
	"strings"
	"time"
)

func CreateDockerTool() *Tool {
	return &Tool{
		Name:        "docker",
		Description: "Docker 容器操作，支持 start、stop、restart、logs、status、inspect、exec 等操作",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"start", "stop", "restart", "logs", "status", "inspect", "exec"},
					"description": "操作类型：status 查状态，inspect 查详细配置（环境变量、网络等），logs 查日志",
				},
				"container": map[string]interface{}{
					"type":        "string",
					"description": "容器名称（status 操作可留空查看所有容器）",
				},
				"lines": map[string]interface{}{
					"type":        "integer",
					"description": "日志行数（仅 logs 操作）",
				},
				"command": map[string]interface{}{
					"type":        "string",
					"description": "容器内执行的命令（仅 exec 操作）",
				},
				"_confirmed": map[string]interface{}{
					"type":        "boolean",
					"description": "内部字段：表示用户已确认（仅系统使用）",
				},
			},
			"required": []string{"action"},
		},
		Handler:     handleDockerTool,
		NeedConfirm: false,
		Timeout:     30 * time.Second,
		RetryCount:  0,
	}
}

func handleDockerTool(args map[string]any) (string, error) {
	action, ok := args["action"].(string)
	if !ok {
		return "", fmt.Errorf("action must be a string")
	}

	container, _ := args["container"].(string)
	checker, _ := args["_safety_checker"].(*SafetyChecker)
	confirmed, _ := args["_confirmed"].(bool)

	switch action {
	case "start":
		if container == "" {
			return "", fmt.Errorf("container name required for start action")
		}
		if !confirmed {
			return "", &ConfirmRequiredError{
				ToolName: "docker",
				Args: map[string]any{
					"action":    "start",
					"container": container,
				},
				Reason: "容器操作需要确认",
			}
		}
		return dockerStart(container)
	case "stop":
		if container == "" {
			return "", fmt.Errorf("container name required for stop action")
		}
		if !confirmed {
			return "", &ConfirmRequiredError{
				ToolName: "docker",
				Args: map[string]any{
					"action":    "stop",
					"container": container,
				},
				Reason: "容器操作需要确认",
			}
		}
		return dockerStop(container)
	case "restart":
		if container == "" {
			return "", fmt.Errorf("container name required for restart action")
		}
		if !confirmed {
			return "", &ConfirmRequiredError{
				ToolName: "docker",
				Args: map[string]any{
					"action":    "restart",
					"container": container,
				},
				Reason: "容器操作需要确认",
			}
		}
		return dockerRestart(container)
	case "logs":
		if container == "" {
			return "", fmt.Errorf("container name required for logs action")
		}
		lines := 50 // 默认 50 行
		if l, ok := args["lines"].(float64); ok {
			lines = int(l)
		}
		return dockerLogs(container, lines)
	case "status":
		return dockerStatus(container)
	case "inspect":
		if container == "" {
			return "", fmt.Errorf("container name required for inspect action")
		}
		return dockerInspect(container)
	case "exec":
		if container == "" {
			return "", fmt.Errorf("container name required for exec action")
		}
		command, ok := args["command"].(string)
		if !ok {
			return "", fmt.Errorf("command must be a string for exec action")
		}
		return dockerExec(container, command, checker, confirmed)
	default:
		return "", fmt.Errorf("unknown action: %s", action)
	}
}

func dockerStart(container string) (string, error) {
	cmd := exec.Command("docker", "start", container)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to start %s: %s", container, string(output))
	}
	return fmt.Sprintf("✅ 容器 %s 已启动", container), nil
}

func dockerStop(container string) (string, error) {
	cmd := exec.Command("docker", "stop", container)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to stop %s: %s", container, string(output))
	}
	return fmt.Sprintf("✅ 容器 %s 已停止", container), nil
}

// dockerRestart 重启容器
func dockerRestart(container string) (string, error) {
	cmd := exec.Command("docker", "restart", container)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to restart %s: %s", container, string(output))
	}
	return fmt.Sprintf("✅ 容器 %s 已重启", container), nil
}

// dockerLogs 查看日志
func dockerLogs(container string, lines int) (string, error) {
	cmd := exec.Command("docker", "logs", "--tail", fmt.Sprintf("%d", lines), container)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get logs from %s: %s", container, string(output))
	}
	return string(output), nil
}

// dockerStatus 查看状态
func dockerStatus(container string) (string, error) {
	if container == "" {
		// 查看所有容器
		cmd := exec.Command("docker", "ps", "-a", "--format", "table {{.Names}}\t{{.Status}}\t{{.Ports}}")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("failed to list containers: %s", string(output))
		}
		return string(output), nil
	}
	// 查看指定容器
	cmd := exec.Command("docker", "inspect", "--format", "状态: {{.State.Status}}\n运行时间: {{.State.StartedAt}}", container)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get status of %s: %s", container, string(output))
	}
	return fmt.Sprintf("容器 %s:\n%s", container, string(output)), nil
}

// dockerInspect 查看容器详细配置
func dockerInspect(container string) (string, error) {
	// 获取关键配置信息：环境变量、网络、端口映射
	format := `容器: {{.Name}}
状态: {{.State.Status}}
镜像: {{.Config.Image}}

环境变量:
{{range .Config.Env}}- {{.}}
{{end}}
网络:
{{range $net, $cfg := .NetworkSettings.Networks}}- {{$net}}: {{$cfg.IPAddress}}
{{end}}
端口映射:
{{range $port, $bindings := .NetworkSettings.Ports}}{{if $bindings}}- {{$port}} -> {{range $bindings}}{{.HostIp}}:{{.HostPort}}{{end}}
{{end}}{{end}}
挂载:
{{range .Mounts}}- {{.Source}} -> {{.Destination}}
{{end}}`
	cmd := exec.Command("docker", "inspect", "--format", format, container)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to inspect %s: %s", container, string(output))
	}
	return string(output), nil
}

// dockerExec 在容器内执行命令
func dockerExec(container, command string, checker *SafetyChecker, confirmed bool) (string, error) {
	//安全检查
	if checker != nil {
		needConfirm, err := checker.CheckDockerExec(container, command)
		if err != nil {
			return "", fmt.Errorf("安全检查失败: %w", err)
		}
		if needConfirm && !confirmed {
			return "", &ConfirmRequiredError{
				ToolName: "docker",
				Args: map[string]any{
					"action":    "exec",
					"container": container,
					"command":   command,
				},
				Reason: "安全策略要求确认",
			}
		}
	}
	//命令执行
	cmd := exec.Command("docker", "exec", container, "sh", "-c", command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to exec in %s: %s", container, string(output))
	}
	return string(output), nil
}

// 本地Shell工具
func CreateLocalShellTool() *Tool {
	return &Tool{
		Name:        "local_shell",
		Description: "执行本地命令（仅限白名单）",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"command": map[string]interface{}{
					"type":        "string",
					"description": "要执行的命令",
				},
			},
			"required": []string{"command"},
		},
		Handler:     handleLocalShellTool,
		NeedConfirm: false, // 白名单内自动执行
		Timeout:     30 * time.Second,
		RetryCount:  0,
	}
}

// handleLocalShellTool 本地 Shell 工具处理函数
func handleLocalShellTool(args map[string]interface{}) (string, error) {
	command, ok := args["command"].(string)
	if !ok {
		return "", fmt.Errorf("command must be a string")
	}
	checker, _ := args["_safety_checker"].(*SafetyChecker)
	confirmed, _ := args["_confirmed"].(bool)

	// 安全检查
	if checker != nil {
		needConfirm, err := checker.CheckLocalShell(command)
		if err != nil {
			return "", fmt.Errorf("安全检查失败: %w", err)
		}
		if needConfirm && !confirmed {
			return "", &ConfirmRequiredError{
				ToolName: "local_shell",
				Args: map[string]any{
					"command": command,
				},
				Reason: "命令不在白名单，需确认",
			}
		}
	}

	// 执行命令
	// 判断操作系统
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", command)
	} else {
		cmd = exec.Command("sh", "-c", command)
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to execute command: %s", string(output))
	}

	return string(output), nil
}

// CreateRemoteShellTool 创建远程 Shell 工具
func CreateRemoteShellTool() *Tool {
	return &Tool{
		Name:        "remote_shell",
		Description: "在远程服务器执行命令（需要用户确认）",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"host": map[string]interface{}{
					"type":        "string",
					"description": "主机地址",
				},
				"command": map[string]interface{}{
					"type":        "string",
					"description": "要执行的命令",
				},
			},
			"required": []string{"host", "command"},
		},
		Handler:     handleRemoteShellTool,
		NeedConfirm: true, // 必须用户确认
		Timeout:     60 * time.Second,
		RetryCount:  0,
	}
}

// handleRemoteShellTool 远程 Shell 工具处理函数
func handleRemoteShellTool(args map[string]interface{}) (string, error) {
	host, ok := args["host"].(string)
	if !ok {
		return "", fmt.Errorf("host must be a string")
	}

	command, ok := args["command"].(string)
	if !ok {
		return "", fmt.Errorf("command must be a string")
	}

	// 执行 SSH 命令
	// 注意：这里假设已经配置了 SSH 密钥认证
	cmd := exec.Command("ssh", host, command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to execute remote command on %s: %s", host, string(output))
	}

	return string(output), nil
}

// CreateRecallMemoryTool 创建记忆检索工具
// 让 LLM 在 ReAct 循环中按需检索历史经验和知识
func CreateRecallMemoryTool(memManager *memory.MemoryManager) *Tool {
	return &Tool{
		Name:        "recall_memory",
		Description: "检索历史经验和知识。当需要参考之前类似问题的处理方式、查找相关知识时调用此工具。",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "检索关键词，描述你想查找的问题或场景，如'nginx 502 错误'、'磁盘空间不足'、'kafka 消费延迟'",
				},
			},
			"required": []string{"query"},
		},
		Handler:     createRecallMemoryHandler(memManager),
		NeedConfirm: false,
		Timeout:     10 * time.Second,
		RetryCount:  0,
	}
}

// createRecallMemoryHandler 创建记忆检索处理函数
func createRecallMemoryHandler(memManager *memory.MemoryManager) func(args map[string]interface{}) (string, error) {
	return func(args map[string]interface{}) (string, error) {
		if memManager == nil {
			return "记忆系统未启用", nil
		}

		query, ok := args["query"].(string)
		if !ok || query == "" {
			return "", fmt.Errorf("query must be a non-empty string")
		}

		var sb strings.Builder

		// 检索相关 Episodes
		episodes, err := memManager.SearchEpisodes(query, 3)
		if err == nil && len(episodes) > 0 {
			sb.WriteString("## 相关历史经验\n\n")
			for i, ep := range episodes {
				sb.WriteString(fmt.Sprintf("### 经验 %d\n", i+1))
				sb.WriteString(fmt.Sprintf("- **问题**: %s\n", ep.TriggerSummary))
				sb.WriteString(fmt.Sprintf("- **结果**: %s (%s)\n", ep.OutcomeSummary, ep.Outcome))
				if ep.Target != "" {
					sb.WriteString(fmt.Sprintf("- **目标**: %s\n", ep.Target))
				}
				if len(ep.Steps) > 0 {
					sb.WriteString("- **执行步骤**:\n")
					for j, step := range ep.Steps {
						sb.WriteString(fmt.Sprintf("  %d. %s", j+1, step.ToolName))
						if step.Args != nil {
							if action, ok := step.Args["action"].(string); ok {
								sb.WriteString(fmt.Sprintf(" (%s)", action))
							}
							if cmd, ok := step.Args["command"].(string); ok && len(cmd) < 50 {
								sb.WriteString(fmt.Sprintf(": %s", cmd))
							}
						}
						sb.WriteString("\n")
					}
				}
				sb.WriteString("\n")
			}
		}

		// 检索相关 Knowledge
		knowledge, err := memManager.SearchKnowledge(query, 2)
		if err == nil && len(knowledge) > 0 {
			sb.WriteString("## 相关知识\n\n")
			for _, k := range knowledge {
				sb.WriteString(fmt.Sprintf("### %s\n", k.Topic))
				sb.WriteString(k.Content)
				if len(k.KeyPoints) > 0 {
					sb.WriteString("\n\n**要点**: ")
					sb.WriteString(strings.Join(k.KeyPoints, "、"))
				}
				sb.WriteString("\n\n")
			}
		}

		if sb.Len() == 0 {
			return fmt.Sprintf("未找到与「%s」相关的历史经验或知识。", query), nil
		}

		return sb.String(), nil
	}
}
