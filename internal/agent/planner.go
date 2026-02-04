package agent

import (
	"encoding/json"
	"fmt"
	"serverowl/internal/llm"
	"strings"
)

// 执行计划
type ExecutionPlan struct {
	TaskSummary string     `json:"task_summary"` //任务摘要
	Steps       []PlanStep `json:"steps"`        // 执行步骤
	IsComplex   bool       `json:"is_complex"`   // 是否复杂任务
}
type PlanStep struct {
	StepNum     int    `json:"step_num"`    // 步骤编号
	Description string `json:"description"` // 步骤描述
	Tool        string `json:"tool"`        // 使用的工具
	Purpose     string `json:"purpose"`     // 这一步的目的
}

// Planner 任务规划器
type Planner struct {
	llmClient llm.Client
}

func NewPlanner(llmClient llm.Client) *Planner {
	return &Planner{
		llmClient: llmClient,
	}
}

// 判断是否需要规则
func (p *Planner) NeedPlanning(userInput string) bool {
	complexKeywords := []string{
		"诊断", "排查", "分析", "为什么", "怎么回事",
		"检查一下", "全面", "所有", "频繁", "一直",
		"troubleshoot", "diagnose", "analyze", "investigate",
	}
	input := strings.ToLower(userInput)
	for _, kw := range complexKeywords {
		if strings.Contains(input, kw) {
			return true
		}
	}
	// 简单任务：直接执行
	simpleKeywords := []string{
		"状态", "日志", "重启", "看看", "查看",
		"status", "logs", "restart",
	}
	for _, kw := range simpleKeywords {
		if strings.Contains(input, kw) && len([]rune(userInput)) < 20 {
			return false
		}
	}

	// 默认
	return len([]rune(userInput)) > 10
}

// 创建执行计划
func (p *Planner) CreatePlan(userInput string) (*ExecutionPlan, error) {
	prompt := p.buildPlanningPrompt(userInput)

	resp, err := p.llmClient.Chat(prompt)
	if err != nil {
		return nil, fmt.Errorf("planning failed: %w", err)
	}

	// 解析 JSON 计划
	plan, err := p.parsePlan(resp)
	if err != nil {
		// 解析失败，返回默认计划
		return &ExecutionPlan{
			TaskSummary: userInput,
			IsComplex:   false,
			Steps:       nil,
		}, nil
	}

	return plan, nil
}

func (p *Planner) buildPlanningPrompt(userInput string) string {
	return fmt.Sprintf(`你是一个运维任务规划器。用户提出了一个运维需求，请分析并生成执行计划。

用户需求：%s

可用工具：
1. docker - 容器操作（status/logs/restart/exec）
2. local_shell - 本地命令执行
3. remote_shell - 远程服务器命令

请生成一个 JSON 格式的执行计划：

{
"task_summary": "任务的一句话摘要",
"is_complex": true/false,
"steps": [
  {
	"step_num": 1,
	"description": "具体要做什么",
	"tool": "使用哪个工具",
	"purpose": "这一步的目的是什么"
  }
]
}

规则：
- 如果是简单任务（如"看状态"、"重启xxx"），设置 is_complex=false，steps 可以为空
- 如果是复杂任务（如"诊断问题"、"排查原因"），设置 is_complex=true，列出 3-6 个步骤
- 步骤要有逻辑顺序：先收集信息，再分析，最后行动
- 每个步骤要明确使用哪个工具

只输出 JSON，不要其他内容。`, userInput)
}

func (p *Planner) parsePlan(response string) (*ExecutionPlan, error) {
	response = strings.TrimSpace(response)

	// 处理可能的 markdown 代码块
	if strings.HasPrefix(response, "```") {
		lines := strings.Split(response, "\n")
		var jsonLines []string
		inBlock := false
		for _, line := range lines {
			if strings.HasPrefix(line, "```") {
				inBlock = !inBlock
				continue
			}
			if inBlock {
				jsonLines = append(jsonLines, line)
			}
		}
		response = strings.Join(jsonLines, "\n")
	}

	var plan ExecutionPlan
	if err := json.Unmarshal([]byte(response), &plan); err != nil {
		return nil, err
	}

	return &plan, nil
}

// FormatPlanForUser 格式化计划供用户查看（可选，用于调试）
func (p *Planner) FormatPlanForUser(plan *ExecutionPlan) string {
	if plan == nil || !plan.IsComplex || len(plan.Steps) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📋 **任务规划**：%s\n\n", plan.TaskSummary))

	for _, step := range plan.Steps {
		sb.WriteString(fmt.Sprintf("%d. %s\n   工具: %s | 目的: %s\n",
			step.StepNum, step.Description, step.Tool, step.Purpose))
	}

	return sb.String()
}
