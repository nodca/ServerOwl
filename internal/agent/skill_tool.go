package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"serverowl/internal/skills"
)

// CreateSkillTool 创建技能执行工具
func CreateSkillTool(manager *skills.Manager) *Tool {
	return &Tool{
		Name:        "skill",
		Description: "执行预定义的技能。技能是一组预配置的操作步骤，可以完成复杂任务如备份数据库、健康检查、重启服务等。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action": map[string]any{
					"type":        "string",
					"enum":        []string{"execute", "list", "info"},
					"description": "操作类型：execute 执行技能，list 列出可用技能，info 查看技能详情",
				},
				"name": map[string]any{
					"type":        "string",
					"description": "技能名称（execute 和 info 操作需要）",
				},
				"params": map[string]any{
					"type":        "object",
					"description": "技能参数（execute 操作需要）",
				},
				"tag": map[string]any{
					"type":        "string",
					"description": "按标签筛选技能（list 操作可选）",
				},
			},
			"required": []string{"action"},
		},
		Handler:     createSkillHandler(manager),
		NeedConfirm: false,
		Timeout:     5 * time.Minute,
		RetryCount:  0,
	}
}

func createSkillHandler(manager *skills.Manager) func(args map[string]any) (string, error) {
	return func(args map[string]any) (string, error) {
		if manager == nil {
			return "", fmt.Errorf("技能系统未启用")
		}

		action, ok := args["action"].(string)
		if !ok {
			return "", fmt.Errorf("action 参数必须是字符串")
		}

		confirmed, _ := args["_confirmed"].(bool)

		switch action {
		case "list":
			return handleSkillList(manager, args)
		case "info":
			return handleSkillInfo(manager, args)
		case "execute":
			return handleSkillExecute(manager, args, confirmed)
		default:
			return "", fmt.Errorf("未知操作: %s", action)
		}
	}
}

