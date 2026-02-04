package middleware

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// AuthConfig 认证配置
type AuthConfig struct {
	// Basic Auth
	Username string
	Password string

	// Token Auth
	Token string

	// 跳过认证的路径
	SkipPaths []string
}

// AuthMiddleware 创建认证中间件
// 支持 Basic Auth 和 Bearer Token 两种方式
func AuthMiddleware(next http.Handler, config *AuthConfig) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 检查是否跳过认证
		for _, path := range config.SkipPaths {
			if strings.HasPrefix(r.URL.Path, path) {
				next.ServeHTTP(w, r)
				return
			}
		}

		// 获取 Authorization 头
		auth := r.Header.Get("Authorization")
		if auth == "" {
			unauthorized(w, "缺少认证信息")
			return
		}

		// 尝试 Bearer Token 认证
		if strings.HasPrefix(auth, "Bearer ") {
			token := strings.TrimPrefix(auth, "Bearer ")
			if config.Token != "" && secureCompare(token, config.Token) {
				next.ServeHTTP(w, r)
				return
			}
			unauthorized(w, "无效的 Token")
			return
		}

		// 尝试 Basic Auth 认证
		if strings.HasPrefix(auth, "Basic ") {
			username, password, ok := r.BasicAuth()
			if !ok {
				unauthorized(w, "无效的 Basic Auth 格式")
				return
			}

			if config.Username != "" && config.Password != "" {
				if secureCompare(username, config.Username) && secureCompare(password, config.Password) {
					next.ServeHTTP(w, r)
					return
				}
			}
			unauthorized(w, "用户名或密码错误")
			return
		}

		unauthorized(w, "不支持的认证方式")
	})
}

// BasicAuthMiddleware 创建简单的 Basic Auth 中间件
func BasicAuthMiddleware(next http.Handler, username, password string) http.Handler {
	return AuthMiddleware(next, &AuthConfig{
		Username: username,
		Password: password,
	})
}

// TokenAuthMiddleware 创建简单的 Token 认证中间件
func TokenAuthMiddleware(next http.Handler, token string) http.Handler {
	return AuthMiddleware(next, &AuthConfig{
		Token: token,
	})
}

// unauthorized 返回 401 未授权响应
func unauthorized(w http.ResponseWriter, message string) {
	w.Header().Set("WWW-Authenticate", `Basic realm="ServerOwl API"`)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusUnauthorized)
	w.Write([]byte(`{"success":false,"error":"` + message + `"}`))
}

// secureCompare 安全比较两个字符串，防止时序攻击
func secureCompare(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
