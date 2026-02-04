package builtin

import (
	"time"

	"serverowl/internal/skills"
)

// HealthcheckSkill 返回内置的健康检查技能
func HealthcheckSkill() *skills.Skill {
	return &skills.Skill{
		Name:        "builtin:healthcheck",
		Description: "Comprehensive health check for containers and services",
		Version:     "1.0.0",
		Author:      "ServerOwl",
		Tags:        []string{"health", "monitoring", "builtin"},
		Parameters: []skills.SkillParameter{
			{
				Name:        "target",
				Type:        "string",
				Description: "Target to check (container name, URL, or host)",
				Required:    true,
			},
			{
				Name:        "check_type",
				Type:        "string",
				Description: "Type of check: container, http, tcp, process",
				Required:    false,
				Default:     "container",
			},
			{
				Name:        "port",
				Type:        "int",
				Description: "Port for TCP/HTTP checks",
				Required:    false,
				Default:     80,
			},
			{
				Name:        "timeout",
				Type:        "int",
				Description: "Timeout in seconds",
				Required:    false,
				Default:     10,
			},
			{
				Name:        "expected_status",
				Type:        "int",
				Description: "Expected HTTP status code",
				Required:    false,
				Default:     200,
			},
		},
		Timeout:     5 * time.Minute,
		NeedConfirm: false,
		Steps:       healthcheckSteps(),
	}
}

func healthcheckSteps() []skills.SkillStep {
	return []skills.SkillStep{
		{
			Name:      "container_health",
			Tool:      "docker",
			Action:    "inspect",
			Condition: "{{.check_type}} == container",
			Args: map[string]any{
				"target": "{{.target}}",
				"format": "{{.State.Health.Status}}",
			},
			OnError: "continue",
		},
		{
			Name:      "container_running",
			Tool:      "docker",
			Action:    "inspect",
			Condition: "{{.check_type}} == container",
			Args: map[string]any{
				"target": "{{.target}}",
				"format": "{{.State.Running}}",
			},
			OnError: "stop",
		},
		{
			Name:      "http_check",
			Tool:      "local_shell",
			Condition: "{{.check_type}} == http",
			Args: map[string]any{
				"command": `STATUS=$(curl -s -o /dev/null -w "%{http_code}" --max-time {{.timeout}} "{{.target}}") && [ "$STATUS" -eq {{.expected_status}} ] && echo "HTTP OK: $STATUS" || (echo "HTTP FAIL: $STATUS" && exit 1)`,
			},
			OnError: "stop",
			Retries: 2,
		},
		{
			Name:      "tcp_check",
			Tool:      "local_shell",
			Condition: "{{.check_type}} == tcp",
			Args: map[string]any{
				"command": `nc -z -w {{.timeout}} {{.target}} {{.port}} && echo "TCP OK: {{.target}}:{{.port}}" || (echo "TCP FAIL: {{.target}}:{{.port}}" && exit 1)`,
			},
			OnError: "stop",
			Retries: 2,
		},
		{
			Name:      "process_check",
			Tool:      "local_shell",
			Condition: "{{.check_type}} == process",
			Args: map[string]any{
				"command": `pgrep -f "{{.target}}" > /dev/null && echo "Process OK: {{.target}}" || (echo "Process NOT FOUND: {{.target}}" && exit 1)`,
			},
			OnError: "stop",
		},
	}
}

// SystemHealthSkill 返回系统健康检查技能
func SystemHealthSkill() *skills.Skill {
	return &skills.Skill{
		Name:        "builtin:healthcheck:system",
		Description: "Check system resources (CPU, memory, disk)",
		Version:     "1.0.0",
		Author:      "ServerOwl",
		Tags:        []string{"health", "system", "monitoring", "builtin"},
		Parameters: []skills.SkillParameter{
			{
				Name:        "cpu_threshold",
				Type:        "int",
				Description: "CPU usage threshold percentage",
				Required:    false,
				Default:     80,
			},
			{
				Name:        "memory_threshold",
				Type:        "int",
				Description: "Memory usage threshold percentage",
				Required:    false,
				Default:     80,
			},
			{
				Name:        "disk_threshold",
				Type:        "int",
				Description: "Disk usage threshold percentage",
				Required:    false,
				Default:     85,
			},
			{
				Name:        "disk_path",
				Type:        "string",
				Description: "Disk path to check",
				Required:    false,
				Default:     "/",
			},
		},
		Timeout:     2 * time.Minute,
		NeedConfirm: false,
		Steps: []skills.SkillStep{
			{
				Name: "check_cpu",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `CPU=$(top -bn1 | grep "Cpu(s)" | awk '{print int($2)}') && [ "$CPU" -lt {{.cpu_threshold}} ] && echo "CPU OK: ${CPU}%" || (echo "CPU HIGH: ${CPU}%" && exit 1)`,
				},
				OnError: "continue",
			},
			{
				Name: "check_memory",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `MEM=$(free | grep Mem | awk '{printf "%.0f", $3/$2 * 100}') && [ "$MEM" -lt {{.memory_threshold}} ] && echo "Memory OK: ${MEM}%" || (echo "Memory HIGH: ${MEM}%" && exit 1)`,
				},
				OnError: "continue",
			},
			{
				Name: "check_disk",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `DISK=$(df {{.disk_path}} | tail -1 | awk '{print int($5)}') && [ "$DISK" -lt {{.disk_threshold}} ] && echo "Disk OK: ${DISK}%" || (echo "Disk HIGH: ${DISK}%" && exit 1)`,
				},
				OnError: "continue",
			},
			{
				Name: "check_load",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `LOAD=$(cat /proc/loadavg | awk '{print $1}') && echo "Load Average: $LOAD"`,
				},
				OnError: "continue",
			},
			{
				Name: "summary",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `echo "=== System Health Summary ===" && uptime && free -h | head -2 && df -h {{.disk_path}} | tail -1`,
				},
				OnError: "continue",
			},
		},
	}
}

