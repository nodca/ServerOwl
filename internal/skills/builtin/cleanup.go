package builtin

import (
	"time"

	"serverowl/internal/skills"
)

// CleanupSkill 返回通用清理技能
func CleanupSkill() *skills.Skill {
	return &skills.Skill{
		Name:        "builtin:cleanup",
		Description: "General cleanup for temporary files and old data",
		Version:     "1.0.0",
		Author:      "ServerOwl",
		Tags:        []string{"cleanup", "maintenance", "builtin"},
		Parameters: []skills.SkillParameter{
			{
				Name:        "target_dir",
				Type:        "string",
				Description: "Directory to clean",
				Required:    true,
			},
			{
				Name:        "pattern",
				Type:        "string",
				Description: "File pattern to match (e.g., *.log, *.tmp)",
				Required:    false,
				Default:     "*",
			},
			{
				Name:        "older_than_days",
				Type:        "int",
				Description: "Delete files older than N days",
				Required:    false,
				Default:     7,
			},
			{
				Name:        "dry_run",
				Type:        "bool",
				Description: "Show what would be deleted without deleting",
				Required:    false,
				Default:     false,
			},
			{
				Name:        "min_size",
				Type:        "string",
				Description: "Minimum file size to delete (e.g., 1M, 100K)",
				Required:    false,
				Default:     "",
			},
		},
		Timeout:     30 * time.Minute,
		NeedConfirm: true,
		Steps: []skills.SkillStep{
			{
				Name: "check_directory",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `[ -d "{{.target_dir}}" ] && echo "Directory exists: {{.target_dir}}" || (echo "Directory not found: {{.target_dir}}" && exit 1)`,
				},
				OnError: "stop",
			},
			{
				Name: "calculate_size_before",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `du -sh "{{.target_dir}}" 2>/dev/null || echo "Unable to calculate size"`,
				},
				OnError: "continue",
			},
			{
				Name:      "dry_run_list",
				Tool:      "local_shell",
				Condition: "{{.dry_run}}",
				Args: map[string]any{
					"command": `find "{{.target_dir}}" -name "{{.pattern}}" -type f -mtime +{{.older_than_days}} {{if .min_size}}-size +{{.min_size}}{{end}} -print | head -50`,
				},
				OnError: "continue",
			},
			{
				Name:      "delete_files",
				Tool:      "local_shell",
				Condition: "{{.dry_run}} != true",
				Args: map[string]any{
					"command": `find "{{.target_dir}}" -name "{{.pattern}}" -type f -mtime +{{.older_than_days}} {{if .min_size}}-size +{{.min_size}}{{end}} -delete -print | wc -l | xargs -I {} echo "Deleted {} files"`,
				},
				OnError: "continue",
			},
			{
				Name: "calculate_size_after",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `du -sh "{{.target_dir}}" 2>/dev/null || echo "Unable to calculate size"`,
				},
				OnError: "continue",
			},
		},
	}
}

// DockerCleanupSkill 返回 Docker 清理技能
func DockerCleanupSkill() *skills.Skill {
	return &skills.Skill{
		Name:        "builtin:cleanup:docker",
		Description: "Clean up Docker resources (images, containers, volumes, networks)",
		Version:     "1.0.0",
		Author:      "ServerOwl",
		Tags:        []string{"cleanup", "docker", "maintenance", "builtin"},
		Parameters: []skills.SkillParameter{
			{
				Name:        "prune_images",
				Type:        "bool",
				Description: "Remove unused images",
				Required:    false,
				Default:     true,
			},
			{
				Name:        "prune_containers",
				Type:        "bool",
				Description: "Remove stopped containers",
				Required:    false,
				Default:     true,
			},
			{
				Name:        "prune_volumes",
				Type:        "bool",
				Description: "Remove unused volumes",
				Required:    false,
				Default:     false,
			},
			{
				Name:        "prune_networks",
				Type:        "bool",
				Description: "Remove unused networks",
				Required:    false,
				Default:     true,
			},
			{
				Name:        "prune_all",
				Type:        "bool",
				Description: "Remove all unused resources (system prune)",
				Required:    false,
				Default:     false,
			},
			{
				Name:        "older_than",
				Type:        "string",
				Description: "Remove resources older than (e.g., 24h, 7d)",
				Required:    false,
				Default:     "24h",
			},
		},
		Timeout:     30 * time.Minute,
		NeedConfirm: true,
		Steps: []skills.SkillStep{
			{
				Name: "disk_usage_before",
				Tool: "local_shell",
				Args: map[string]any{
					"command": "docker system df",
				},
				OnError: "continue",
			},
			{
				Name:      "prune_all",
				Tool:      "local_shell",
				Condition: "{{.prune_all}}",
				Args: map[string]any{
					"command": "docker system prune -af --filter 'until={{.older_than}}'",
				},
				OnError: "continue",
			},
			{
				Name:      "prune_containers",
				Tool:      "local_shell",
				Condition: "{{.prune_containers}}",
				Args: map[string]any{
					"command": "docker container prune -f --filter 'until={{.older_than}}'",
				},
				OnError: "continue",
			},
			{
				Name:      "prune_images",
				Tool:      "local_shell",
				Condition: "{{.prune_images}}",
				Args: map[string]any{
					"command": "docker image prune -af --filter 'until={{.older_than}}'",
				},
				OnError: "continue",
			},
			{
				Name:      "prune_volumes",
				Tool:      "local_shell",
				Condition: "{{.prune_volumes}}",
				Args: map[string]any{
					"command": "docker volume prune -f",
				},
				OnError: "continue",
			},
			{
				Name:      "prune_networks",
				Tool:      "local_shell",
				Condition: "{{.prune_networks}}",
				Args: map[string]any{
					"command": "docker network prune -f --filter 'until={{.older_than}}'",
				},
				OnError: "continue",
			},
			{
				Name: "disk_usage_after",
				Tool: "local_shell",
				Args: map[string]any{
					"command": "docker system df",
				},
				OnError: "continue",
			},
		},
	}
}

