package agent

import (
	"context"
	crand "crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"serverowl/internal/llm"
	"serverowl/internal/memory"
	"strconv"
	"strings"
	"time"
)

// ErrorType 错误分类
type ErrorType int

const (
	ErrorRetryable    ErrorType = iota // 网络超时、临时故障
	ErrorNonRetryable                  // 权限不足、资源不存在
)

// classifyError 对错误进行分类，决定是否可重试
func classifyError(err error) ErrorType {
	if err == nil {
		return ErrorRetryable
	}
	msg := strings.ToLower(err.Error())
	// 不可重试的错误
	if strings.Contains(msg, "permission denied") ||
		strings.Contains(msg, "not found") ||
		strings.Contains(msg, "no such") ||
		strings.Contains(msg, "does not exist") ||
		strings.Contains(msg, "invalid") ||
		strings.Contains(msg, "forbidden") {
		return ErrorNonRetryable
	}
	// 默认可重试
	return ErrorRetryable
}

type AgentExecutor struct {
	llmClient          llm.Client
	toolRegistry       *ToolRegistry
	safetyChecker      *SafetyChecker
	actionLogger       *ActionLogger
	memoryManager      *memory.MemoryManager
	planner            *Planner
	maxIterations      int
	maxConsecutiveFail int // 最大连续失败次数
}

func NewAgentExecutor(llmClient llm.Client, toolRegistry *ToolRegistry, safetyChecker *SafetyChecker, logger *ActionLogger, memManager *memory.MemoryManager) *AgentExecutor {
	return &AgentExecutor{
		llmClient:          llmClient,
		toolRegistry:       toolRegistry,
		safetyChecker:      safetyChecker,
		actionLogger:       logger,
		memoryManager:      memManager,
		planner:            NewPlanner(llmClient),
		maxIterations:      10, // 默认最多 10 轮
		maxConsecutiveFail: 3,  // 最多连续失败 3 次
	}
}

// buildSystemPrompt 构造系统提示词
func (a *AgentExecutor) buildSystemPrompt() string {
	return `# ServerOwl 运维助手

你是 ServerOwl，运行在服务器本地的运维 Agent。

## 核心原则

1. **简洁回复**：直接给答案，不要长篇大论。用户问什么答什么，不要过度解释。
2. **高效执行**：一次命令能解决的不要分多次。优先用能获取完整信息的命令。
3. **安全第一**：危险操作需确认，黑名单命令拒绝执行。

## 回复风格

- **简短直接**：用户问 PostgreSQL 密码，就回复密码，不需要解释什么是 DSN
- **只给关键信息**：不要列出所有可能的连接方式，除非用户问
- **避免模板化**：不要每次都用"基于收集的信息..."开头

**好的回复示例**：
用户：postgres 容器的用户名密码是什么？
回复：
- 用户名：days
- 密码：days
- 数据库：days
- 端口：5432

**差的回复示例**：
（长篇大论解释 DSN 格式、各种连接方式、注意事项等）

## 可用工具

### get_environment - 环境信息（优先使用）
查询已扫描的服务器环境信息，包括容器配置、数据库密码、代理配置等。
- 参数：category（all/host/containers/databases/proxies）或 name（具体名称）
- **优先使用此工具**，避免重复执行命令

### docker - 容器管理
| 操作 | 说明 |
|------|------|
| status | 查看容器状态（可不指定 container 查看所有） |
| logs | 查看日志（默认50行） |
| inspect | 查看容器详细配置（环境变量、网络、挂载等）|
| start/stop/restart | 启停容器（需确认） |
| exec | 在容器内执行命令 |

### local_shell - 本地命令
直接在服务器执行命令：df -h、free -m、cat、ls、docker ps 等

### remote_shell - 远程命令
管理其他服务器（需确认）

### recall_memory - 记忆检索
检索历史操作经验和知识。使用场景：
- 遇到类似问题时，查找之前的处理方式
- 用户问"上次怎么处理的"、"之前遇到过吗"
- 参数：query（关键词，如"nginx 502"、"磁盘满"）

## 工具选择指南

| 用户需求 | 推荐做法 |
|---------|---------|
| 查容器密码/配置 | **get_environment** 优先，没有再用 docker inspect |
| 查数据库连接信息 | **get_environment(category=databases)** |
| 查代理/站点配置 | **get_environment(category=proxies)** |
| 查容器状态 | docker status |
| 查系统资源 | local_shell: df -h / free -m |
| 查容器日志 | docker logs |
| 启停容器 | docker start/stop/restart |
| 查历史经验 | recall_memory |

## 确认机制

- 需要确认的操作：直接调用工具，系统自动生成确认提示
- **不要自己写"请回复 yes/no"**，这会导致确认流程失效

## 禁止操作

rm -rf、DROP DATABASE、TRUNCATE、dd if=、mkfs、chmod 777、FLUSHALL`
}

