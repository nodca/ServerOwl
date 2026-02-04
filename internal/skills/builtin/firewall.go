package builtin

import (
	"time"

	"serverowl/internal/skills"
)

// FirewallStatusSkill 返回防火墙状态检查技能
func FirewallStatusSkill() *skills.Skill {
	return &skills.Skill{
		Name:        "builtin:firewall_status",
		Description: "检查防火墙状态和规则（支持 ufw、firewalld、iptables）",
		Version:     "1.0.0",
		Author:      "ServerOwl",
		Tags:        []string{"firewall", "security", "network", "builtin"},
		Parameters: []skills.SkillParameter{
			{
				Name:        "show_rules",
				Type:        "bool",
				Description: "显示详细规则",
				Required:    false,
				Default:     true,
			},
		},
		Timeout:     2 * time.Minute,
		NeedConfirm: false,
		Steps: []skills.SkillStep{
			{
				Name: "detect_firewall",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `echo "=== 检测防火墙类型 ===" && (which ufw > /dev/null && echo "检测到: ufw") || (which firewall-cmd > /dev/null && echo "检测到: firewalld") || (which iptables > /dev/null && echo "检测到: iptables") || echo "未检测到防火墙"`,
				},
				OnError: "continue",
			},
			{
				Name: "ufw_status",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `if which ufw > /dev/null 2>&1; then echo "=== UFW 状态 ===" && ufw status verbose 2>/dev/null; fi`,
				},
				OnError: "continue",
			},
			{
				Name: "firewalld_status",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `if which firewall-cmd > /dev/null 2>&1; then echo "=== Firewalld 状态 ===" && firewall-cmd --state 2>/dev/null && echo "默认区域: $(firewall-cmd --get-default-zone)" && echo "活动区域: $(firewall-cmd --get-active-zones | head -5)"; fi`,
				},
				OnError: "continue",
			},
			{
				Name:      "firewalld_rules",
				Tool:      "local_shell",
				Condition: "{{.show_rules}} == true",
				Args: map[string]any{
					"command": `if which firewall-cmd > /dev/null 2>&1; then echo "=== Firewalld 规则 ===" && firewall-cmd --list-all 2>/dev/null; fi`,
				},
				OnError: "continue",
			},
			{
				Name:      "iptables_rules",
				Tool:      "local_shell",
				Condition: "{{.show_rules}} == true",
				Args: map[string]any{
					"command": `echo "=== IPTables 规则 ===" && iptables -L -n --line-numbers 2>/dev/null | head -50`,
				},
				OnError: "continue",
			},
			{
				Name: "open_ports",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `echo "=== 开放端口 ===" && ss -tlnp | awk 'NR>1 {print $4}' | cut -d: -f2 | sort -n | uniq`,
				},
				OnError: "continue",
			},
		},
	}
}

// FirewallAllowSkill 返回防火墙放行端口技能
func FirewallAllowSkill() *skills.Skill {
	return &skills.Skill{
		Name:        "builtin:firewall_allow",
		Description: "防火墙放行端口",
		Version:     "1.0.0",
		Author:      "ServerOwl",
		Tags:        []string{"firewall", "security", "network", "builtin"},
		Parameters: []skills.SkillParameter{
			{
				Name:        "port",
				Type:        "int",
				Description: "端口号",
				Required:    true,
			},
			{
				Name:        "protocol",
				Type:        "string",
				Description: "协议: tcp, udp, both",
				Required:    false,
				Default:     "tcp",
			},
			{
				Name:        "source",
				Type:        "string",
				Description: "来源 IP（留空允许所有）",
				Required:    false,
				Default:     "",
			},
		},
		Timeout:     2 * time.Minute,
		NeedConfirm: true,
		Steps: []skills.SkillStep{
			{
				Name: "ufw_allow",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `if which ufw > /dev/null 2>&1 && ufw status | grep -q "active"; then
						echo "=== UFW 放行端口 {{.port}}/{{.protocol}} ==="
						if [ "{{.source}}" != "" ]; then
							ufw allow from {{.source}} to any port {{.port}} proto {{.protocol}}
						else
							ufw allow {{.port}}/{{.protocol}}
						fi
					fi`,
				},
				OnError: "continue",
			},
			{
				Name: "firewalld_allow",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `if which firewall-cmd > /dev/null 2>&1 && firewall-cmd --state 2>/dev/null | grep -q "running"; then
						echo "=== Firewalld 放行端口 {{.port}}/{{.protocol}} ==="
						firewall-cmd --permanent --add-port={{.port}}/{{.protocol}}
						firewall-cmd --reload
					fi`,
				},
				OnError: "continue",
			},
			{
				Name: "verify",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `echo "=== 验证规则 ===" && (ufw status 2>/dev/null | grep {{.port}} || firewall-cmd --list-ports 2>/dev/null | grep {{.port}} || iptables -L -n | grep {{.port}})`,
				},
				OnError: "continue",
			},
		},
	}
}

// FirewallDenySkill 返回防火墙阻止端口技能
func FirewallDenySkill() *skills.Skill {
	return &skills.Skill{
		Name:        "builtin:firewall_deny",
		Description: "防火墙阻止端口",
		Version:     "1.0.0",
		Author:      "ServerOwl",
		Tags:        []string{"firewall", "security", "network", "builtin"},
		Parameters: []skills.SkillParameter{
			{
				Name:        "port",
				Type:        "int",
				Description: "端口号",
				Required:    true,
			},
			{
				Name:        "protocol",
				Type:        "string",
				Description: "协议: tcp, udp",
				Required:    false,
				Default:     "tcp",
			},
		},
		Timeout:     2 * time.Minute,
		NeedConfirm: true,
		Steps: []skills.SkillStep{
			{
				Name: "ufw_deny",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `if which ufw > /dev/null 2>&1 && ufw status | grep -q "active"; then
						echo "=== UFW 阻止端口 {{.port}}/{{.protocol}} ==="
						ufw deny {{.port}}/{{.protocol}}
					fi`,
				},
				OnError: "continue",
			},
			{
				Name: "firewalld_deny",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `if which firewall-cmd > /dev/null 2>&1 && firewall-cmd --state 2>/dev/null | grep -q "running"; then
						echo "=== Firewalld 移除端口 {{.port}}/{{.protocol}} ==="
						firewall-cmd --permanent --remove-port={{.port}}/{{.protocol}}
						firewall-cmd --reload
					fi`,
				},
				OnError: "continue",
			},
			{
				Name: "verify",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `echo "=== 验证规则 ===" && (ufw status 2>/dev/null || firewall-cmd --list-all 2>/dev/null | head -20)`,
				},
				OnError: "continue",
			},
		},
	}
}