// DockerHealthSkill 返回 Docker 健康检查技能
func DockerHealthSkill() *skills.Skill {
	return &skills.Skill{
		Name:        "builtin:healthcheck:docker",
		Description: "Check Docker daemon and container health",
		Version:     "1.0.0",
		Author:      "ServerOwl",
		Tags:        []string{"health", "docker", "monitoring", "builtin"},
		Parameters: []skills.SkillParameter{
			{
				Name:        "check_all_containers",
				Type:        "bool",
				Description: "Check all containers including stopped",
				Required:    false,
				Default:     false,
			},
			{
				Name:        "container_filter",
				Type:        "string",
				Description: "Filter containers by name pattern",
				Required:    false,
				Default:     "",
			},
		},
		Timeout:     5 * time.Minute,
		NeedConfirm: false,
		Steps: []skills.SkillStep{
			{
				Name: "docker_info",
				Tool: "local_shell",
				Args: map[string]any{
					"command": "docker info --format '{{.ServerVersion}}' 2>/dev/null && echo 'Docker daemon: OK' || echo 'Docker daemon: FAIL'",
				},
				OnError: "stop",
			},
			{
				Name:   "list_containers",
				Tool:   "docker",
				Action: "ps",
				Args: map[string]any{
					"all":    "{{.check_all_containers}}",
					"filter": "{{.container_filter}}",
				},
				OnError: "continue",
			},
			{
				Name: "unhealthy_containers",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `docker ps --filter "health=unhealthy" --format "{{.Names}}: {{.Status}}" | head -10`,
				},
				OnError: "continue",
			},
			{
				Name: "exited_containers",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `docker ps -a --filter "status=exited" --format "{{.Names}}: exited {{.Status}}" | head -10`,
				},
				OnError: "continue",
			},
			{
				Name: "disk_usage",
				Tool: "local_shell",
				Args: map[string]any{
					"command": "docker system df",
				},
				OnError: "continue",
			},
		},
	}
}

// ServiceHealthSkill 返回服务健康检查技能
func ServiceHealthSkill() *skills.Skill {
	return &skills.Skill{
		Name:        "builtin:healthcheck:service",
		Description: "Check health of a specific service with multiple endpoints",
		Version:     "1.0.0",
		Author:      "ServerOwl",
		Tags:        []string{"health", "service", "monitoring", "builtin"},
		Parameters: []skills.SkillParameter{
			{
				Name:        "container",
				Type:        "string",
				Description: "Container name",
				Required:    true,
			},
			{
				Name:        "health_endpoint",
				Type:        "string",
				Description: "Health check endpoint URL",
				Required:    false,
				Default:     "",
			},
			{
				Name:        "port",
				Type:        "int",
				Description: "Service port",
				Required:    false,
				Default:     8080,
			},
			{
				Name:        "log_lines",
				Type:        "int",
				Description: "Number of recent log lines to show",
				Required:    false,
				Default:     10,
			},
		},
		Timeout:     5 * time.Minute,
		NeedConfirm: false,
		Steps: []skills.SkillStep{
			{
				Name:   "container_status",
				Tool:   "docker",
				Action: "inspect",
				Args: map[string]any{
					"target": "{{.container}}",
					"format": "Status: {{.State.Status}}, Running: {{.State.Running}}",
				},
				OnError: "stop",
			},
			{
				Name:   "container_health",
				Tool:   "docker",
				Action: "inspect",
				Args: map[string]any{
					"target": "{{.container}}",
					"format": "Health: {{.State.Health.Status}}",
				},
				OnError: "continue",
			},
			{
				Name:      "endpoint_check",
				Tool:      "local_shell",
				Condition: "{{.health_endpoint}} != ",
				Args: map[string]any{
					"command": `curl -sf --max-time 10 "{{.health_endpoint}}" && echo "Endpoint OK" || echo "Endpoint FAIL"`,
				},
				OnError: "continue",
				Retries: 2,
			},
			{
				Name:   "recent_logs",
				Tool:   "docker",
				Action: "logs",
				Args: map[string]any{
					"container": "{{.container}}",
					"tail":      "{{.log_lines}}",
				},
				OnError: "continue",
			},
			{
				Name: "resource_usage",
				Tool: "local_shell",
				Args: map[string]any{
					"command": "docker stats {{.container}} --no-stream --format 'CPU: {{.CPUPerc}}, Memory: {{.MemUsage}}'",
				},
				OnError: "continue",
			},
		},
	}
}