func handleSkillList(manager *skills.Manager, args map[string]any) (string, error) {
	var skillList []*skills.Skill

	if tag, ok := args["tag"].(string); ok && tag != "" {
		skillList = manager.ListSkillsByTag(tag)
	} else {
		skillList = manager.ListSkills()
	}

	if len(skillList) == 0 {
		return "没有找到可用的技能。", nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## 可用技能 (%d 个)\n\n", len(skillList)))

	for _, skill := range skillList {
		sb.WriteString(fmt.Sprintf("### %s\n", skill.Name))
		sb.WriteString(fmt.Sprintf("- **描述**: %s\n", skill.Description))
		if len(skill.Tags) > 0 {
			sb.WriteString(fmt.Sprintf("- **标签**: %s\n", strings.Join(skill.Tags, ", ")))
		}
		if skill.NeedConfirm {
			sb.WriteString("- **需要确认**: 是\n")
		}
		sb.WriteString(fmt.Sprintf("- **步骤数**: %d\n", len(skill.Steps)))
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

func handleSkillInfo(manager *skills.Manager, args map[string]any) (string, error) {
	name, ok := args["name"].(string)
	if !ok || name == "" {
		return "", fmt.Errorf("name 参数是必需的")
	}

	skill, found := manager.GetSkill(name)
	if !found {
		return fmt.Sprintf("技能 %q 不存在。使用 action=list 查看可用技能。", name), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## 技能: %s\n\n", skill.Name))
	sb.WriteString(fmt.Sprintf("**描述**: %s\n", skill.Description))
	sb.WriteString(fmt.Sprintf("**版本**: %s\n", skill.Version))
	sb.WriteString(fmt.Sprintf("**作者**: %s\n", skill.Author))

	if len(skill.Tags) > 0 {
		sb.WriteString(fmt.Sprintf("**标签**: %s\n", strings.Join(skill.Tags, ", ")))
	}

	if skill.NeedConfirm {
		sb.WriteString("**需要确认**: 是\n")
	}

	if skill.Timeout > 0 {
		sb.WriteString(fmt.Sprintf("**超时**: %s\n", skill.Timeout))
	}

	// 参数列表
	if len(skill.Parameters) > 0 {
		sb.WriteString("\n### 参数\n\n")
		for _, param := range skill.Parameters {
			required := ""
			if param.Required {
				required = " (必需)"
			}
			sb.WriteString(fmt.Sprintf("- **%s**%s: %s\n", param.Name, required, param.Description))
			if param.Default != nil {
				sb.WriteString(fmt.Sprintf("  - 默认值: %v\n", param.Default))
			}
		}
	}

	// 步骤列表
	if len(skill.Steps) > 0 {
		sb.WriteString("\n### 执行步骤\n\n")
		for i, step := range skill.Steps {
			sb.WriteString(fmt.Sprintf("%d. **%s** (工具: %s", i+1, step.Name, step.Tool))
			if step.Action != "" {
				sb.WriteString(fmt.Sprintf(", 操作: %s", step.Action))
			}
			sb.WriteString(")\n")
			if step.Condition != "" {
				sb.WriteString(fmt.Sprintf("   - 条件: %s\n", step.Condition))
			}
			if step.OnError != "" {
				sb.WriteString(fmt.Sprintf("   - 错误处理: %s\n", step.OnError))
			}
		}
	}

	return sb.String(), nil
}

func handleSkillExecute(manager *skills.Manager, args map[string]any, confirmed bool) (string, error) {
	name, ok := args["name"].(string)
	if !ok || name == "" {
		return "", fmt.Errorf("name 参数是必需的")
	}

	skill, found := manager.GetSkill(name)
	if !found {
		return "", fmt.Errorf("技能 %q 不存在", name)
	}

	// 检查是否需要确认
	if skill.NeedConfirm && !confirmed {
		params, _ := args["params"].(map[string]any)
		return "", &ConfirmRequiredError{
			ToolName: "skill",
			Args: map[string]any{
				"action": "execute",
				"name":   name,
				"params": params,
			},
			Reason: fmt.Sprintf("技能 %q 需要确认后执行", name),
		}
	}

	// 提取参数
	params := make(map[string]any)
	if p, ok := args["params"].(map[string]any); ok {
		params = p
	}

	// 验证参数
	if err := manager.ValidateParams(name, params); err != nil {
		return "", fmt.Errorf("参数验证失败: %w", err)
	}

	// 执行技能
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	result, err := manager.Execute(ctx, name, params)
	if err != nil {
		return "", fmt.Errorf("技能执行失败: %w", err)
	}

	// 格式化结果
	return formatSkillResult(result), nil
}

func formatSkillResult(result *skills.SkillResult) string {
	var sb strings.Builder

	if result.Success {
		sb.WriteString(fmt.Sprintf("✅ 技能 %s 执行成功\n\n", result.SkillName))
	} else {
		sb.WriteString(fmt.Sprintf("❌ 技能 %s 执行失败\n\n", result.SkillName))
		if result.Error != "" {
			sb.WriteString(fmt.Sprintf("**错误**: %s\n\n", result.Error))
		}
	}

	sb.WriteString(fmt.Sprintf("**耗时**: %s\n\n", result.EndTime.Sub(result.StartTime).Round(time.Millisecond)))

	if len(result.Steps) > 0 {
		sb.WriteString("### 步骤详情\n\n")
		for i, step := range result.Steps {
			status := "✅"
			if !step.Success {
				status = "❌"
			}
			sb.WriteString(fmt.Sprintf("%d. %s **%s** (%s, %s)\n",
				i+1, status, step.StepName, step.Tool, step.Duration.Round(time.Millisecond)))

			if step.Output != "" {
				// 截断过长的输出
				output := step.Output
				if len(output) > 500 {
					output = output[:500] + "...(输出已截断)"
				}
				sb.WriteString(fmt.Sprintf("   ```\n   %s\n   ```\n", strings.TrimSpace(output)))
			}

			if step.Error != "" {
				sb.WriteString(fmt.Sprintf("   **错误**: %s\n", step.Error))
			}
		}
	}

	return sb.String()
}

// CreateListSkillsTool 创建列出技能的简化工具
func CreateListSkillsTool(manager *skills.Manager) *Tool {
	return &Tool{
		Name:        "list_skills",
		Description: "列出所有可用的技能",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		Handler: func(args map[string]any) (string, error) {
			if manager == nil {
				return "技能系统未启用", nil
			}

			infos := manager.ListSkillInfos()
			if len(infos) == 0 {
				return "没有可用的技能", nil
			}

			data, _ := json.MarshalIndent(infos, "", "  ")
			return string(data), nil
		},
		NeedConfirm: false,
		Timeout:     10 * time.Second,
	}
}