func (a *AgentExecutor) Execute(sessionID, userInput, requestID string) (string, error) {
	consecutiveFailures := 0

	// 跟踪执行的步骤，用于生成 Episode
	var executedSteps []memory.TaskStep

	//判断是否需要规划
	var planContext string
	if a.planner != nil && a.planner.NeedPlanning(userInput) {
		plan, err := a.planner.CreatePlan(userInput)
		if err == nil && plan != nil && plan.IsComplex && len(plan.Steps) > 0 {
			// 将计划注入到上下文中，指导后续执行
			planContext = a.buildPlanContext(plan)
		}
	}

	// 加载同一会话的历史对话，作为真正的多轮对话注入
	var historyMessages []llm.Message
	log.Printf("[DEBUG] Loading history: memoryManager=%v, sessionID=%q", a.memoryManager != nil, sessionID)
	if a.memoryManager != nil && sessionID != "" {
		// 只加载短期记忆（最近对话），长期记忆通过 recall_memory 工具按需检索
		ctx, err := a.memoryManager.BuildContext(sessionID, "")
		log.Printf("[DEBUG] BuildContext result: err=%v, ctx=%v, recentMessages=%d", err, ctx != nil, func() int { if ctx != nil { return len(ctx.RecentMessages) }; return 0 }())
		if err == nil && ctx != nil && len(ctx.RecentMessages) > 0 {
			// 过滤消息：去掉当前输入和所有确认/取消指令
			for _, msg := range ctx.RecentMessages {
				// 跳过当前用户输入（main.go 已经保存了）
				if msg.Role == "user" && msg.Content == userInput {
					continue
				}
				// 跳过所有确认/取消指令
				if msg.Role == "user" && isConfirmCancelOnly(msg.Content) {
					continue
				}
				// 压缩过长的历史消息
				content := msg.Content
				if msg.Role == "assistant" {
					content = compressForHistory(content)
				}
				historyMessages = append(historyMessages, llm.Message{
					Role:    msg.Role,
					Content: content,
				})
			}
		}
	}

	// 构造初始消息
	messages := []llm.Message{
		{
			Role:    "system",
			Content: a.buildSystemPrompt(),
		},
	}
	if planContext != "" {
		messages = append(messages, llm.Message{
			Role:    "system",
			Content: planContext,
		})
	}
	// 注入历史对话（真正的多轮格式）
	if len(historyMessages) > 0 {
		messages = append(messages, historyMessages...)
	}
	// 当前用户输入
	messages = append(messages, llm.Message{Role: "user", Content: userInput})

	// ReAct循环
	for i := 0; i < a.maxIterations; i++ {
		//调用LLM
		llmStart := time.Now()
		resp, err := a.llmClient.ChatWithTools(messages, a.toolRegistry.ToLLMTools())
		llmMs := time.Since(llmStart).Milliseconds()
		if err != nil {
			return "", fmt.Errorf("LLM call failed: %w", err)
		}

		choice := resp.Choices[0]
		//判断是否结束
		if choice.FinishReason == "stop" {
			// 如果有执行过工具，保存为 Episode
			if len(executedSteps) > 0 {
				outcome := "success"
				// 检查是否有失败的步骤
				for _, step := range executedSteps {
					if step.Status == "failed" {
						outcome = "partial"
						break
					}
				}
				a.saveEpisode(sessionID, sessionID, userInput, executedSteps, outcome, choice.Message.Content)
			}
			return choice.Message.Content, nil
		}

		if choice.FinishReason == "tool_calls" {
			//  将 LLM 的 tool_calls 消息添加到历史
			messages = append(messages, llm.Message{
				Role:      "assistant",
				ToolCalls: choice.Message.ToolCalls,
			})
			//执行每个工具的调用
			for _, toolCall := range choice.Message.ToolCalls {
				toolStart := time.Now()
				result, err := a.executeTool(sessionID, sessionID, requestID, userInput, toolCall, &consecutiveFailures)

				// 确认提示：直接返回，避免 LLM 二次改写导致 id 不匹配
				if perr, ok := err.(*PendingActionReplyError); ok {
					_ = llmMs
					return perr.Reply, nil
				}

				// 记录执行步骤
				stepStatus := "completed"
				if err != nil {
					stepStatus = "failed"
					result = fmt.Sprintf("❌ 工具执行失败: %s", err.Error())
				}

				// 解析工具参数用于记录
				var toolArgs map[string]any
				_ = json.Unmarshal([]byte(toolCall.Function.Arguments), &toolArgs)
				// 移除内部字段
				delete(toolArgs, "_safety_checker")
				delete(toolArgs, "_confirmed")

				completedAt := time.Now()
				executedSteps = append(executedSteps, memory.TaskStep{
					Description: toolCall.Function.Name,
					ToolName:    toolCall.Function.Name,
					Args:        toolArgs,
					Result:      truncateToolResult(result),
					Status:      stepStatus,
					StartedAt:   &toolStart,
					CompletedAt: &completedAt,
				})

				// 多次失败直接终止，避免卡到最大迭代次数
				if err != nil && consecutiveFailures >= a.maxConsecutiveFail {
					// 保存失败的 Episode
					a.saveEpisode(sessionID, sessionID, userInput, executedSteps, "failed", "多次尝试失败")
					return "❌ 多次尝试仍失败，我先停止以避免循环。\n\n最后错误：" + err.Error() + "\n\n建议：\n- 如果你是在做「关闭/调整配置」等变更，请明确目标（容器名/服务名/配置位置）\n- 如果是执行写操作，我会先发起确认，再执行。", nil
				}
				_ = llmMs // reserved: could be persisted separately if needed

				// 截断过长的工具输出，减少 Token 用量
				result = truncateToolResult(result)

				// 将工具结果添加到消息历史
				messages = append(messages, llm.Message{
					Role:       "tool",
					ToolCallID: toolCall.ID,
					Content:    result,
				})
			}
			//下一轮循环
			continue
		}
		// 未知的 finish_reason
		return "", fmt.Errorf("unknown finish_reason: %s", choice.FinishReason)
	}
	return "", fmt.Errorf("达到最大迭代次数 %d", a.maxIterations)
}

