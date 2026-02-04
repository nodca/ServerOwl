package builtin

import (
	"time"

	"serverowl/internal/skills"
)

// ServiceRestartSkill 返回安全重启 systemd 服务技能
func ServiceRestartSkill() *skills.Skill {
	return &skills.Skill{
		Name:        "builtin:service_restart",
		Description: "安全重启 systemd 服务（先检查状态，再重启，最后验证）",
		Version:     "1.0.0",
		Author:      "ServerOwl",
		Tags:        []string{"service", "systemd", "restart", "builtin"},
		Parameters: []skills.SkillParameter{
			{
				Name:        "service",
				Type:        "string",
				Description: "服务名称（如 nginx, docker, mysql）",
				Required:    true,
			},
			{
				Name:        "action",
				Type:        "string",
				Description: "操作类型: restart, reload, stop, start",
				Required:    false,
				Default:     "restart",
			},
			{
				Name:        "wait_seconds",
				Type:        "int",
				Description: "重启后等待秒数",
				Required:    false,
				Default:     3,
			},
		},
		Timeout:     3 * time.Minute,
		NeedConfirm: true,
		Steps: []skills.SkillStep{
			{
				Name: "check_service_exists",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `systemctl list-unit-files | grep -q "{{.service}}" && echo "服务 {{.service}} 存在" || (echo "服务 {{.service}} 不存在" && exit 1)`,
				},
				OnError: "stop",
			},
			{
				Name: "pre_status",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `echo "=== 操作前状态 ===" && systemctl status {{.service}} --no-pager | head -15`,
				},
				OnError: "continue",
			},
			{
				Name: "execute_action",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `echo "执行: systemctl {{.action}} {{.service}}" && systemctl {{.action}} {{.service}}`,
				},
				OnError: "stop",
			},
			{
				Name: "wait_for_service",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `echo "等待 {{.wait_seconds}} 秒..." && sleep {{.wait_seconds}}`,
				},
				OnError: "continue",
			},
			{
				Name: "post_status",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `echo "=== 操作后状态 ===" && systemctl status {{.service}} --no-pager | head -15`,
				},
				OnError: "continue",
			},
			{
				Name: "verify_running",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `systemctl is-active {{.service}} && echo "✅ 服务 {{.service}} 运行正常" || echo "⚠️ 服务 {{.service}} 未运行"`,
				},
				OnError: "continue",
			},
		},
	}
}

// NginxReloadSkill 返回 Nginx 安全重载技能
func NginxReloadSkill() *skills.Skill {
	return &skills.Skill{
		Name:        "builtin:nginx_reload",
		Description: "安全重载 Nginx 配置（先测试配置，再重载）",
		Version:     "1.0.0",
		Author:      "ServerOwl",
		Tags:        []string{"nginx", "reload", "web", "builtin"},
		Parameters:  []skills.SkillParameter{},
		Timeout:     2 * time.Minute,
		NeedConfirm: true,
		Steps: []skills.SkillStep{
			{
				Name: "test_config",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `echo "=== 测试 Nginx 配置 ===" && nginx -t`,
				},
				OnError: "stop",
			},
			{
				Name: "reload_nginx",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `echo "=== 重载 Nginx ===" && systemctl reload nginx`,
				},
				OnError: "stop",
			},
			{
				Name: "verify_status",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `echo "=== 验证状态 ===" && systemctl status nginx --no-pager | head -10`,
				},
				OnError: "continue",
			},
		},
	}
}
