package middleware

import (
	"log"
	"net/http"
	"time"
)

// responseWriter 包装 http.ResponseWriter 以捕获状态码
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	written    int64
}

// newResponseWriter 创建新的 responseWriter
func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
	}
}

// WriteHeader 重写 WriteHeader 方法以捕获状态码
func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Write 重写 Write 方法以统计写入字节数
func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.written += int64(n)
	return n, err
}

// LoggingMiddleware 创建日志中间件
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// 包装 ResponseWriter
		rw := newResponseWriter(w)

		// 处理请求
		next.ServeHTTP(rw, r)

		// 计算耗时
		duration := time.Since(start)

		// 记录日志
		log.Printf("[HTTP] %s %s %d %s %d bytes",
			r.Method,
			r.URL.Path,
			rw.statusCode,
			duration,
			rw.written,
		)
	})
}

// LoggingMiddlewareWithConfig 创建带配置的日志中间件
type LoggingConfig struct {
	// 跳过日志的路径
	SkipPaths []string

	// 自定义日志函数
	LogFunc func(method, path string, statusCode int, duration time.Duration, written int64)
}

// LoggingMiddlewareWithConfig 创建带配置的日志中间件
func LoggingMiddlewareWithConfig(next http.Handler, config *LoggingConfig) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 检查是否跳过日志
		for _, path := range config.SkipPaths {
			if r.URL.Path == path {
				next.ServeHTTP(w, r)
				return
			}
		}

		start := time.Now()

		// 包装 ResponseWriter
		rw := newResponseWriter(w)

		// 处理请求
		next.ServeHTTP(rw, r)

		// 计算耗时
		duration := time.Since(start)

		// 记录日志
		if config.LogFunc != nil {
			config.LogFunc(r.Method, r.URL.Path, rw.statusCode, duration, rw.written)
		} else {
			log.Printf("[HTTP] %s %s %d %s %d bytes",
				r.Method,
				r.URL.Path,
				rw.statusCode,
				duration,
				rw.written,
			)
		}
	})
}
