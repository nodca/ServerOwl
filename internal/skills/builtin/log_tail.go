package builtin

import (
	"time"

	"serverowl/internal/skills"
)

// LogTailSkill 返回查看最近日志技能
func LogTailSkill() *skills.Skill {
	return &skills.Skill{
		Name:        "builtin:log_tail",
		Description: "查看最近日志（支持系统日志、服务日志、文件日志）",
		Version:     "1.0.0",
		Author:      "ServerOwl",
		Tags:        []string{"log", "tail", "monitoring", "builtin"},
		Parameters: []skills.SkillParameter{
			{
				Name:        "source",
				Type:        "string",
				Description: "日志来源: system, service, file, docker",
				Required:    false,
				Default:     "system",
			},
			{
				Name:        "target",
				Type:        "string",
				Description: "目标（服务名、文件路径或容器名）",
				Required:    false,
				Default:     "",
			},
			{
				Name:        "lines",
				Type:        "int",
				Description: "显示行数",
				Required:    false,
				Default:     50,
			},
			{
				Name:        "filter",
				Type:        "string",
				Description: "过滤关键词（grep）",
				Required:    false,
				Default:     "",
			},
			{
				Name:        "level",
				Type:        "string",
				Description: "日志级别过滤: error, warn, info",
				Required:    false,
				Default:     "",
			},
		},
		Timeout:     2 * time.Minute,
		NeedConfirm: false,
		Steps: []skills.SkillStep{
			{
				Name:      "system_log",
				Tool:      "local_shell",
				Condition: "{{.source}} == system",
				Args: map[string]any{
					"command": `journalctl -n {{.lines}} --no-pager 2>/dev/null || tail -n {{.lines}} /var/log/syslog 2>/dev/null || tail -n {{.lines}} /var/log/messages`,
				},
				OnError: "continue",
			},
			{
				Name:      "service_log",
				Tool:      "local_shell",
				Condition: "{{.source}} == service",
				Args: map[string]any{
					"command": `journalctl -u {{.target}} -n {{.lines}} --no-pager`,
				},
				OnError: "stop",
			},
			{
				Name:      "file_log",
				Tool:      "local_shell",
				Condition: "{{.source}} == file",
				Args: map[string]any{
					"command": `tail -n {{.lines}} {{.target}}`,
				},
				OnError: "stop",
			},
			{
				Name:      "docker_log",
				Tool:      "docker",
				Action:    "logs",
				Condition: "{{.source}} == docker",
				Args: map[string]any{
					"container": "{{.target}}",
					"tail":      "{{.lines}}",
				},
				OnError: "stop",
			},
			{
				Name:      "filter_errors",
				Tool:      "local_shell",
				Condition: "{{.level}} == error",
				Args: map[string]any{
					"command": `journalctl -p err -n {{.lines}} --no-pager 2>/dev/null || grep -i "error\|fail\|critical" /var/log/syslog | tail -n {{.lines}}`,
				},
				OnError: "continue",
			},
			{
				Name:      "filter_warnings",
				Tool:      "local_shell",
				Condition: "{{.level}} == warn",
				Args: map[string]any{
					"command": `journalctl -p warning -n {{.lines}} --no-pager 2>/dev/null || grep -i "warn" /var/log/syslog | tail -n {{.lines}}`,
				},
				OnError: "continue",
			},
		},
	}
}

// LogSearchSkill 返回日志搜索技能
func LogSearchSkill() *skills.Skill {
	return &skills.Skill{
		Name:        "builtin:log_search",
		Description: "在日志中搜索关键词",
		Version:     "1.0.0",
		Author:      "ServerOwl",
		Tags:        []string{"log", "search", "grep", "builtin"},
		Parameters: []skills.SkillParameter{
			{
				Name:        "keyword",
				Type:        "string",
				Description: "搜索关键词",
				Required:    true,
			},
			{
				Name:        "path",
				Type:        "string",
				Description: "日志文件路径",
				Required:    false,
				Default:     "/var/log",
			},
			{
				Name:        "max_results",
				Type:        "int",
				Description: "最大结果数",
				Required:    false,
				Default:     100,
			},
			{
				Name:        "time_range",
				Type:        "string",
				Description: "时间范围: 1h, 24h, 7d",
				Required:    false,
				Default:     "24h",
			},
		},
		Timeout:     3 * time.Minute,
		NeedConfirm: false,
		Steps: []skills.SkillStep{
			{
				Name: "search_journalctl",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `journalctl --since "{{.time_range}} ago" --no-pager 2>/dev/null | grep -i "{{.keyword}}" | tail -n {{.max_results}}`,
				},
				OnError: "continue",
			},
			{
				Name: "search_files",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `find {{.path}} -name "*.log" -mtime -1 -exec grep -l -i "{{.keyword}}" {} \; 2>/dev/null | head -20`,
				},
				OnError: "continue",
			},
		},
	}
}
