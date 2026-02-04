package main

import (
	"encoding/json"
	"fmt"
)

func main() {
	fmt.Println("📋 DeepSeek Function Calling API 格式参考")
	fmt.Println("==========================================\n")

	// 1. 请求格式
	fmt.Println("1️⃣ 请求体格式 (Request Body)")
	fmt.Println("----------------------------")

	request := map[string]interface{}{
		"model": "deepseek-ai/DeepSeek-V3.2",
		"messages": []map[string]interface{}{
			{
				"role":    "user",
				"content": "北京今天天气怎么样？",
			},
		},
		"tools": []map[string]interface{}{
			{
				"type": "function",
				"function": map[string]interface{}{
					"name":        "get_weather",
					"description": "获取指定城市的天气信息",
					"parameters": map[string]interface{}{
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
		},
	}

	reqJSON, _ := json.MarshalIndent(request, "", "  ")
	fmt.Println(string(reqJSON))

	// 2. 响应格式（tool_calls）
	fmt.Println("\n\n2️⃣ 响应体格式 - 当 LLM 调用工具时")
	fmt.Println("-----------------------------------")

	responseWithTools := map[string]interface{}{
		"id": "019be5c639b80de2406e83ef8b82a804",
		"choices": []map[string]interface{}{
			{
				"index": 0,
				"message": map[string]interface{}{
					"role":    "assistant",
					"content": nil, // 当调用工具时，content 可能为 null
					"tool_calls": []map[string]interface{}{
						{
							"id":   "call_abc123",
							"type": "function",
							"function": map[string]interface{}{
								"name":      "get_weather",
								"arguments": "{\"city\": \"北京\"}",
							},
						},
					},
				},
				"finish_reason": "tool_calls",
			},
		},
		"usage": map[string]interface{}{
			"prompt_tokens":     319,
			"completion_tokens": 52,
			"total_tokens":      371,
		},
	}

	respJSON, _ := json.MarshalIndent(responseWithTools, "", "  ")
	fmt.Println(string(respJSON))

	// 3. 响应格式（stop）
	fmt.Println("\n\n3️⃣ 响应体格式 - 当 LLM 直接回答时")
	fmt.Println("-----------------------------------")

	responseWithoutTools := map[string]interface{}{
		"id": "019be5c639b80de2406e83ef8b82a805",
		"choices": []map[string]interface{}{
			{
				"index": 0,
				"message": map[string]interface{}{
					"role":    "assistant",
					"content": "你好！我是一个AI智能助手...",
				},
				"finish_reason": "stop",
			},
		},
		"usage": map[string]interface{}{
			"prompt_tokens":     100,
			"completion_tokens": 50,
			"total_tokens":      150,
		},
	}

	respJSON2, _ := json.MarshalIndent(responseWithoutTools, "", "  ")
	fmt.Println(string(respJSON2))

	// 4. ReAct 循环中的消息格式
	fmt.Println("\n\n4️⃣ ReAct 循环中的消息格式")
	fmt.Println("----------------------------")
	fmt.Println("第一轮：用户提问")

	round1 := map[string]interface{}{
		"model": "deepseek-ai/DeepSeek-V3.2",
		"messages": []map[string]interface{}{
			{"role": "system", "content": "你是运维助手"},
			{"role": "user", "content": "重启 nginx"},
		},
		"tools": []map[string]interface{}{
			{
				"type": "function",
				"function": map[string]interface{}{
					"name":        "docker_restart",
					"description": "重启容器",
					"parameters": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"container": map[string]string{"type": "string"},
						},
						"required": []string{"container"},
					},
				},
			},
		},
	}

	r1JSON, _ := json.MarshalIndent(round1, "", "  ")
	fmt.Println(string(r1JSON))

	fmt.Println("\n第二轮：LLM 返回 tool_calls，我们执行工具后，将结果发回")

	round2 := map[string]interface{}{
		"model": "deepseek-ai/DeepSeek-V3.2",
		"messages": []map[string]interface{}{
			{"role": "system", "content": "你是运维助手"},
			{"role": "user", "content": "重启 nginx"},
			// LLM 的 tool_calls 响应
			{
				"role": "assistant",
				"tool_calls": []map[string]interface{}{
					{
						"id":   "call_abc123",
						"type": "function",
						"function": map[string]interface{}{
							"name":      "docker_restart",
							"arguments": "{\"container\": \"nginx\"}",
						},
					},
				},
			},
			// 工具执行结果
			{
				"role":         "tool",
				"tool_call_id": "call_abc123",
				"content":      "✅ nginx 已重启",
			},
		},
		"tools": []map[string]interface{}{
			{
				"type": "function",
				"function": map[string]interface{}{
					"name":        "docker_restart",
					"description": "重启容器",
					"parameters": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"container": map[string]string{"type": "string"},
						},
						"required": []string{"container"},
					},
				},
			},
		},
	}

	r2JSON, _ := json.MarshalIndent(round2, "", "  ")
	fmt.Println(string(r2JSON))

	fmt.Println("\n第三轮：LLM 看到工具结果后，给用户最终回复")
	fmt.Println("finish_reason = 'stop'")
	fmt.Println("content = '✅ 已成功重启 nginx 容器'")

	// 5. 关键点总结
	fmt.Println("\n\n5️⃣ 关键点总结")
	fmt.Println("----------------------------")
	fmt.Println("✅ tools 参数是可选的，不传就是普通对话")
	fmt.Println("✅ finish_reason 有两种值：")
	fmt.Println("   - 'tool_calls': LLM 想调用工具")
	fmt.Println("   - 'stop': LLM 直接回答")
	fmt.Println("✅ tool_calls 是数组，可能同时调用多个工具")
	fmt.Println("✅ arguments 是 JSON 字符串，需要 unmarshal")
	fmt.Println("✅ 工具结果通过 role='tool' 的消息返回")
	fmt.Println("✅ tool_call_id 用于关联工具调用和结果")
}
