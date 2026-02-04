package builtin

import (
	"time"

	"serverowl/internal/skills"
)

// RestartServiceSkill 返回服务重启技能
func RestartServiceSkill() *skills.Skill {
	return &skills.Skill{
		Name:        "builtin:restart",
		Description: "Safely restart a Docker container with health verification",
		Version:     "1.0.0",
		Author:      "ServerOwl",
		Tags:        []string{"service", "restart", "docker", "builtin"},
		Parameters: []skills.SkillParameter{
			{
				Name:        "container",
				Type:        "string",
				Description: "Container name to restart",
				Required:    true,
			},
			{
				Name:        "wait_seconds",
				Type:        "int",
				Description: "Seconds to wait after restart",
				Required:    false,
				Default:     10,
			},
			{
				Name:        "verify_health",
				Type:        "bool",
				Description: "Verify container health after restart",
				Required:    false,
				Default:     true,
			},
			{
				Name:        "backup_logs",
				Type:        "bool",
				Description: "Backup logs before restart",
				Required:    false,
				Default:     false,
			},
		},
		Timeout:     10 * time.Minute,
		NeedConfirm: true,
		Steps: []skills.SkillStep{
			{
				Name:   "check_exists",
				Tool:   "docker",
				Action: "inspect",
				Args: map[string]any{
					"target": "{{.container}}",
					"format": "{{.Name}}",
				},
				OnError: "stop",
			},
			{
				Name:      "backup_logs",
				Tool:      "local_shell",
				Condition: "{{.backup_logs}}",
				Args: map[string]any{
					"command": `mkdir -p /tmp/serverowl/logs && docker logs {{.container}} > /tmp/serverowl/logs/{{.container}}_$(date +%Y%m%d_%H%M%S).log 2>&1`,
				},
				OnError: "continue",
			},
			{
				Name:   "restart",
				Tool:   "docker",
				Action: "restart",
				Args: map[string]any{
					"container": "{{.container}}",
				},
				OnError: "stop",
			},
			{
				Name: "wait",
				Tool: "local_shell",
				Args: map[string]any{
					"command": "sleep {{.wait_seconds}}",
				},
				OnError: "continue",
			},
			{
				Name:      "verify_running",
				Tool:      "docker",
				Action:    "inspect",
				Condition: "{{.verify_health}}",
				Args: map[string]any{
					"target": "{{.container}}",
					"format": "{{.State.Running}}",
				},
				OnError: "stop",
				Retries: 3,
			},
			{
				Name:      "verify_health",
				Tool:      "docker",
				Action:    "inspect",
				Condition: "{{.verify_health}}",
				Args: map[string]any{
					"target": "{{.container}}",
					"format": "{{.State.Health.Status}}",
				},
				OnError: "continue",
			},
		},
	}
}

// ScaleServiceSkill 返回服务扩缩容技能
func ScaleServiceSkill() *skills.Skill {
	return &skills.Skill{
		Name:        "builtin:scale",
		Description: "Scale Docker Compose service replicas",
		Version:     "1.0.0",
		Author:      "ServerOwl",
		Tags:        []string{"service", "scale", "docker", "builtin"},
		Parameters: []skills.SkillParameter{
			{
				Name:        "service",
				Type:        "string",
				Description: "Service name to scale",
				Required:    true,
			},
			{
				Name:        "replicas",
				Type:        "int",
				Description: "Number of replicas",
				Required:    true,
			},
			{
				Name:        "compose_file",
				Type:        "string",
				Description: "Path to docker-compose file",
				Required:    false,
				Default:     "docker-compose.yml",
			},
			{
				Name:        "project_dir",
				Type:        "string",
				Description: "Project directory",
				Required:    false,
				Default:     ".",
			},
		},
		Timeout:     10 * time.Minute,
		NeedConfirm: true,
		Steps: []skills.SkillStep{
			{
				Name: "current_state",
				Tool: "local_shell",
				Args: map[string]any{
					"command": "cd {{.project_dir}} && docker compose -f {{.compose_file}} ps {{.service}}",
				},
				OnError: "continue",
			},
			{
				Name: "scale",
				Tool: "local_shell",
				Args: map[string]any{
					"command": "cd {{.project_dir}} && docker compose -f {{.compose_file}} up -d --scale {{.service}}={{.replicas}} --no-recreate",
				},
				OnError: "stop",
			},
			{
				Name: "verify",
				Tool: "local_shell",
				Args: map[string]any{
					"command": "cd {{.project_dir}} && docker compose -f {{.compose_file}} ps {{.service}}",
				},
				OnError: "continue",
			},
		},
	}
}

// RollingRestartSkill 返回滚动重启技能
func RollingRestartSkill() *skills.Skill {
	return &skills.Skill{
		Name:        "builtin:rolling-restart",
		Description: "Perform rolling restart of Docker Compose service",
		Version:     "1.0.0",
		Author:      "ServerOwl",
		Tags:        []string{"service", "restart", "rolling", "docker", "builtin"},
		Parameters: []skills.SkillParameter{
			{
				Name:        "service",
				Type:        "string",
				Description: "Service name",
				Required:    true,
			},
			{
				Name:        "compose_file",
				Type:        "string",
				Description: "Path to docker-compose file",
				Required:    false,
				Default:     "docker-compose.yml",
			},
			{
				Name:        "project_dir",
				Type:        "string",
				Description: "Project directory",
				Required:    false,
				Default:     ".",
			},
			{
				Name:        "delay_seconds",
				Type:        "int",
				Description: "Delay between container restarts",
				Required:    false,
				Default:     10,
			},
			{
				Name:        "health_check_retries",
				Type:        "int",
				Description: "Health check retry count",
				Required:    false,
				Default:     5,
			},
		},
		Timeout:     30 * time.Minute,
		NeedConfirm: true,
		Steps: []skills.SkillStep{
			{
				Name: "get_containers",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `cd {{.project_dir}} && docker compose -f {{.compose_file}} ps -q {{.service}}`,
				},
				OnError: "stop",
			},
			{
				Name: "rolling_restart",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `
cd {{.project_dir}}
CONTAINERS=$(docker compose -f {{.compose_file}} ps -q {{.service}})
for CONTAINER in $CONTAINERS; do
  echo "Restarting $CONTAINER..."
  docker restart $CONTAINER
  echo "Waiting {{.delay_seconds}} seconds..."
  sleep {{.delay_seconds}}

  # Health check
  for i in $(seq 1 {{.health_check_retries}}); do
    STATUS=$(docker inspect --format='{{.State.Running}}' $CONTAINER)
    if [ "$STATUS" = "true" ]; then
      echo "$CONTAINER is running"
      break
    fi
    echo "Waiting for $CONTAINER to be healthy (attempt $i)..."
    sleep 5
  done
done
echo "Rolling restart complete"
`,
				},
				OnError: "stop",
			},
			{
				Name: "final_status",
				Tool: "local_shell",
				Args: map[string]any{
					"command": "cd {{.project_dir}} && docker compose -f {{.compose_file}} ps {{.service}}",
				},
				OnError: "continue",
			},
		},
	}
}
