package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
)

// Transport MCP 传输层接口
type Transport interface {
	Start(ctx context.Context) error
	Stop() error
}

// StdioTransport 标准输入输出传输（用于 Claude Desktop）
type StdioTransport struct {
	server *MCPServer
	reader *bufio.Reader
	writer io.Writer
	done   chan struct{}
	mu     sync.Mutex
}

// NewStdioTransport 创建标准输入输出传输
func NewStdioTransport(server *MCPServer) *StdioTransport {
	return &StdioTransport{
		server: server,
		reader: bufio.NewReader(os.Stdin),
		writer: os.Stdout,
		done:   make(chan struct{}),
	}
}

// Start 启动标准输入输出传输
func (t *StdioTransport) Start(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.done:
			return nil
		default:
			// 读取一行 JSON
			line, err := t.reader.ReadBytes('\n')
			if err != nil {
				if err == io.EOF {
					return nil
				}
				continue
			}

			// 跳过空行
			if len(line) <= 1 {
				continue
			}

			// 处理请求
			response, err := t.server.HandleRequest(line)
			if err != nil {
				// 记录错误但继续处理
				continue
			}

			// 发送响应（如果有）
			if response != nil {
				t.mu.Lock()
				t.writer.Write(response)
				t.writer.Write([]byte("\n"))
				t.mu.Unlock()
			}
		}
	}
}

// Stop 停止标准输入输出传输
func (t *StdioTransport) Stop() error {
	close(t.done)
	return nil
}

// WriteNotification 发送通知（服务端主动推送）
func (t *StdioTransport) WriteNotification(method string, params any) error {
	notification := map[string]any{
		"jsonrpc": JSONRPCVersion,
		"method":  method,
	}
	if params != nil {
		notification["params"] = params
	}

	data, err := json.Marshal(notification)
	if err != nil {
		return err
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	t.writer.Write(data)
	t.writer.Write([]byte("\n"))
	return nil
}

// HTTPTransport HTTP 传输（用于 Web 集成）
type HTTPTransport struct {
	server     *MCPServer
	httpServer *http.Server
	addr       string
	done       chan struct{}
}

// NewHTTPTransport 创建 HTTP 传输
func NewHTTPTransport(server *MCPServer, addr string) *HTTPTransport {
	return &HTTPTransport{
		server: server,
		addr:   addr,
		done:   make(chan struct{}),
	}
}

// Start 启动 HTTP 传输
func (t *HTTPTransport) Start(ctx context.Context) error {
	mux := http.NewServeMux()

	// MCP JSON-RPC 端点
	mux.HandleFunc("/mcp", t.handleMCP)

	// 健康检查端点
	mux.HandleFunc("/health", t.handleHealth)

	// SSE 端点（用于服务端推送）
	mux.HandleFunc("/mcp/sse", t.handleSSE)

	t.httpServer = &http.Server{
		Addr:    t.addr,
		Handler: mux,
	}

	// 启动服务器
	errChan := make(chan error, 1)
	go func() {
		if err := t.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	select {
	case <-ctx.Done():
		return t.Stop()
	case <-t.done:
		return nil
	case err := <-errChan:
		return err
	}
}

// Stop 停止 HTTP 传输
func (t *HTTPTransport) Stop() error {
	close(t.done)
	if t.httpServer != nil {
		return t.httpServer.Shutdown(context.Background())
	}
	return nil
}

// handleMCP 处理 MCP 请求
func (t *HTTPTransport) handleMCP(w http.ResponseWriter, r *http.Request) {
	// 只接受 POST 请求
	if r.Method != http.MethodPost {
		http.Error(w, "只支持 POST 请求", http.StatusMethodNotAllowed)
		return
	}

	// 读取请求体
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "读取请求失败", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// 处理请求
	response, err := t.server.HandleRequest(body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 发送响应
	w.Header().Set("Content-Type", "application/json")
	if response != nil {
		w.Write(response)
	}
}

// handleHealth 处理健康检查
func (t *HTTPTransport) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"status":      "ok",
		"initialized": t.server.IsInitialized(),
		"server":      t.server.GetServerInfo(),
	})
}

// handleSSE 处理 Server-Sent Events（用于服务端推送通知）
func (t *HTTPTransport) handleSSE(w http.ResponseWriter, r *http.Request) {
	// 设置 SSE 头
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "不支持 SSE", http.StatusInternalServerError)
		return
	}

	// 发送初始连接事件
	fmt.Fprintf(w, "event: connected\ndata: {\"status\":\"connected\"}\n\n")
	flusher.Flush()

	// 保持连接直到客户端断开
	<-r.Context().Done()
}

// SSEClient SSE 客户端连接
type SSEClient struct {
	id       string
	messages chan []byte
}

// HTTPTransportWithSSE 带 SSE 支持的 HTTP 传输
type HTTPTransportWithSSE struct {
	*HTTPTransport
	clients   map[string]*SSEClient
	clientsMu sync.RWMutex
}

// NewHTTPTransportWithSSE 创建带 SSE 支持的 HTTP 传输
func NewHTTPTransportWithSSE(server *MCPServer, addr string) *HTTPTransportWithSSE {
	return &HTTPTransportWithSSE{
		HTTPTransport: NewHTTPTransport(server, addr),
		clients:       make(map[string]*SSEClient),
	}
}

// BroadcastNotification 广播通知给所有 SSE 客户端
func (t *HTTPTransportWithSSE) BroadcastNotification(method string, params any) error {
	notification := map[string]any{
		"jsonrpc": JSONRPCVersion,
		"method":  method,
	}
	if params != nil {
		notification["params"] = params
	}

	data, err := json.Marshal(notification)
	if err != nil {
		return err
	}

	t.clientsMu.RLock()
	defer t.clientsMu.RUnlock()

	for _, client := range t.clients {
		select {
		case client.messages <- data:
		default:
			// 客户端消息队列已满，跳过
		}
	}

	return nil
}

// WebSocketTransport WebSocket 传输（可选，用于双向实时通信）
type WebSocketTransport struct {
	server *MCPServer
	addr   string
	done   chan struct{}
}

// NewWebSocketTransport 创建 WebSocket 传输
func NewWebSocketTransport(server *MCPServer, addr string) *WebSocketTransport {
	return &WebSocketTransport{
		server: server,
		addr:   addr,
		done:   make(chan struct{}),
	}
}

// Start 启动 WebSocket 传输
// 注意：完整实现需要引入 gorilla/websocket 或类似库
func (t *WebSocketTransport) Start(ctx context.Context) error {
	// WebSocket 实现需要额外依赖
	// 这里提供基本框架，实际使用时需要引入 websocket 库
	return fmt.Errorf("WebSocket 传输需要额外依赖，请使用 HTTP 或 Stdio 传输")
}

// Stop 停止 WebSocket 传输
func (t *WebSocketTransport) Stop() error {
	close(t.done)
	return nil
}