func (a *AgentExecutor) executeTool(sessionID, userID, requestID, userInput string, toolCall llm.ToolCall, consecutiveFailures *int) (string, error) {
	//获取工具
	tool, err := a.toolRegistry.Get(toolCall.Function.Name)
	if err != nil {
		return "", err
	}
	//解析参数 JSON string -> map
	var args map[string]any
	if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
		return "", fmt.Errorf("failed to parse arguments: %w", err)
	}
	// 注入 SafetyChecker
	args["_safety_checker"] = a.safetyChecker

	// 工具本身标记为必须确认：创建 pending action 并中断
	if tool.NeedConfirm {
		return a.createPendingAndReply(sessionID, userID, tool.Name, args, "工具要求确认", userInput)
	}
	//执行工具
	start := time.Now()
	result, err := tool.Handler(args)
	durationMs := time.Since(start).Milliseconds()

	// Handler 返回“需要确认”
	if confirmErr, ok := isConfirmRequired(err); ok {
		return a.createPendingAndReply(sessionID, userID, confirmErr.ToolName, confirmErr.Args, confirmErr.Reason, userInput)
	}

	// 错误恢复机制
	if err != nil {
		if consecutiveFailures != nil {
			// 根据错误类型决定是否计入连续失败
			if classifyError(err) == ErrorNonRetryable {
				// 不可重试的错误，直接计入并可能终止
				*consecutiveFailures = *consecutiveFailures + 1
			} else {
				// 可重试的错误，也计入但给 LLM 机会调整
				*consecutiveFailures = *consecutiveFailures + 1
			}
		}

		// 检查是否超过连续失败阈值
		if consecutiveFailures != nil && *consecutiveFailures >= a.maxConsecutiveFail {
			return "", fmt.Errorf("连续失败 %d 次，终止执行。最后错误: %v", *consecutiveFailures, err)
		}
	} else {
		// 成功，重置计数器
		if consecutiveFailures != nil {
			*consecutiveFailures = 0
		}
	}
	//记录日志
	if a.actionLogger != nil {
		logArgs := make(map[string]any)
		for k, v := range args {
			if k != "_safety_checker" {
				logArgs[k] = v
			}
		}
		if sessionID == "" {
			sessionID = "default-session"
		}
		if userID == "" {
			userID = "default-user"
		}
		_ = a.actionLogger.Log(sessionID, userID, requestID, toolCall.Function.Name, logArgs, result, err, durationMs)
	}
	return result, err
}

