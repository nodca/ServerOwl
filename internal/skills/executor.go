package skills

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"text/template"
	"time"
)

// Executor 技能执行引擎
type Executor struct {
	registry    *SkillRegistry
	variables   map[string]any
	stepOutputs map[string]string
}

// NewExecutor 创建新的执行器
func NewExecutor(registry *SkillRegistry) *Executor {
	return &Executor{
		registry:    registry,
		variables:   make(map[string]any),
		stepOutputs: make(map[string]string),
	}
}

// Execute 执行技能
func (e *Executor) Execute(ctx context.Context, skillName string, params map[string]any) (*SkillResult, error) {
	skill, ok := e.registry.Get(skillName)
	if !ok {
		return nil, fmt.Errorf("skill not found: %s", skillName)
	}

	return e.ExecuteSkill(ctx, skill, params)
}

// ExecuteSkill 执行指定的技能
func (e *Executor) ExecuteSkill(ctx context.Context, skill *Skill, params map[string]any) (*SkillResult, error) {
	result := &SkillResult{
		SkillName: skill.Name,
		StartTime: time.Now(),
		Steps:     make([]StepResult, 0, len(skill.Steps)),
	}

	// 设置超时
	if skill.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, skill.Timeout)
		defer cancel()
	}

	// 初始化变量
	e.variables = make(map[string]any)
	e.stepOutputs = make(map[string]string)

	// 处理参数默认值
	if err := e.initParameters(skill, params); err != nil {
		result.Success = false
		result.Error = err.Error()
		result.EndTime = time.Now()
		return result, err
	}

	// 执行每个步骤
	for i, step := range skill.Steps {
		select {
		case <-ctx.Done():
			result.Success = false
			result.Error = "execution timeout or cancelled"
			result.EndTime = time.Now()
			return result, ctx.Err()
		default:
		}

		stepResult := e.executeStep(ctx, &step, i)
		result.Steps = append(result.Steps, stepResult)

		// 保存步骤输出供后续步骤使用
		if step.Name != "" {
			e.stepOutputs[step.Name] = stepResult.Output
		}

		if !stepResult.Success {
			switch step.OnError {
			case "continue":
				continue
			case "retry":
				// 重试逻辑
				retried := false
				for r := 0; r < step.Retries; r++ {
					time.Sleep(time.Second * time.Duration(r+1))
					retryResult := e.executeStep(ctx, &step, i)
					if retryResult.Success {
						result.Steps[len(result.Steps)-1] = retryResult
						if step.Name != "" {
							e.stepOutputs[step.Name] = retryResult.Output
						}
						retried = true
						break
					}
				}
				if !retried {
					result.Success = false
					result.Error = fmt.Sprintf("step %d failed after %d retries: %s", i+1, step.Retries, stepResult.Error)
					result.EndTime = time.Now()
					return result, fmt.Errorf(result.Error)
				}
			default: // stop
				result.Success = false
				result.Error = fmt.Sprintf("step %d failed: %s", i+1, stepResult.Error)
				result.EndTime = time.Now()
				return result, fmt.Errorf(result.Error)
			}
		}
	}

	result.Success = true
	result.EndTime = time.Now()
	return result, nil
}

func (e *Executor) initParameters(skill *Skill, params map[string]any) error {
	for _, param := range skill.Parameters {
		value, provided := params[param.Name]
		if !provided {
			if param.Required && param.Default == nil {
				return fmt.Errorf("required parameter %q not provided", param.Name)
			}
			value = param.Default
		}

		// 类型转换
		converted, err := e.convertType(value, param.Type)
		if err != nil {
			return fmt.Errorf("parameter %q: %w", param.Name, err)
		}

		e.variables[param.Name] = converted
	}

	// 添加用户提供的额外参数
	for k, v := range params {
		if _, exists := e.variables[k]; !exists {
			e.variables[k] = v
		}
	}

	return nil
}

func (e *Executor) convertType(value any, targetType string) (any, error) {
	if value == nil {
		return nil, nil
	}

	switch targetType {
	case "string":
		return fmt.Sprintf("%v", value), nil
	case "int":
		switch v := value.(type) {
		case int:
			return v, nil
		case int64:
			return int(v), nil
		case float64:
			return int(v), nil
		case string:
			return strconv.Atoi(v)
		default:
			return nil, fmt.Errorf("cannot convert %T to int", value)
		}
	case "bool":
		switch v := value.(type) {
		case bool:
			return v, nil
		case string:
			return strconv.ParseBool(v)
		default:
			return nil, fmt.Errorf("cannot convert %T to bool", value)
		}
	default:
		return value, nil
	}
}

