package builtin

import (
	"time"

	"serverowl/internal/skills"
)

// CaddyReloadSkill 返回 Caddy 重载技能
func CaddyReloadSkill() *skills.Skill {
	return &skills.Skill{
		Name:        "builtin:caddy_reload",
		Description: "验证 Caddy 配置并重载",
		Version:     "1.0.0",
		Author:      "ServerOwl",
		Tags:        []string{"caddy", "web", "reload", "builtin"},
		Parameters:  []skills.SkillParameter{},
		Timeout:     2 * time.Minute,
		NeedConfirm: false,
		Steps: []skills.SkillStep{
			{
				Name: "validate_config",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `echo "=== 验证 Caddy 配置 ===" && caddy validate --config /etc/caddy/Caddyfile 2>&1`,
				},
				OnError: "stop",
			},
			{
				Name: "reload",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `echo "=== 重载 Caddy ===" && systemctl reload caddy && echo "Caddy 重载成功"`,
				},
				OnError: "stop",
			},
			{
				Name: "verify",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `echo "=== 验证状态 ===" && systemctl status caddy --no-pager | head -15`,
				},
				OnError: "continue",
			},
		},
	}
}

// CaddyStatusSkill 返回 Caddy 状态检查技能
func CaddyStatusSkill() *skills.Skill {
	return &skills.Skill{
		Name:        "builtin:caddy_status",
		Description: "检查 Caddy 服务状态和配置",
		Version:     "1.0.0",
		Author:      "ServerOwl",
		Tags:        []string{"caddy", "web", "status", "builtin"},
		Parameters: []skills.SkillParameter{
			{
				Name:        "show_config",
				Type:        "bool",
				Description: "显示当前配置",
				Required:    false,
				Default:     false,
			},
		},
		Timeout:     1 * time.Minute,
		NeedConfirm: false,
		Steps: []skills.SkillStep{
			{
				Name: "service_status",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `echo "=== Caddy 服务状态 ===" && systemctl status caddy --no-pager 2>/dev/null || echo "Caddy 服务未安装或未运行"`,
				},
				OnError: "continue",
			},
			{
				Name: "version",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `echo "=== Caddy 版本 ===" && caddy version 2>/dev/null || echo "Caddy 未安装"`,
				},
				OnError: "continue",
			},
			{
				Name:      "show_config",
				Tool:      "local_shell",
				Condition: "{{.show_config}} == true",
				Args: map[string]any{
					"command": `echo "=== Caddyfile 配置 ===" && cat /etc/caddy/Caddyfile 2>/dev/null || echo "配置文件不存在"`,
				},
				OnError: "continue",
			},
			{
				Name: "listening_ports",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `echo "=== Caddy 监听端口 ===" && ss -tlnp | grep caddy || echo "未检测到 Caddy 监听端口"`,
				},
				OnError: "continue",
			},
		},
	}
}

// CaddyLogsSkill 返回 Caddy 日志查看技能
func CaddyLogsSkill() *skills.Skill {
	return &skills.Skill{
		Name:        "builtin:caddy_logs",
		Description: "查看 Caddy 访问日志和错误日志",
		Version:     "1.0.0",
		Author:      "ServerOwl",
		Tags:        []string{"caddy", "web", "logs", "builtin"},
		Parameters: []skills.SkillParameter{
			{
				Name:        "lines",
				Type:        "int",
				Description: "显示行数",
				Required:    false,
				Default:     50,
			},
			{
				Name:        "type",
				Type:        "string",
				Description: "日志类型: access, error, all",
				Required:    false,
				Default:     "all",
			},
		},
		Timeout:     1 * time.Minute,
		NeedConfirm: false,
		Steps: []skills.SkillStep{
			{
				Name: "journald_logs",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `echo "=== Caddy 系统日志 ===" && journalctl -u caddy --no-pager -n {{.lines}} 2>/dev/null || echo "无法读取 journald 日志"`,
				},
				OnError: "continue",
			},
			{
				Name:      "access_logs",
				Tool:      "local_shell",
				Condition: "{{.type}} == access || {{.type}} == all",
				Args: map[string]any{
					"command": `echo "=== Caddy 访问日志 ===" && tail -n {{.lines}} /var/log/caddy/access.log 2>/dev/null || echo "访问日志文件不存在"`,
				},
				OnError: "continue",
			},
			{
				Name:      "error_logs",
				Tool:      "local_shell",
				Condition: "{{.type}} == error || {{.type}} == all",
				Args: map[string]any{
					"command": `echo "=== Caddy 错误日志 ===" && tail -n {{.lines}} /var/log/caddy/error.log 2>/dev/null || echo "错误日志文件不存在"`,
				},
				OnError: "continue",
			},
		},
	}
}