func (a *AgentExecutor) createPendingAndReply(sessionID, userID, toolName string, args map[string]any, reason, userInput string) (string, error) {
	if a.memoryManager == nil {
		return "", fmt.Errorf("此操作需要用户确认，但未启用记忆存储，无法创建确认流程")
	}
	if sessionID == "" {
		sessionID = "default-session"
	}
	if userID == "" {
		userID = "default-user"
	}

	// 为了让用户只需回复"确认/取消"，同一会话同一用户只允许存在一个待确认操作
	if existing, err := a.memoryManager.GetLatestPendingAction(sessionID, userID); err == nil && existing != nil {
		// 有未过期 pending，先让用户处理它，避免"确认"指向不明确
		reply := "当前已有一个待确认操作，请先回复：确认 " + existing.ID + "（执行）或 取消 " + existing.ID + "（放弃）。\n\n" + existing.Prompt
		return "", &PendingActionReplyError{ID: existing.ID, Reply: reply}
	}

	// 移除注入字段，避免写入数据库
	safeArgs := make(map[string]any, len(args))
	for k, v := range args {
		if k == "_safety_checker" {
			continue
		}
		safeArgs[k] = v
	}

	argsJSON, _ := json.Marshal(safeArgs)
	id := newPendingID()
	expires := time.Now().Add(defaultConfirmTTL())
	prompt := buildConfirmPrompt(toolName, safeArgs, reason)

	action := &memory.PendingAction{
		ID:        id,
		UserID:    userID,
		SessionID: sessionID,
		UserInput: userInput,
		ToolName:  toolName,
		ArgsJSON:  string(argsJSON),
		Prompt:    prompt,
		Status:    "pending",
		ExpiresAt: &expires,
	}
	if err := a.memoryManager.CreatePendingAction(action); err != nil {
		return "", err
	}

	reply := prompt + "\n\n回复：确认 " + id + "  或  取消 " + id
	return "", &PendingActionReplyError{ID: id, Reply: reply}
}

func newPendingID() string {
	// 8 位：时间后 4 位 + 4 位随机，够用且便于用户手输
	now := time.Now().UnixNano() % 10000
	var b [2]byte
	_, _ = crand.Read(b[:])
	randPart := int(b[0])<<8 | int(b[1])
	randPart = randPart % 10000
	return fmt.Sprintf("%04d%04d", now, randPart)
}