func (e *Executor) executeStep(ctx context.Context, step *SkillStep, index int) StepResult {
	startTime := time.Now()
	result := StepResult{
		StepName: step.Name,
		Tool:     step.Tool,
	}

	// 检查条件
	if step.Condition != "" {
		shouldRun, err := e.evaluateCondition(step.Condition)
		if err != nil {
			result.Success = false
			result.Error = fmt.Sprintf("condition evaluation failed: %v", err)
			result.Duration = time.Since(startTime)
			return result
		}
		if !shouldRun {
			result.Success = true
			result.Output = "skipped: condition not met"
			result.Duration = time.Since(startTime)
			return result
		}
	}

	// 处理模板变量
	args, err := e.processArgs(step.Args)
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("process args failed: %v", err)
		result.Duration = time.Since(startTime)
		return result
	}

	// 根据工具类型执行
	switch step.Tool {
	case "local_shell":
		result = e.executeLocalShell(ctx, args)
	case "remote_shell":
		result = e.executeRemoteShell(ctx, args)
	case "docker":
		result = e.executeDocker(ctx, step.Action, args)
	case "skill":
		result = e.executeNestedSkill(ctx, args)
	default:
		result.Success = false
		result.Error = fmt.Sprintf("unknown tool: %s", step.Tool)
	}

	result.StepName = step.Name
	result.Tool = step.Tool
	result.Duration = time.Since(startTime)
	return result
}

func (e *Executor) processArgs(args map[string]any) (map[string]any, error) {
	processed := make(map[string]any)

	for key, value := range args {
		switch v := value.(type) {
		case string:
			rendered, err := e.renderTemplate(v)
			if err != nil {
				return nil, fmt.Errorf("render %q: %w", key, err)
			}
			processed[key] = rendered
		default:
			processed[key] = value
		}
	}

	return processed, nil
}