// LogCleanupSkill 返回日志清理技能
func LogCleanupSkill() *skills.Skill {
	return &skills.Skill{
		Name:        "builtin:cleanup:logs",
		Description: "Clean up old log files",
		Version:     "1.0.0",
		Author:      "ServerOwl",
		Tags:        []string{"cleanup", "logs", "maintenance", "builtin"},
		Parameters: []skills.SkillParameter{
			{
				Name:        "log_dirs",
				Type:        "string",
				Description: "Comma-separated list of log directories",
				Required:    false,
				Default:     "/var/log",
			},
			{
				Name:        "older_than_days",
				Type:        "int",
				Description: "Delete logs older than N days",
				Required:    false,
				Default:     30,
			},
			{
				Name:        "patterns",
				Type:        "string",
				Description: "Comma-separated file patterns",
				Required:    false,
				Default:     "*.log,*.log.*,*.gz",
			},
			{
				Name:        "max_size_mb",
				Type:        "int",
				Description: "Truncate logs larger than N MB (0 to disable)",
				Required:    false,
				Default:     0,
			},
			{
				Name:        "compress_older_than_days",
				Type:        "int",
				Description: "Compress logs older than N days (0 to disable)",
				Required:    false,
				Default:     7,
			},
		},
		Timeout:     30 * time.Minute,
		NeedConfirm: true,
		Steps: []skills.SkillStep{
			{
				Name: "calculate_size_before",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `for dir in $(echo "{{.log_dirs}}" | tr ',' ' '); do du -sh "$dir" 2>/dev/null; done`,
				},
				OnError: "continue",
			},
			{
				Name:      "compress_old_logs",
				Tool:      "local_shell",
				Condition: "{{.compress_older_than_days}} != 0",
				Args: map[string]any{
					"command": `for dir in $(echo "{{.log_dirs}}" | tr ',' ' '); do find "$dir" -name "*.log" -type f -mtime +{{.compress_older_than_days}} ! -name "*.gz" -exec gzip {} \; 2>/dev/null; done; echo "Compression complete"`,
				},
				OnError: "continue",
			},
			{
				Name: "delete_old_logs",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `
DELETED=0
for dir in $(echo "{{.log_dirs}}" | tr ',' ' '); do
  for pattern in $(echo "{{.patterns}}" | tr ',' ' '); do
    COUNT=$(find "$dir" -name "$pattern" -type f -mtime +{{.older_than_days}} -delete -print 2>/dev/null | wc -l)
    DELETED=$((DELETED + COUNT))
  done
done
echo "Deleted $DELETED files"
`,
				},
				OnError: "continue",
			},
			{
				Name:      "truncate_large_logs",
				Tool:      "local_shell",
				Condition: "{{.max_size_mb}} != 0",
				Args: map[string]any{
					"command": `
for dir in $(echo "{{.log_dirs}}" | tr ',' ' '); do
  find "$dir" -name "*.log" -type f -size +{{.max_size_mb}}M -exec sh -c 'echo "Truncating: $1"; tail -c $(({{.max_size_mb}} * 1024 * 1024 / 2)) "$1" > "$1.tmp" && mv "$1.tmp" "$1"' _ {} \; 2>/dev/null
done
echo "Truncation complete"
`,
				},
				OnError: "continue",
			},
			{
				Name: "calculate_size_after",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `for dir in $(echo "{{.log_dirs}}" | tr ',' ' '); do du -sh "$dir" 2>/dev/null; done`,
				},
				OnError: "continue",
			},
		},
	}
}