// TryHandleConfirmCancel 处理用户的"确认/取消"消息（不走 LLM）
func (a *AgentExecutor) TryHandleConfirmCancel(userID, sessionID, requestID, text string) (bool, string, map[string]any, error) {
	if a.memoryManager == nil || a.toolRegistry == nil {
		log.Printf("[DEBUG] TryHandleConfirmCancel: memoryManager=%v, toolRegistry=%v", a.memoryManager != nil, a.toolRegistry != nil)
		return false, "", nil, nil
	}
	t := strings.TrimSpace(text)
	if t == "" {
		return false, "", nil, nil
	}

	cmd, id, ok := parseConfirmCancel(t)
	log.Printf("[DEBUG] parseConfirmCancel(%q) -> cmd=%q, id=%q, ok=%v", t, cmd, id, ok)
	if !ok {
		log.Printf("[DEBUG] parseConfirmCancel returned ok=false, skipping confirm handler")
		return false, "", nil, nil
	}

	// 如果没有指定 ID，自动使用最近的待确认操作
	if id == "" {
		pa, err := a.memoryManager.GetLatestPendingAction(sessionID, userID)
		if err != nil {
			log.Printf("[DEBUG] GetLatestPendingAction error: %v", err)
			return true, "", nil, err
		}
		if pa == nil {
			return true, "当前没有待确认的操作。", map[string]any{"kind": "confirm"}, nil
		}
		// 自动填充 ID，继续处理
		id = pa.ID
		log.Printf("[DEBUG] Auto-filled pending action ID: %s", id)
	}

	log.Printf("[DEBUG] GetPendingAction(id=%q), sessionID=%q, userID=%q", id, sessionID, userID)
	pa, err := a.memoryManager.GetPendingAction(id)
	if err != nil {
		log.Printf("[DEBUG] GetPendingAction error: %v", err)
		return true, "", nil, err
	}
	if pa == nil {
		log.Printf("[DEBUG] GetPendingAction returned nil (not found)")
		return true, "未找到待确认操作：" + id, map[string]any{"kind": "confirm"}, nil
	}
	log.Printf("[DEBUG] Found pending action: pa.SessionID=%q, pa.UserID=%q, pa.Status=%q", pa.SessionID, pa.UserID, pa.Status)
	if pa.SessionID != sessionID || pa.UserID != userID {
		return true, "该操作不属于当前会话，无法处理。", map[string]any{"kind": "confirm"}, nil
	}
	if pa.Status != "pending" {
		return true, "该操作已处理（当前状态：" + pa.Status + "）。", map[string]any{"kind": "confirm"}, nil
	}
	if pa.ExpiresAt != nil && time.Now().After(*pa.ExpiresAt) {
		_ = a.memoryManager.UpdatePendingActionStatus(id, "expired", "")
		return true, "该操作已超时失效。", map[string]any{"kind": "confirm"}, nil
	}

	if cmd == "cancel" {
		_ = a.memoryManager.UpdatePendingActionStatus(id, "cancelled", "")
		return true, "已取消操作：" + id, map[string]any{"kind": "confirm"}, nil
	}

	// confirm
	_ = a.memoryManager.UpdatePendingActionStatus(id, "confirmed", "")
	var args map[string]any
	if err := json.Unmarshal([]byte(pa.ArgsJSON), &args); err != nil {
		_ = a.memoryManager.UpdatePendingActionStatus(id, "failed", "invalid args json")
		return true, "待确认操作参数已损坏，无法执行。", map[string]any{"kind": "confirm"}, nil
	}
	args["_safety_checker"] = a.safetyChecker
	args["_confirmed"] = true

	tool, err := a.toolRegistry.Get(pa.ToolName)
	if err != nil {
		_ = a.memoryManager.UpdatePendingActionStatus(id, "failed", err.Error())
		return true, "工具不存在：" + pa.ToolName, map[string]any{"kind": "confirm"}, nil
	}

	start := time.Now()
	result, execErr := tool.Handler(args)
	durationMs := time.Since(start).Milliseconds()
	if execErr != nil {
		_ = a.memoryManager.UpdatePendingActionStatus(id, "failed", execErr.Error())
		result = fmt.Sprintf("❌ 工具执行失败: %s", execErr.Error())
	} else {
		_ = a.memoryManager.UpdatePendingActionStatus(id, "executed", "")
	}
	if a.actionLogger != nil {
		logArgs := make(map[string]any)
		for k, v := range args {
			if k != "_safety_checker" {
				logArgs[k] = v
			}
		}
		_ = a.actionLogger.Log(sessionID, userID, requestID, tool.Name, logArgs, result, nil, durationMs)
	}

	// 继续让 Agent 基于工具输出完成原始任务（而不是直接把工具输出回给用户）
	if strings.TrimSpace(pa.UserInput) == "" {
		return true, result, map[string]any{"kind": "confirm", "action_id": id}, nil
	}
	followup := buildFollowupPrompt(pa.UserInput, pa.ToolName, pa.ArgsJSON, result)
	// 给继续执行设置一个短超时（避免确认后再卡住太久）
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	reply, err := a.executeWithContext(ctx, sessionID, followup, requestID)
	if err != nil {
		// 继续失败时至少返回工具输出
		return true, result + "\n\n（继续处理失败：" + err.Error() + "）", map[string]any{"kind": "confirm", "action_id": id}, nil
	}
	return true, reply, map[string]any{"kind": "confirm", "action_id": id, "continued": true}, nil
}

