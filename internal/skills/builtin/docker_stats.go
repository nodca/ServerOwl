package builtin

import (
	"time"

	"serverowl/internal/skills"
)

// DockerStatsSkill 返回 Docker 容器资源统计技能
func DockerStatsSkill() *skills.Skill {
	return &skills.Skill{
		Name:        "builtin:docker_stats",
		Description: "Docker 容器资源统计（CPU、内存、网络、磁盘）",
		Version:     "1.0.0",
		Author:      "ServerOwl",
		Tags:        []string{"docker", "stats", "monitoring", "builtin"},
		Parameters: []skills.SkillParameter{
			{
				Name:        "container",
				Type:        "string",
				Description: "容器名称（留空显示所有）",
				Required:    false,
				Default:     "",
			},
			{
				Name:        "format",
				Type:        "string",
				Description: "输出格式: table, json",
				Required:    false,
				Default:     "table",
			},
		},
		Timeout:     2 * time.Minute,
		NeedConfirm: false,
		Steps: []skills.SkillStep{
			{
				Name:      "stats_all",
				Tool:      "local_shell",
				Condition: "{{.container}} == ",
				Args: map[string]any{
					"command": `echo "=== Docker 容器资源统计 ===" && docker stats --no-stream --format "table {{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}\t{{.NetIO}}\t{{.BlockIO}}"`,
				},
				OnError: "continue",
			},
			{
				Name:      "stats_single",
				Tool:      "local_shell",
				Condition: "{{.container}} != ",
				Args: map[string]any{
					"command": `echo "=== 容器 {{.container}} 资源统计 ===" && docker stats {{.container}} --no-stream --format "table {{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}\t{{.NetIO}}\t{{.BlockIO}}"`,
				},
				OnError: "stop",
			},
			{
				Name: "container_top",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `echo "=== 资源占用 Top 5 ===" && docker stats --no-stream --format "{{.Name}}: CPU {{.CPUPerc}}, Mem {{.MemPerc}}" | sort -t: -k2 -rn | head -5`,
				},
				OnError: "continue",
			},
			{
				Name: "disk_usage",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `echo "=== Docker 磁盘使用 ===" && docker system df`,
				},
				OnError: "continue",
			},
			{
				Name: "running_count",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `echo "=== 容器状态统计 ===" && echo "运行中: $(docker ps -q | wc -l)" && echo "已停止: $(docker ps -aq --filter 'status=exited' | wc -l)" && echo "总计: $(docker ps -aq | wc -l)"`,
				},
				OnError: "continue",
			},
		},
	}
}

// DockerInspectSkill 返回 Docker 容器详情技能
func DockerInspectSkill() *skills.Skill {
	return &skills.Skill{
		Name:        "builtin:docker_inspect",
		Description: "查看 Docker 容器详细信息",
		Version:     "1.0.0",
		Author:      "ServerOwl",
		Tags:        []string{"docker", "inspect", "info", "builtin"},
		Parameters: []skills.SkillParameter{
			{
				Name:        "container",
				Type:        "string",
				Description: "容器名称或 ID",
				Required:    true,
			},
		},
		Timeout:     1 * time.Minute,
		NeedConfirm: false,
		Steps: []skills.SkillStep{
			{
				Name: "basic_info",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `echo "=== 容器基本信息 ===" && docker inspect {{.container}} --format "名称: {{.Name}}
ID: {{.Id}}
镜像: {{.Config.Image}}
状态: {{.State.Status}}
启动时间: {{.State.StartedAt}}
重启次数: {{.RestartCount}}"`,
				},
				OnError: "stop",
			},
			{
				Name: "network_info",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `echo "=== 网络配置 ===" && docker inspect {{.container}} --format "{{range .NetworkSettings.Networks}}网络: {{.NetworkID}}
IP: {{.IPAddress}}
网关: {{.Gateway}}
{{end}}"`,
				},
				OnError: "continue",
			},
			{
				Name: "port_mappings",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `echo "=== 端口映射 ===" && docker port {{.container}} 2>/dev/null || echo "无端口映射"`,
				},
				OnError: "continue",
			},
			{
				Name: "volume_mounts",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `echo "=== 挂载卷 ===" && docker inspect {{.container}} --format "{{range .Mounts}}{{.Type}}: {{.Source}} -> {{.Destination}}
{{end}}"`,
				},
				OnError: "continue",
			},
			{
				Name: "env_vars",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `echo "=== 环境变量 ===" && docker inspect {{.container}} --format "{{range .Config.Env}}{{.}}
{{end}}" | grep -v -E "(PASSWORD|SECRET|KEY|TOKEN)" | head -20`,
				},
				OnError: "continue",
			},
		},
	}
}

// DockerLogsAnalyzeSkill 返回 Docker 日志分析技能
func DockerLogsAnalyzeSkill() *skills.Skill {
	return &skills.Skill{
		Name:        "builtin:docker_logs_analyze",
		Description: "分析 Docker 容器日志中的错误和警告",
		Version:     "1.0.0",
		Author:      "ServerOwl",
		Tags:        []string{"docker", "logs", "analyze", "builtin"},
		Parameters: []skills.SkillParameter{
			{
				Name:        "container",
				Type:        "string",
				Description: "容器名称",
				Required:    true,
			},
			{
				Name:        "since",
				Type:        "string",
				Description: "时间范围: 1h, 24h, 7d",
				Required:    false,
				Default:     "1h",
			},
		},
		Timeout:     3 * time.Minute,
		NeedConfirm: false,
		Steps: []skills.SkillStep{
			{
				Name: "error_count",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `echo "=== 错误统计 ===" && docker logs {{.container}} --since {{.since}} 2>&1 | grep -ci "error\|exception\|fatal\|panic" || echo "0"`,
				},
				OnError: "continue",
			},
			{
				Name: "warning_count",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `echo "=== 警告统计 ===" && docker logs {{.container}} --since {{.since}} 2>&1 | grep -ci "warn\|warning" || echo "0"`,
				},
				OnError: "continue",
			},
			{
				Name: "recent_errors",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `echo "=== 最近错误 ===" && docker logs {{.container}} --since {{.since}} 2>&1 | grep -i "error\|exception\|fatal\|panic" | tail -20`,
				},
				OnError: "continue",
			},
			{
				Name: "recent_warnings",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `echo "=== 最近警告 ===" && docker logs {{.container}} --since {{.since}} 2>&1 | grep -i "warn\|warning" | tail -10`,
				},
				OnError: "continue",
			},
		},
	}
}
