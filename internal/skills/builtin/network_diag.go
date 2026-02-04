package builtin

import (
	"time"

	"serverowl/internal/skills"
)

// NetworkDiagSkill 返回网络诊断技能
func NetworkDiagSkill() *skills.Skill {
	return &skills.Skill{
		Name:        "builtin:network_diag",
		Description: "网络诊断（ping、traceroute、DNS、端口检测）",
		Version:     "1.0.0",
		Author:      "ServerOwl",
		Tags:        []string{"network", "diagnostic", "ping", "dns", "builtin"},
		Parameters: []skills.SkillParameter{
			{
				Name:        "target",
				Type:        "string",
				Description: "目标主机或 IP",
				Required:    true,
			},
			{
				Name:        "check_type",
				Type:        "string",
				Description: "检查类型: all, ping, traceroute, dns, port",
				Required:    false,
				Default:     "all",
			},
			{
				Name:        "port",
				Type:        "int",
				Description: "端口号（port 检查时使用）",
				Required:    false,
				Default:     80,
			},
			{
				Name:        "count",
				Type:        "int",
				Description: "ping 次数",
				Required:    false,
				Default:     4,
			},
		},
		Timeout:     3 * time.Minute,
		NeedConfirm: false,
		Steps: []skills.SkillStep{
			{
				Name:      "ping_test",
				Tool:      "local_shell",
				Condition: "{{.check_type}} == all || {{.check_type}} == ping",
				Args: map[string]any{
					"command": `echo "=== Ping 测试 ===" && ping -c {{.count}} {{.target}} 2>&1 || echo "Ping 失败"`,
				},
				OnError: "continue",
			},
			{
				Name:      "dns_lookup",
				Tool:      "local_shell",
				Condition: "{{.check_type}} == all || {{.check_type}} == dns",
				Args: map[string]any{
					"command": `echo "=== DNS 解析 ===" && nslookup {{.target}} 2>&1 || dig {{.target}} +short 2>&1 || host {{.target}} 2>&1`,
				},
				OnError: "continue",
			},
			{
				Name:      "traceroute_test",
				Tool:      "local_shell",
				Condition: "{{.check_type}} == all || {{.check_type}} == traceroute",
				Args: map[string]any{
					"command": `echo "=== 路由追踪 ===" && traceroute -m 15 {{.target}} 2>&1 || tracepath {{.target}} 2>&1 | head -20`,
				},
				OnError: "continue",
			},
			{
				Name:      "port_test",
				Tool:      "local_shell",
				Condition: "{{.check_type}} == all || {{.check_type}} == port",
				Args: map[string]any{
					"command": `echo "=== 端口检测 {{.target}}:{{.port}} ===" && (nc -zv -w 5 {{.target}} {{.port}} 2>&1 && echo "端口 {{.port}} 开放") || (echo "端口 {{.port}} 关闭或不可达")`,
				},
				OnError: "continue",
			},
			{
				Name:      "curl_test",
				Tool:      "local_shell",
				Condition: "{{.check_type}} == all",
				Args: map[string]any{
					"command": `echo "=== HTTP 连接测试 ===" && curl -sI --connect-timeout 5 "http://{{.target}}" 2>&1 | head -10 || echo "HTTP 连接失败"`,
				},
				OnError: "continue",
			},
		},
	}
}

// PortCheckSkill 返回端口检查技能
func PortCheckSkill() *skills.Skill {
	return &skills.Skill{
		Name:        "builtin:port_check",
		Description: "检查端口占用情况",
		Version:     "1.0.0",
		Author:      "ServerOwl",
		Tags:        []string{"network", "port", "diagnostic", "builtin"},
		Parameters: []skills.SkillParameter{
			{
				Name:        "port",
				Type:        "int",
				Description: "要检查的端口号",
				Required:    false,
				Default:     0,
			},
			{
				Name:        "show_all",
				Type:        "bool",
				Description: "显示所有监听端口",
				Required:    false,
				Default:     false,
			},
		},
		Timeout:     1 * time.Minute,
		NeedConfirm: false,
		Steps: []skills.SkillStep{
			{
				Name:      "check_specific_port",
				Tool:      "local_shell",
				Condition: "{{.port}} > 0",
				Args: map[string]any{
					"command": `echo "=== 端口 {{.port}} 占用情况 ===" && ss -tlnp | grep ":{{.port}} " || echo "端口 {{.port}} 未被占用"`,
				},
				OnError: "continue",
			},
			{
				Name:      "list_all_ports",
				Tool:      "local_shell",
				Condition: "{{.show_all}} == true || {{.port}} == 0",
				Args: map[string]any{
					"command": `echo "=== 所有监听端口 ===" && ss -tlnp | head -30`,
				},
				OnError: "continue",
			},
			{
				Name: "port_process",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `echo "=== 端口与进程对应 ===" && ss -tlnp | awk 'NR>1 {print $4, $6}' | head -20`,
				},
				OnError: "continue",
			},
		},
	}
}

// NetworkStatsSkill 返回网络统计技能
func NetworkStatsSkill() *skills.Skill {
	return &skills.Skill{
		Name:        "builtin:network_stats",
		Description: "网络流量和连接统计",
		Version:     "1.0.0",
		Author:      "ServerOwl",
		Tags:        []string{"network", "stats", "monitoring", "builtin"},
		Parameters:  []skills.SkillParameter{},
		Timeout:     1 * time.Minute,
		NeedConfirm: false,
		Steps: []skills.SkillStep{
			{
				Name: "connection_stats",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `echo "=== 连接状态统计 ===" && ss -s`,
				},
				OnError: "continue",
			},
			{
				Name: "interface_stats",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `echo "=== 网卡流量 ===" && cat /proc/net/dev | awk 'NR>2 {print $1, "RX:", $2, "TX:", $10}'`,
				},
				OnError: "continue",
			},
			{
				Name: "top_connections",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `echo "=== Top 连接 IP ===" && ss -tn | awk 'NR>1 {print $5}' | cut -d: -f1 | sort | uniq -c | sort -rn | head -10`,
				},
				OnError: "continue",
			},
		},
	}
}