func parseConfirmCancel(s string) (cmd string, id string, ok bool) {
	// 支持：确认 1234 / 取消 1234 / 确认 / 取消 / y / yes / n / no / 好 / 行
	raw := strings.TrimSpace(s)
	if raw == "" {
		return "", "", false
	}

	lower := strings.ToLower(raw)

	// 支持简短确认（当只有一个待确认操作时，返回空 ID）
	if lower == "y" || lower == "yes" || lower == "ok" ||
		raw == "确认" || raw == "好" || raw == "行" || raw == "是" || raw == "对" {
		return "confirm", "", true
	}
	if lower == "n" || lower == "no" || raw == "取消" || raw == "不" || raw == "否" {
		return "cancel", "", true
	}

	// 允许 "确认1234" 这种无空格形式
	parts := strings.Fields(raw)
	head := parts[0]
	if len(parts) == 0 {
		return "", "", false
	}
	switch head {
	case "确认", "同意", "执行", "yes", "YES":
		cmd = "confirm"
	case "取消", "拒绝", "不执行", "no", "NO":
		cmd = "cancel"
	default:
		// 无空格：确认1234 / 取消1234
		if strings.HasPrefix(raw, "确认") {
			cmd = "confirm"
			parts = []string{"确认", strings.TrimSpace(strings.TrimPrefix(raw, "确认"))}
		} else if strings.HasPrefix(raw, "取消") {
			cmd = "cancel"
			parts = []string{"取消", strings.TrimSpace(strings.TrimPrefix(raw, "取消"))}
		} else {
			return "", "", false
		}
	}
	if len(parts) >= 2 {
		token := strings.TrimSpace(parts[1])
		// 兼容微信/LLM 的加粗、标点等：提取其中的数字
		for _, r := range token {
			if r >= '0' && r <= '9' {
				id += string(r)
			}
		}
	} else {
		// 无空格情况下可能是 "确认1234"，继续从整句中提取数字
		for _, r := range raw {
			if r >= '0' && r <= '9' {
				id += string(r)
			}
		}
	}
	// 如果没有提取到 ID，返回空 ID（表示确认最近的操作）
	if id == "" {
		return cmd, "", true
	}
	if _, err := strconv.Atoi(id); err != nil {
		return cmd, "", true // ID 无效时也返回空 ID
	}
	return cmd, id, true
}

func isConfirmCancelOnly(s string) bool {
	return IsConfirmCancelOnly(s)
}

// IsConfirmCancelOnly 判断消息是否仅为确认/取消指令（导出供外部使用）
func IsConfirmCancelOnly(s string) bool {
	t := strings.TrimSpace(s)
	if t == "" {
		return false
	}
	// 支持简短确认
	lower := strings.ToLower(t)
	if lower == "y" || lower == "yes" || lower == "ok" ||
		t == "确认" || t == "好" || t == "行" ||
		lower == "n" || lower == "no" || t == "取消" || t == "不" {
		return true
	}
	if strings.HasPrefix(t, "确认") || strings.HasPrefix(t, "取消") {
		// 避免误判长句，只对短指令生效
		return len([]rune(t)) <= 24
	}
	return false
}

func buildFollowupPrompt(userInput, toolName, argsJSON, toolOutput string) string {
	out := toolOutput
	if len(out) > 4000 {
		out = out[:4000] + "\n...(输出过长已截断)"
	}
	return "继续完成用户原始需求：\n" + userInput + "\n\n" +
		"补充：用户已确认并执行了一次工具调用（不要重复执行同一调用，除非必要）：\n" +
		"tool=" + toolName + "\nargs=" + argsJSON + "\noutput:\n" + out + "\n\n" +
		"请基于上述输出继续完成原始任务，必要时可继续调用工具。"
}

// saveEpisode 保存执行记录为 Episode（长期记忆）
func (a *AgentExecutor) saveEpisode(sessionID, userID, userInput string, steps []memory.TaskStep, outcome, summary string) {
	if a.memoryManager == nil || len(steps) == 0 {
		return
	}

	// 提取目标
	target := extractTarget(steps)

	// 压缩 summary
	if len(summary) > 500 {
		summary = summary[:500] + "..."
	}

	episode := &memory.Episode{
		SessionID:      sessionID,
		UserID:         userID,
		TriggerType:    "user_request",
		TriggerSummary: userInput,
		Steps:          steps,
		Outcome:        outcome,
		OutcomeSummary: summary,
		Target:         target,
		Importance:     calculateImportance(steps, outcome),
	}

	if err := a.memoryManager.SaveEpisode(episode); err != nil {
		log.Printf("[WARN] Failed to save episode: %v", err)
	} else {
		log.Printf("[INFO] Episode saved: sessionID=%s, steps=%d, outcome=%s", sessionID, len(steps), outcome)
	}
}

