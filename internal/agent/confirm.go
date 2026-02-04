package agent

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ConfirmRequiredError 表示“该工具调用需要用户确认后才能执行”
type ConfirmRequiredError struct {
	ToolName string
	Args     map[string]any
	Reason   string
}

func (e *ConfirmRequiredError) Error() string {
	if e.Reason != "" {
		return fmt.Sprintf("confirm required: %s (%s)", e.ToolName, e.Reason)
	}
	return fmt.Sprintf("confirm required: %s", e.ToolName)
}

func isConfirmRequired(err error) (*ConfirmRequiredError, bool) {
	e, ok := err.(*ConfirmRequiredError)
	return e, ok
}

// PendingActionReplyError 用于让执行器“直接把确认提示返回给用户”，避免被 LLM 改写导致 id 不一致。
type PendingActionReplyError struct {
	ID    string
	Reply string
}

func (e *PendingActionReplyError) Error() string {
	return "pending action created: " + e.ID
}

// buildConfirmPrompt 生成给用户看的确认提示
func buildConfirmPrompt(toolName string, args map[string]any, reason string) string {
	argsJSON, _ := json.Marshal(args)
	var summary string
	switch toolName {
	case "docker":
		action, _ := args["action"].(string)
		container, _ := args["container"].(string)
		command, _ := args["command"].(string)
		if command != "" && len([]rune(command)) > 200 {
			rs := []rune(command)
			command = string(rs[:200]) + "..."
		}
		if command != "" {
			summary = fmt.Sprintf("将对容器 %s 执行 docker.%s（命令：%s）", container, action, command)
		} else {
			summary = fmt.Sprintf("将对容器 %s 执行 docker.%s", container, action)
		}
	case "local_shell":
		command, _ := args["command"].(string)
		if command != "" && len([]rune(command)) > 200 {
			rs := []rune(command)
			command = string(rs[:200]) + "..."
		}
		summary = fmt.Sprintf("将执行本机命令：%s", command)
	case "remote_shell":
		host, _ := args["host"].(string)
		command, _ := args["command"].(string)
		if command != "" && len([]rune(command)) > 200 {
			rs := []rune(command)
			command = string(rs[:200]) + "..."
		}
		summary = fmt.Sprintf("将对远程主机 %s 执行命令：%s", host, command)
	}

	var b strings.Builder
	if reason != "" {
		b.WriteString("该操作需要确认（" + reason + "）。\n")
	} else {
		b.WriteString("该操作需要确认。\n")
	}
	if summary != "" {
		b.WriteString(summary + "\n")
	}
	b.WriteString("工具：" + toolName + "\n")
	b.WriteString("参数：" + string(argsJSON))
	return b.String()
}

func defaultConfirmTTL() time.Duration {
	return 10 * time.Minute
}