func (e *Executor) renderTemplate(text string) (string, error) {
	// 创建模板数据
	data := make(map[string]any)
	for k, v := range e.variables {
		data[k] = v
	}
	data["steps"] = e.stepOutputs

	tmpl, err := template.New("arg").Parse(text)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func (e *Executor) evaluateCondition(condition string) (bool, error) {
	// 简单的条件评估
	// 支持: {{.var}} == "value", {{.var}} != "", {{.var}}
	rendered, err := e.renderTemplate(condition)
	if err != nil {
		return false, err
	}

	rendered = strings.TrimSpace(rendered)

	// 检查比较操作
	if strings.Contains(rendered, "==") {
		parts := strings.SplitN(rendered, "==", 2)
		if len(parts) == 2 {
			left := strings.TrimSpace(parts[0])
			right := strings.TrimSpace(strings.Trim(parts[1], "\"'"))
			return left == right, nil
		}
	}

	if strings.Contains(rendered, "!=") {
		parts := strings.SplitN(rendered, "!=", 2)
		if len(parts) == 2 {
			left := strings.TrimSpace(parts[0])
			right := strings.TrimSpace(strings.Trim(parts[1], "\"'"))
			return left != right, nil
		}
	}

	// 布尔值检查
	if rendered == "true" || rendered == "1" {
		return true, nil
	}
	if rendered == "false" || rendered == "0" || rendered == "" {
		return false, nil
	}

	// 非空字符串视为 true
	return rendered != "", nil
}

func (e *Executor) executeLocalShell(ctx context.Context, args map[string]any) StepResult {
	result := StepResult{}

	command, ok := args["command"].(string)
	if !ok {
		result.Success = false
		result.Error = "command argument is required"
		return result
	}

	// 获取工作目录
	workDir, _ := args["workdir"].(string)

	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	if workDir != "" {
		cmd.Dir = workDir
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result.Output = stdout.String()

	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("%v: %s", err, stderr.String())
		return result
	}

	result.Success = true
	return result
}

func (e *Executor) executeRemoteShell(ctx context.Context, args map[string]any) StepResult {
	result := StepResult{}

	host, ok := args["host"].(string)
	if !ok {
		result.Success = false
		result.Error = "host argument is required"
		return result
	}

	command, ok := args["command"].(string)
	if !ok {
		result.Success = false
		result.Error = "command argument is required"
		return result
	}

	// 构建 SSH 命令
	sshArgs := []string{host}

	// 添加可选参数
	if user, ok := args["user"].(string); ok {
		sshArgs = []string{"-l", user, host}
	}

	if port, ok := args["port"].(int); ok {
		sshArgs = append([]string{"-p", strconv.Itoa(port)}, sshArgs...)
	}

	if key, ok := args["key"].(string); ok {
		sshArgs = append([]string{"-i", key}, sshArgs...)
	}

	sshArgs = append(sshArgs, command)

	cmd := exec.CommandContext(ctx, "ssh", sshArgs...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result.Output = stdout.String()

	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("%v: %s", err, stderr.String())
		return result
	}

	result.Success = true
	return result
}

func (e *Executor) executeDocker(ctx context.Context, action string, args map[string]any) StepResult {
	result := StepResult{}

	var cmdArgs []string

	switch action {
	case "exec":
		container, ok := args["container"].(string)
		if !ok {
			result.Success = false
			result.Error = "container argument is required"
			return result
		}
		command, ok := args["command"].(string)
		if !ok {
			result.Success = false
			result.Error = "command argument is required"
			return result
		}
		cmdArgs = []string{"exec", container, "sh", "-c", command}

	case "run":
		image, ok := args["image"].(string)
		if !ok {
			result.Success = false
			result.Error = "image argument is required"
			return result
		}
		cmdArgs = []string{"run", "--rm"}

		if env, ok := args["env"].(map[string]any); ok {
			for k, v := range env {
				cmdArgs = append(cmdArgs, "-e", fmt.Sprintf("%s=%v", k, v))
			}
		}

		if volumes, ok := args["volumes"].([]any); ok {
			for _, v := range volumes {
				cmdArgs = append(cmdArgs, "-v", fmt.Sprintf("%v", v))
			}
		}

		cmdArgs = append(cmdArgs, image)

		if command, ok := args["command"].(string); ok {
			cmdArgs = append(cmdArgs, "sh", "-c", command)
		}

	case "restart":
		container, ok := args["container"].(string)
		if !ok {
			result.Success = false
			result.Error = "container argument is required"
			return result
		}
		cmdArgs = []string{"restart", container}

	case "stop":
		container, ok := args["container"].(string)
		if !ok {
			result.Success = false
			result.Error = "container argument is required"
			return result
		}
		cmdArgs = []string{"stop", container}

	case "start":
		container, ok := args["container"].(string)
		if !ok {
			result.Success = false
			result.Error = "container argument is required"
			return result
		}
		cmdArgs = []string{"start", container}

	case "logs":
		container, ok := args["container"].(string)
		if !ok {
			result.Success = false
			result.Error = "container argument is required"
			return result
		}
		cmdArgs = []string{"logs"}
		if tail, ok := args["tail"].(int); ok {
			cmdArgs = append(cmdArgs, "--tail", strconv.Itoa(tail))
		}
		cmdArgs = append(cmdArgs, container)

	case "ps":
		cmdArgs = []string{"ps"}
		if all, ok := args["all"].(bool); ok && all {
			cmdArgs = append(cmdArgs, "-a")
		}
		if filter, ok := args["filter"].(string); ok {
			cmdArgs = append(cmdArgs, "-f", filter)
		}

	case "inspect":
		target, ok := args["target"].(string)
		if !ok {
			result.Success = false
			result.Error = "target argument is required"
			return result
		}
		cmdArgs = []string{"inspect", target}
		if format, ok := args["format"].(string); ok {
			cmdArgs = append(cmdArgs, "-f", format)
		}

	default:
		result.Success = false
		result.Error = fmt.Sprintf("unknown docker action: %s", action)
		return result
	}

	cmd := exec.CommandContext(ctx, "docker", cmdArgs...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result.Output = stdout.String()

	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("%v: %s", err, stderr.String())
		return result
	}

	result.Success = true
	return result
}

func (e *Executor) executeNestedSkill(ctx context.Context, args map[string]any) StepResult {
	result := StepResult{}

	skillName, ok := args["name"].(string)
	if !ok {
		result.Success = false
		result.Error = "skill name argument is required"
		return result
	}

	// 提取嵌套技能的参数
	skillParams := make(map[string]any)
	if params, ok := args["params"].(map[string]any); ok {
		skillParams = params
	}

	// 执行嵌套技能
	skillResult, err := e.Execute(ctx, skillName, skillParams)
	if err != nil {
		result.Success = false
		result.Error = err.Error()
		return result
	}

	result.Success = skillResult.Success
	if !skillResult.Success {
		result.Error = skillResult.Error
	}

	// 汇总输出
	var outputs []string
	for _, step := range skillResult.Steps {
		if step.Output != "" {
			outputs = append(outputs, step.Output)
		}
	}
	result.Output = strings.Join(outputs, "\n")

	return result
}

// SetVariable 设置执行变量
func (e *Executor) SetVariable(name string, value any) {
	e.variables[name] = value
}

// GetVariable 获取执行变量
func (e *Executor) GetVariable(name string) (any, bool) {
	v, ok := e.variables[name]
	return v, ok
}

// GetStepOutput 获取步骤输出
func (e *Executor) GetStepOutput(stepName string) (string, bool) {
	output, ok := e.stepOutputs[stepName]
	return output, ok
}

// ValidateParams 验证参数是否满足技能要求
func (e *Executor) ValidateParams(skill *Skill, params map[string]any) error {
	for _, param := range skill.Parameters {
		_, provided := params[param.Name]
		if param.Required && !provided && param.Default == nil {
			return fmt.Errorf("required parameter %q not provided", param.Name)
		}
	}
	return nil
}

// ExtractVariables 从文本中提取变量引用
func ExtractVariables(text string) []string {
	re := regexp.MustCompile(`\{\{\s*\.(\w+)\s*\}\}`)
	matches := re.FindAllStringSubmatch(text, -1)

	var vars []string
	seen := make(map[string]bool)
	for _, match := range matches {
		if len(match) > 1 && !seen[match[1]] {
			vars = append(vars, match[1])
			seen[match[1]] = true
		}
	}
	return vars
}
