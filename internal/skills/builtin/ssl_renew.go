package builtin

import (
	"time"

	"serverowl/internal/skills"
)

// SSLCheckSkill 返回 SSL 证书检查技能
func SSLCheckSkill() *skills.Skill {
	return &skills.Skill{
		Name:        "builtin:ssl_check",
		Description: "检查 SSL 证书过期时间和状态",
		Version:     "1.0.0",
		Author:      "ServerOwl",
		Tags:        []string{"ssl", "certificate", "security", "builtin"},
		Parameters: []skills.SkillParameter{
			{
				Name:        "domain",
				Type:        "string",
				Description: "域名",
				Required:    true,
			},
			{
				Name:        "port",
				Type:        "int",
				Description: "端口",
				Required:    false,
				Default:     443,
			},
		},
		Timeout:     2 * time.Minute,
		NeedConfirm: false,
		Steps: []skills.SkillStep{
			{
				Name: "check_expiry",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `echo "=== SSL 证书过期时间 ===" && echo | openssl s_client -servername {{.domain}} -connect {{.domain}}:{{.port}} 2>/dev/null | openssl x509 -noout -dates`,
				},
				OnError: "continue",
			},
			{
				Name: "check_issuer",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `echo "=== 证书颁发者 ===" && echo | openssl s_client -servername {{.domain}} -connect {{.domain}}:{{.port}} 2>/dev/null | openssl x509 -noout -issuer`,
				},
				OnError: "continue",
			},
			{
				Name: "check_subject",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `echo "=== 证书主题 ===" && echo | openssl s_client -servername {{.domain}} -connect {{.domain}}:{{.port}} 2>/dev/null | openssl x509 -noout -subject`,
				},
				OnError: "continue",
			},
			{
				Name: "days_remaining",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `echo "=== 剩余天数 ===" && EXPIRY=$(echo | openssl s_client -servername {{.domain}} -connect {{.domain}}:{{.port}} 2>/dev/null | openssl x509 -noout -enddate | cut -d= -f2) && EXPIRY_EPOCH=$(date -d "$EXPIRY" +%s) && NOW_EPOCH=$(date +%s) && DAYS=$(( ($EXPIRY_EPOCH - $NOW_EPOCH) / 86400 )) && echo "剩余 $DAYS 天" && [ $DAYS -gt 30 ] && echo "✅ 证书状态正常" || echo "⚠️ 证书即将过期，请尽快续期"`,
				},
				OnError: "continue",
			},
			{
				Name: "check_chain",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `echo "=== 证书链验证 ===" && echo | openssl s_client -servername {{.domain}} -connect {{.domain}}:{{.port}} 2>/dev/null | grep -E "Verify return code|depth="`,
				},
				OnError: "continue",
			},
		},
	}
}

// SSLRenewSkill 返回 Let's Encrypt 证书续期技能
func SSLRenewSkill() *skills.Skill {
	return &skills.Skill{
		Name:        "builtin:ssl_renew",
		Description: "续期 Let's Encrypt SSL 证书",
		Version:     "1.0.0",
		Author:      "ServerOwl",
		Tags:        []string{"ssl", "letsencrypt", "certbot", "security", "builtin"},
		Parameters: []skills.SkillParameter{
			{
				Name:        "domain",
				Type:        "string",
				Description: "域名（留空则续期所有）",
				Required:    false,
				Default:     "",
			},
			{
				Name:        "dry_run",
				Type:        "bool",
				Description: "模拟运行（不实际续期）",
				Required:    false,
				Default:     true,
			},
			{
				Name:        "reload_nginx",
				Type:        "bool",
				Description: "续期后重载 Nginx",
				Required:    false,
				Default:     true,
			},
		},
		Timeout:     10 * time.Minute,
		NeedConfirm: true,
		Steps: []skills.SkillStep{
			{
				Name: "check_certbot",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `which certbot > /dev/null && echo "certbot 已安装: $(certbot --version)" || (echo "certbot 未安装" && exit 1)`,
				},
				OnError: "stop",
			},
			{
				Name: "list_certificates",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `echo "=== 当前证书列表 ===" && certbot certificates 2>/dev/null | grep -E "Certificate Name|Expiry Date|Domains"`,
				},
				OnError: "continue",
			},
			{
				Name:      "renew_dry_run",
				Tool:      "local_shell",
				Condition: "{{.dry_run}} == true",
				Args: map[string]any{
					"command": `echo "=== 模拟续期 ===" && certbot renew --dry-run`,
				},
				OnError: "continue",
			},
			{
				Name:      "renew_all",
				Tool:      "local_shell",
				Condition: "{{.dry_run}} == false && {{.domain}} == ",
				Args: map[string]any{
					"command": `echo "=== 续期所有证书 ===" && certbot renew`,
				},
				OnError: "stop",
			},
			{
				Name:      "renew_single",
				Tool:      "local_shell",
				Condition: "{{.dry_run}} == false && {{.domain}} != ",
				Args: map[string]any{
					"command": `echo "=== 续期 {{.domain}} ===" && certbot certonly --nginx -d {{.domain}} --non-interactive`,
				},
				OnError: "stop",
			},
			{
				Name:      "reload_nginx",
				Tool:      "local_shell",
				Condition: "{{.reload_nginx}} == true && {{.dry_run}} == false",
				Args: map[string]any{
					"command": `echo "=== 重载 Nginx ===" && nginx -t && systemctl reload nginx`,
				},
				OnError: "continue",
			},
			{
				Name: "verify_renewal",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `echo "=== 验证续期结果 ===" && certbot certificates 2>/dev/null | grep -E "Certificate Name|Expiry Date" | head -10`,
				},
				OnError: "continue",
			},
		},
	}
}
