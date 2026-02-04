package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Request structures
type ChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Tools    []Tool    `json:"tools,omitempty"`
}

type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
}

type Tool struct {
	Type     string   `json:"type"`
	Function Function `json:"function"`
}

type Function struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// Response structures
type ChatResponse struct {
	ID      string   `json:"id"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

func main() {
	fmt.Println("🔍 DeepSeek Function Calling 验证脚本")
	fmt.Println("=====================================\n")

	// 从环境变量或配置读取
	apiKey := "sk-bpbtodncdyqvavluhnxysqdxnxlnyzhzdpbjwxaxtuspybnz"
	baseURL := "https://api.siliconflow.cn/v1"
	model := "deepseek-ai/DeepSeek-V3.2"

	// 测试 1: 简单工具调用
	fmt.Println("📝 测试 1: 简单工具调用")
	testSimpleToolCall(apiKey, baseURL, model)

	fmt.Println("\n📝 测试 2: 多工具定义")
	testMultipleTools(apiKey, baseURL, model)

	fmt.Println("\n📝 测试 3: 不需要工具的查询")
	testNoToolNeeded(apiKey, baseURL, model)
}

func testSimpleToolCall(apiKey, baseURL, model string) {
	tools := []Tool{
		{
			Type: "function",
			Function: Function{
				Name:        "get_weather",
				Description: "获取指定城市的天气信息",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"city": map[string]interface{}{
							"type":        "string",
							"description": "城市名称",
						},
					},
					"required": []string{"city"},
				},
			},
		},
	}

	messages := []Message{
		{Role: "user", Content: "北京今天天气怎么样？"},
	}

	resp, err := callAPI(apiKey, baseURL, model, messages, tools)
	if err != nil {
		fmt.Printf("❌ API 调用失败: %v\n", err)
		return
	}

	// 验证响应
	fmt.Printf("✅ API 调用成功\n")
	fmt.Printf("   finish_reason: %s\n", resp.Choices[0].FinishReason)

	if resp.Choices[0].FinishReason == "tool_calls" {
		fmt.Printf("✅ 返回了 tool_calls\n")
		toolCalls := resp.Choices[0].Message.ToolCalls
		if len(toolCalls) > 0 {
			fmt.Printf("✅ tool_calls 数量: %d\n", len(toolCalls))
			for i, tc := range toolCalls {
				fmt.Printf("   [%d] ID: %s\n", i, tc.ID)
				fmt.Printf("   [%d] Type: %s\n", i, tc.Type)
				fmt.Printf("   [%d] Function: %s\n", i, tc.Function.Name)
				fmt.Printf("   [%d] Arguments: %s\n", i, tc.Function.Arguments)
			}
		}
	} else {
		fmt.Printf("⚠️  finish_reason 不是 'tool_calls'，而是 '%s'\n", resp.Choices[0].FinishReason)
		fmt.Printf("   响应内容: %s\n", resp.Choices[0].Message.Content)
	}

	fmt.Printf("   Token 使用: %d (prompt) + %d (completion) = %d\n",
		resp.Usage.PromptTokens, resp.Usage.CompletionTokens, resp.Usage.TotalTokens)
}

func testMultipleTools(apiKey, baseURL, model string) {
	tools := []Tool{
		{
			Type: "function",
			Function: Function{
				Name:        "docker_restart",
				Description: "重启 Docker 容器",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"container": map[string]interface{}{
							"type":        "string",
							"description": "容器名称",
						},
					},
					"required": []string{"container"},
				},
			},
		},
		{
			Type: "function",
			Function: Function{
				Name:        "docker_logs",
				Description: "查看 Docker 容器日志",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"container": map[string]interface{}{
							"type":        "string",
							"description": "容器名称",
						},
						"lines": map[string]interface{}{
							"type":        "integer",
							"description": "显示的行数",
						},
					},
					"required": []string{"container"},
				},
			},
		},
	}

	messages := []Message{
		{Role: "user", Content: "重启 nginx 容器"},
	}

	resp, err := callAPI(apiKey, baseURL, model, messages, tools)
	if err != nil {
		fmt.Printf("❌ API 调用失败: %v\n", err)
		return
	}

	fmt.Printf("✅ API 调用成功\n")
	fmt.Printf("   finish_reason: %s\n", resp.Choices[0].FinishReason)

	if resp.Choices[0].FinishReason == "tool_calls" {
		toolCalls := resp.Choices[0].Message.ToolCalls
		if len(toolCalls) > 0 {
			fmt.Printf("✅ LLM 选择了正确的工具: %s\n", toolCalls[0].Function.Name)
			fmt.Printf("   参数: %s\n", toolCalls[0].Function.Arguments)
		}
	}
}

func testNoToolNeeded(apiKey, baseURL, model string) {
	tools := []Tool{
		{
			Type: "function",
			Function: Function{
				Name:        "get_weather",
				Description: "获取天气信息",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"city": map[string]interface{}{
							"type": "string",
						},
					},
					"required": []string{"city"},
				},
			},
		},
	}

	messages := []Message{
		{Role: "user", Content: "你好，介绍一下你自己"},
	}

	resp, err := callAPI(apiKey, baseURL, model, messages, tools)
	if err != nil {
		fmt.Printf("❌ API 调用失败: %v\n", err)
		return
	}

	fmt.Printf("✅ API 调用成功\n")
	fmt.Printf("   finish_reason: %s\n", resp.Choices[0].FinishReason)

	if resp.Choices[0].FinishReason == "stop" {
		fmt.Printf("✅ 正确返回 'stop'（不需要工具）\n")
		fmt.Printf("   响应: %s\n", resp.Choices[0].Message.Content)
	} else {
		fmt.Printf("⚠️  finish_reason 应该是 'stop'，但是是 '%s'\n", resp.Choices[0].FinishReason)
	}
}

func callAPI(apiKey, baseURL, model string, messages []Message, tools []Tool) (*ChatResponse, error) {
	req := ChatRequest{
		Model:    model,
		Messages: messages,
		Tools:    tools,
	}

	reqBytes, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequest("POST", baseURL+"/chat/completions", bytes.NewReader(reqBytes))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 60 * time.Second}
	httpResp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer httpResp.Body.Close()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, err
	}

	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d: %s", httpResp.StatusCode, string(body))
	}

	var resp ChatResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		// 打印原始响应帮助调试
		fmt.Printf("⚠️  JSON 解析失败，原始响应:\n%s\n", string(body))
		return nil, err
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	return &resp, nil
}