// extractTarget 从执行步骤中提取操作目标
func extractTarget(steps []memory.TaskStep) string {
	for _, step := range steps {
		switch step.ToolName {
		case "docker":
			if container, ok := step.Args["container"].(string); ok && container != "" {
				return "docker:" + container
			}
		case "local_shell":
			if cmd, ok := step.Args["command"].(string); ok {
				if t := extractTargetFromCommand(cmd); t != "" {
					return t
				}
			}
		case "remote_shell":
			if host, ok := step.Args["host"].(string); ok && host != "" {
				return "host:" + host
			}
		}
	}
	return ""
}

// extractTargetFromCommand 从 shell 命令中提取目标
func extractTargetFromCommand(cmd string) string {
	// psql -d dbname 或 psql -U user -d dbname
	if strings.Contains(cmd, "psql") {
		if idx := strings.Index(cmd, "-d "); idx != -1 {
			parts := strings.Fields(cmd[idx+3:])
			if len(parts) > 0 {
				return "db:" + parts[0]
			}
		}
	}
	// mysql -D dbname 或 mysql dbname
	if strings.Contains(cmd, "mysql") {
		if idx := strings.Index(cmd, "-D "); idx != -1 {
			parts := strings.Fields(cmd[idx+3:])
			if len(parts) > 0 {
				return "db:" + parts[0]
			}
		}
	}
	// docker exec container_name
	if strings.Contains(cmd, "docker exec") {
		parts := strings.Fields(cmd)
		for i, p := range parts {
			if p == "exec" && i+1 < len(parts) {
				next := parts[i+1]
				if !strings.HasPrefix(next, "-") {
					return "docker:" + next
				}
			}
		}
	}
	return ""
}

// calculateImportance 计算 Episode 重要性
func calculateImportance(steps []memory.TaskStep, outcome string) float64 {
	importance := 0.5 // 基础分

	// 有写操作的更重要
	for _, step := range steps {
		if step.ToolName == "docker" {
			if action, ok := step.Args["action"].(string); ok {
				if action == "stop" || action == "start" || action == "restart" {
					importance += 0.2
				}
			}
		}
	}

	// 成功的操作更重要
	if outcome == "success" {
		importance += 0.1
	}

	// 限制在 0-1 范围
	if importance > 1.0 {
		importance = 1.0
	}

	return importance
}

func (a *AgentExecutor) executeWithContext(ctx context.Context, sessionID, userInput, requestID string) (string, error) {
	type out struct {
		reply string
		err   error
	}
	ch := make(chan out, 1)
	go func() {
		reply, err := a.Execute(sessionID, userInput, requestID)
		ch <- out{reply: reply, err: err}
	}()
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case r := <-ch:
		return r.reply, r.err
	}
}

func (a *AgentExecutor) buildPlanContext(plan *ExecutionPlan) string {
	if plan == nil || len(plan.Steps) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("## 执行计划\n\n")
	sb.WriteString(fmt.Sprintf("任务：%s\n\n", plan.TaskSummary))
	sb.WriteString("请按以下步骤执行：\n")

	for _, step := range plan.Steps {
		sb.WriteString(fmt.Sprintf("%d. %s（使用 %s）\n", step.StepNum, step.Description, step.Tool))
	}
	sb.WriteString("\n注意：按顺序执行，每步完成后再进行下一步。如果某步失败，分析原因后决定是否继续。")
	return sb.String()
}

// truncateToolResult 截断过长的工具输出，保留头尾关键信息
func truncateToolResult(result string) string {
	const maxToolResultLines = 50
	const maxToolResultChars = 4000

	lines := strings.Split(result, "\n")
	if len(lines) > maxToolResultLines {
		head := strings.Join(lines[:20], "\n")
		tail := strings.Join(lines[len(lines)-10:], "\n")
		return fmt.Sprintf("%s\n\n... [省略 %d 行] ...\n\n%s",
			head, len(lines)-30, tail)
	}
	if len(result) > maxToolResultChars {
		return result[:maxToolResultChars] + "\n... [输出已截断]"
	}
	return result
}

// compressForHistory 压缩长内容用于历史存储
func compressForHistory(content string) string {
	if len(content) < 500 {
		return content
	}
	// 保留前 200 字符 + 摘要标记 + 后 100 字符
	return content[:200] + "\n...[内容已压缩]...\n" + content[len(content)-100:]
}
