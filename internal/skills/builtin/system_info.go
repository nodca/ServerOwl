package builtin

import (
	"time"

	"serverowl/internal/skills"
)

// SystemInfoSkill 返回系统信息汇总技能
func SystemInfoSkill() *skills.Skill {
	return &skills.Skill{
		Name:        "builtin:system_info",
		Description: "获取系统信息汇总（CPU、内存、磁盘、网络、进程）",
		Version:     "1.0.0",
		Author:      "ServerOwl",
		Tags:        []string{"system", "info", "monitoring", "builtin"},
		Parameters: []skills.SkillParameter{
			{
				Name:        "detailed",
				Type:        "bool",
				Description: "是否显示详细信息",
				Required:    false,
				Default:     false,
			},
		},
		Timeout:     2 * time.Minute,
		NeedConfirm: false,
		Steps: []skills.SkillStep{
			{
				Name: "os_info",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `echo "=== 操作系统信息 ===" && cat /etc/os-release 2>/dev/null | grep -E "^(NAME|VERSION)=" || uname -a`,
				},
				OnError: "continue",
			},
			{
				Name: "hostname_info",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `echo "=== 主机信息 ===" && echo "主机名: $(hostname)" && echo "运行时间: $(uptime -p 2>/dev/null || uptime)"`,
				},
				OnError: "continue",
			},
			{
				Name: "cpu_info",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `echo "=== CPU 信息 ===" && echo "核心数: $(nproc)" && echo "型号: $(cat /proc/cpuinfo | grep 'model name' | head -1 | cut -d: -f2 | xargs)" && echo "负载: $(cat /proc/loadavg | awk '{print $1, $2, $3}')"`,
				},
				OnError: "continue",
			},
			{
				Name: "memory_info",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `echo "=== 内存信息 ===" && free -h | head -2 && echo "" && echo "使用率: $(free | grep Mem | awk '{printf "%.1f%%", $3/$2 * 100}')"`,
				},
				OnError: "continue",
			},
			{
				Name: "disk_info",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `echo "=== 磁盘信息 ===" && df -h | grep -E "^/dev|Filesystem" | head -10`,
				},
				OnError: "continue",
			},
			{
				Name: "network_info",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `echo "=== 网络信息 ===" && ip -4 addr show | grep -E "inet |^[0-9]" | head -10 2>/dev/null || ifconfig | grep -E "inet |^[a-z]" | head -10`,
				},
				OnError: "continue",
			},
			{
				Name:      "detailed_processes",
				Tool:      "local_shell",
				Condition: "{{.detailed}} == true",
				Args: map[string]any{
					"command": `echo "=== Top 10 进程 (CPU) ===" && ps aux --sort=-%cpu | head -11`,
				},
				OnError: "continue",
			},
			{
				Name:      "detailed_connections",
				Tool:      "local_shell",
				Condition: "{{.detailed}} == true",
				Args: map[string]any{
					"command": `echo "=== 网络连接统计 ===" && ss -s 2>/dev/null || netstat -s | head -20`,
				},
				OnError: "continue",
			},
		},
	}
}