// NginxStatusSkill 返回 Nginx 状态检查技能
func NginxStatusSkill() *skills.Skill {
	return &skills.Skill{
		Name:        "builtin:nginx_status",
		Description: "检查 Nginx 服务状态和配置",
		Version:     "1.0.0",
		Author:      "ServerOwl",
		Tags:        []string{"nginx", "web", "status", "builtin"},
		Parameters: []skills.SkillParameter{
			{
				Name:        "show_config",
				Type:        "bool",
				Description: "显示主配置文件",
				Required:    false,
				Default:     false,
			},
		},
		Timeout:     1 * time.Minute,
		NeedConfirm: false,
		Steps: []skills.SkillStep{
			{
				Name: "service_status",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `echo "=== Nginx 服务状态 ===" && systemctl status nginx --no-pager 2>/dev/null || echo "Nginx 服务未安装或未运行"`,
				},
				OnError: "continue",
			},
			{
				Name: "version",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `echo "=== Nginx 版本 ===" && nginx -v 2>&1`,
				},
				OnError: "continue",
			},
			{
				Name: "test_config",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `echo "=== 配置测试 ===" && nginx -t 2>&1`,
				},
				OnError: "continue",
			},
			{
				Name:      "show_config",
				Tool:      "local_shell",
				Condition: "{{.show_config}} == true",
				Args: map[string]any{
					"command": `echo "=== Nginx 主配置 ===" && cat /etc/nginx/nginx.conf 2>/dev/null | head -100`,
				},
				OnError: "continue",
			},
			{
				Name: "listening_ports",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `echo "=== Nginx 监听端口 ===" && ss -tlnp | grep nginx || echo "未检测到 Nginx 监听端口"`,
				},
				OnError: "continue",
			},
			{
				Name: "active_sites",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `echo "=== 已启用站点 ===" && ls -la /etc/nginx/sites-enabled/ 2>/dev/null || ls -la /etc/nginx/conf.d/*.conf 2>/dev/null || echo "未找到站点配置"`,
				},
				OnError: "continue",
			},
		},
	}
}

// NginxLogsSkill 返回 Nginx 日志查看技能
func NginxLogsSkill() *skills.Skill {
	return &skills.Skill{
		Name:        "builtin:nginx_logs",
		Description: "查看 Nginx 访问日志和错误日志",
		Version:     "1.0.0",
		Author:      "ServerOwl",
		Tags:        []string{"nginx", "web", "logs", "builtin"},
		Parameters: []skills.SkillParameter{
			{
				Name:        "lines",
				Type:        "int",
				Description: "显示行数",
				Required:    false,
				Default:     50,
			},
			{
				Name:        "type",
				Type:        "string",
				Description: "日志类型: access, error, all",
				Required:    false,
				Default:     "all",
			},
			{
				Name:        "site",
				Type:        "string",
				Description: "站点名称（留空查看主日志）",
				Required:    false,
				Default:     "",
			},
		},
		Timeout:     1 * time.Minute,
		NeedConfirm: false,
		Steps: []skills.SkillStep{
			{
				Name:      "access_logs",
				Tool:      "local_shell",
				Condition: "{{.type}} == access || {{.type}} == all",
				Args: map[string]any{
					"command": `echo "=== Nginx 访问日志 ===" && if [ "{{.site}}" != "" ]; then tail -n {{.lines}} /var/log/nginx/{{.site}}.access.log 2>/dev/null || tail -n {{.lines}} /var/log/nginx/{{.site}}_access.log 2>/dev/null; else tail -n {{.lines}} /var/log/nginx/access.log 2>/dev/null; fi || echo "访问日志不存在"`,
				},
				OnError: "continue",
			},
			{
				Name:      "error_logs",
				Tool:      "local_shell",
				Condition: "{{.type}} == error || {{.type}} == all",
				Args: map[string]any{
					"command": `echo "=== Nginx 错误日志 ===" && if [ "{{.site}}" != "" ]; then tail -n {{.lines}} /var/log/nginx/{{.site}}.error.log 2>/dev/null || tail -n {{.lines}} /var/log/nginx/{{.site}}_error.log 2>/dev/null; else tail -n {{.lines}} /var/log/nginx/error.log 2>/dev/null; fi || echo "错误日志不存在"`,
				},
				OnError: "continue",
			},
		},
	}
}
