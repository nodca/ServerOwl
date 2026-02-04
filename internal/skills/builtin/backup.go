package builtin

import (
	"time"

	"serverowl/internal/skills"
)

// BackupSkill 返回内置的备份技能
func BackupSkill() *skills.Skill {
	return &skills.Skill{
		Name:        "builtin:backup",
		Description: "Generic backup skill for databases and files",
		Version:     "1.0.0",
		Author:      "ServerOwl",
		Tags:        []string{"backup", "builtin"},
		Parameters: []skills.SkillParameter{
			{
				Name:        "type",
				Type:        "string",
				Description: "Backup type: postgres, mysql, files",
				Required:    true,
			},
			{
				Name:        "source",
				Type:        "string",
				Description: "Source to backup (container name or path)",
				Required:    true,
			},
			{
				Name:        "destination",
				Type:        "string",
				Description: "Destination path for backup",
				Required:    false,
				Default:     "/tmp/backups",
			},
			{
				Name:        "database",
				Type:        "string",
				Description: "Database name (for database backups)",
				Required:    false,
			},
			{
				Name:        "compress",
				Type:        "bool",
				Description: "Compress backup with gzip",
				Required:    false,
				Default:     true,
			},
			{
				Name:        "retention_days",
				Type:        "int",
				Description: "Days to keep old backups",
				Required:    false,
				Default:     7,
			},
		},
		Timeout:     30 * time.Minute,
		NeedConfirm: true,
		Steps:       backupSteps(),
	}
}

func backupSteps() []skills.SkillStep {
	return []skills.SkillStep{
		{
			Name: "prepare_destination",
			Tool: "local_shell",
			Args: map[string]any{
				"command": "mkdir -p {{.destination}}",
			},
			OnError: "stop",
		},
		{
			Name:      "backup_postgres",
			Tool:      "docker",
			Action:    "exec",
			Condition: "{{.type}} == postgres",
			Args: map[string]any{
				"container": "{{.source}}",
				"command":   "pg_dump -U postgres {{.database}}",
			},
			OnError: "stop",
		},
		{
			Name:      "backup_mysql",
			Tool:      "docker",
			Action:    "exec",
			Condition: "{{.type}} == mysql",
			Args: map[string]any{
				"container": "{{.source}}",
				"command":   "mysqldump -u root {{.database}}",
			},
			OnError: "stop",
		},
		{
			Name:      "backup_files",
			Tool:      "local_shell",
			Condition: "{{.type}} == files",
			Args: map[string]any{
				"command": "tar -czf {{.destination}}/files_$(date +%Y%m%d_%H%M%S).tar.gz {{.source}}",
			},
			OnError: "stop",
		},
		{
			Name: "cleanup_old",
			Tool: "local_shell",
			Args: map[string]any{
				"command": "find {{.destination}} -type f -mtime +{{.retention_days}} -delete",
			},
			OnError: "continue",
		},
	}
}

// PostgresBackupSkill 返回 PostgreSQL 专用备份技能
func PostgresBackupSkill() *skills.Skill {
	return &skills.Skill{
		Name:        "builtin:backup:postgres",
		Description: "Backup PostgreSQL database with advanced options",
		Version:     "1.0.0",
		Author:      "ServerOwl",
		Tags:        []string{"backup", "postgres", "database", "builtin"},
		Parameters: []skills.SkillParameter{
			{
				Name:        "container",
				Type:        "string",
				Description: "PostgreSQL container name",
				Required:    true,
			},
			{
				Name:        "database",
				Type:        "string",
				Description: "Database name",
				Required:    true,
			},
			{
				Name:        "user",
				Type:        "string",
				Description: "Database user",
				Required:    false,
				Default:     "postgres",
			},
			{
				Name:        "output_dir",
				Type:        "string",
				Description: "Output directory",
				Required:    false,
				Default:     "/tmp/backups/postgres",
			},
			{
				Name:        "format",
				Type:        "string",
				Description: "Backup format: plain, custom, directory, tar",
				Required:    false,
				Default:     "custom",
			},
			{
				Name:        "compress_level",
				Type:        "int",
				Description: "Compression level (0-9)",
				Required:    false,
				Default:     6,
			},
		},
		Timeout:     60 * time.Minute,
		NeedConfirm: true,
		Steps: []skills.SkillStep{
			{
				Name: "create_output_dir",
				Tool: "local_shell",
				Args: map[string]any{
					"command": "mkdir -p {{.output_dir}}",
				},
				OnError: "stop",
			},
			{
				Name:   "verify_container",
				Tool:   "docker",
				Action: "inspect",
				Args: map[string]any{
					"target": "{{.container}}",
					"format": "{{.State.Running}}",
				},
				OnError: "stop",
			},
			{
				Name:   "run_backup",
				Tool:   "docker",
				Action: "exec",
				Args: map[string]any{
					"container": "{{.container}}",
					"command":   "pg_dump -U {{.user}} -F {{.format}} -Z {{.compress_level}} {{.database}}",
				},
				OnError: "stop",
				Retries: 2,
			},
			{
				Name: "save_backup",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `TIMESTAMP=$(date +%Y%m%d_%H%M%S) && echo '{{index .steps "run_backup"}}' > "{{.output_dir}}/{{.database}}_${TIMESTAMP}.dump"`,
				},
				OnError: "stop",
			},
			{
				Name: "verify_backup",
				Tool: "local_shell",
				Args: map[string]any{
					"command": "ls -la {{.output_dir}}/{{.database}}_*.dump | tail -1",
				},
				OnError: "continue",
			},
		},
	}
}

// MySQLBackupSkill 返回 MySQL 专用备份技能
func MySQLBackupSkill() *skills.Skill {
	return &skills.Skill{
		Name:        "builtin:backup:mysql",
		Description: "Backup MySQL database",
		Version:     "1.0.0",
		Author:      "ServerOwl",
		Tags:        []string{"backup", "mysql", "database", "builtin"},
		Parameters: []skills.SkillParameter{
			{
				Name:        "container",
				Type:        "string",
				Description: "MySQL container name",
				Required:    true,
			},
			{
				Name:        "database",
				Type:        "string",
				Description: "Database name",
				Required:    true,
			},
			{
				Name:        "user",
				Type:        "string",
				Description: "Database user",
				Required:    false,
				Default:     "root",
			},
			{
				Name:        "password_env",
				Type:        "string",
				Description: "Environment variable containing password",
				Required:    false,
				Default:     "MYSQL_ROOT_PASSWORD",
			},
			{
				Name:        "output_dir",
				Type:        "string",
				Description: "Output directory",
				Required:    false,
				Default:     "/tmp/backups/mysql",
			},
		},
		Timeout:     60 * time.Minute,
		NeedConfirm: true,
		Steps: []skills.SkillStep{
			{
				Name: "create_output_dir",
				Tool: "local_shell",
				Args: map[string]any{
					"command": "mkdir -p {{.output_dir}}",
				},
				OnError: "stop",
			},
			{
				Name:   "run_backup",
				Tool:   "docker",
				Action: "exec",
				Args: map[string]any{
					"container": "{{.container}}",
					"command":   "mysqldump -u {{.user}} -p${{.password_env}} {{.database}}",
				},
				OnError: "stop",
				Retries: 2,
			},
			{
				Name: "save_and_compress",
				Tool: "local_shell",
				Args: map[string]any{
					"command": `TIMESTAMP=$(date +%Y%m%d_%H%M%S) && echo '{{index .steps "run_backup"}}' | gzip > "{{.output_dir}}/{{.database}}_${TIMESTAMP}.sql.gz"`,
				},
				OnError: "stop",
			},
		},
	}
}
