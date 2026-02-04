package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type DeepSeekClient struct {
	apiKey  string
	baseURL string
	model   string
	client  *http.Client
	retries int
}

func NewDeepSeekClient(apiKey, baseURL, model string) *DeepSeekClient {
	return &DeepSeekClient{
		apiKey:  apiKey,
		baseURL: baseURL,
		model:   model,
		client: &http.Client{
			Timeout: 360 * time.Second,
		},
		retries: 0,
	}
}

func (d *DeepSeekClient) SetTimeout(timeout time.Duration) {
	if timeout > 0 {
		d.client.Timeout = timeout
	}
}

func (d *DeepSeekClient) SetRetries(retries int) {
	if retries < 0 {
		retries = 0
	}
	d.retries = retries
}

type httpStatusError struct {
	Code int
	Body string
}

func (e *httpStatusError) Error() string {
	body := e.Body
	if len(body) > 300 {
		body = body[:300] + "..."
	}
	return fmt.Sprintf("HTTP %d: %s", e.Code, body)
}

type ChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Tools    []Tool    `json:"tools,omitempty"`
}

type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

type ChatResponse struct {
	ID      string   `json:"id"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"` // "stop" or "tool_calls"
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

func (d *DeepSeekClient) Chat(prompt string) (string, error) {
	req := ChatRequest{
		Model: d.model,
		Messages: []Message{
			{Role: "system", Content: "你是用户的服务器运维助手"},
			{Role: "user", Content: prompt},
		},
	}

	var lastErr error
	for attempt := 0; attempt <= d.retries; attempt++ {
		res, err := d.doChat(req)
		if err == nil {
			return res.Choices[0].Message.Content, nil
		}
		lastErr = err
		if !isRetryableLLMError(err) || attempt == d.retries {
			break
		}
		time.Sleep(time.Duration(200*(attempt+1)) * time.Millisecond)
	}
	return "", lastErr
}

func (d *DeepSeekClient) ChatWithTools(messages []Message, tools []Tool) (*ChatResponse, error) {
	req := ChatRequest{
		Model:    d.model,
		Messages: messages,
		Tools:    tools,
	}

	var lastErr error
	for attempt := 0; attempt <= d.retries; attempt++ {
		res, err := d.doChat(req)
		if err == nil {
			return res, nil
		}
		lastErr = err
		if !isRetryableLLMError(err) || attempt == d.retries {
			break
		}
		time.Sleep(time.Duration(200*(attempt+1)) * time.Millisecond)
	}
	return nil, lastErr
}

func (d *DeepSeekClient) doChat(req ChatRequest) (*ChatResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	HTTPReq, err := http.NewRequest("POST", d.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	HTTPReq.Header.Set("Content-Type", "application/json")
	HTTPReq.Header.Set("Authorization", "Bearer "+d.apiKey)

	HTTPRes, err := d.client.Do(HTTPReq)
	if err != nil {
		return nil, err
	}
	defer HTTPRes.Body.Close()

	resBody, err := io.ReadAll(HTTPRes.Body)
	if err != nil {
		return nil, err
	}
	if HTTPRes.StatusCode < 200 || HTTPRes.StatusCode >= 300 {
		return nil, &httpStatusError{Code: HTTPRes.StatusCode, Body: string(resBody)}
	}

	var res ChatResponse
	if err := json.Unmarshal(resBody, &res); err != nil {
		return nil, err
	}
	if len(res.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}
	return &res, nil
}

func isRetryableLLMError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	if strings.Contains(err.Error(), "Client.Timeout") || strings.Contains(err.Error(), "context deadline exceeded") {
		return true
	}
	var se *httpStatusError
	if errors.As(err, &se) {
		if se.Code == 429 {
			return true
		}
		if se.Code >= 500 && se.Code <= 599 {
			return true
		}
	}
	return false
}
