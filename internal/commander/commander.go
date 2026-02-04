package commander

import (
	"encoding/json"
	"fmt"
	"serverowl/internal/llm"
	"serverowl/internal/memory"
	"strings"
)

type Commander struct {
	llm           llm.Client
	containers    []string
	memoryManager *memory.MemoryManager
}

func NewCommander(llmClient llm.Client, containers []string, memManager *memory.MemoryManager) *Commander {
	return &Commander{
		llm:           llmClient,
		containers:    containers,
		memoryManager: memManager,
	}
}

// ParseIntent 解析用户输入，返回意图
func (c *Commander) ParseIntent(userInput, sessionID string) (*Intent, error) {
	// 1. 加载记忆上下文
	var memoryContext string
	if c.memoryManager != nil {
		ctx, err := c.memoryManager.BuildContext(sessionID, userInput)
		if err == nil && ctx != nil {
			memoryContext = c.memoryManager.FormatContextForLLM(ctx)
		}
	}

	// 2. 构造 Prompt（注入记忆上下文）
	prompt := c.buildPrompt(userInput, memoryContext)

	// 3. 调用 LLM
	resp, err := c.llm.Chat(prompt)
	if err != nil {
		return nil, err
	}

	// 4. 解析响应
	var intent Intent
	if err := json.Unmarshal([]byte(resp), &intent); err != nil {
		return &Intent{Type: "unknown"}, nil
	}

	return &intent, nil
}

// buildPrompt 构造意图解析的 Prompt
func (c *Commander) buildPrompt(userInput, memoryContext string) string {
	var prompt strings.Builder

	prompt.WriteString("你是 ServerOwl 运维助手的意图解析器。\n\n")

	// 注入记忆上下文
	if memoryContext != "" {
		prompt.WriteString("## 历史上下文\n")
		prompt.WriteString(memoryContext)
		prompt.WriteString("\n")
	}

	prompt.WriteString(fmt.Sprintf("用户输入: \"%s\"\n\n", userInput))

	prompt.WriteString("请分析用户意图，返回 JSON：\n")
	prompt.WriteString("{\n")
	prompt.WriteString("  \"type\": \"status|restart|logs|help|unknown\",\n")
	prompt.WriteString("  \"target\": \"容器名或服务名，没有则为空字符串\",\n")
	prompt.WriteString("  \"params\": {}\n")
	prompt.WriteString("}\n\n")

	prompt.WriteString(fmt.Sprintf("已知容器列表：%v\n\n", c.containers))

	prompt.WriteString("规则：\n")
	prompt.WriteString("- status: 查看状态，如\"看看服务状态\"、\"现在怎么样\"\n")
	prompt.WriteString("- restart: 重启容器，如\"重启 shortlink\"、\"shortlink 挂了重启下\"\n")
	prompt.WriteString("- logs: 查看日志，如\"看看 app 的日志\"、\"shortlink 有什么报错\"\n")
	prompt.WriteString("- help: 帮助，如\"你能干什么\"、\"帮助\"\n")
	prompt.WriteString("- unknown: 无法识别或者用户的意图不属于其他任何一项。\n\n")

	prompt.WriteString("注意：如果用户说\"再重启一次\"、\"再看看\"等，请根据历史上下文推断目标。\n\n")

	prompt.WriteString("只返回 JSON，不要解释。")

	return prompt.String()
}
