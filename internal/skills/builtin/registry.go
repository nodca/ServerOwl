package builtin

import (
	"serverowl/internal/skills"
)

// RegisterAll 注册所有内置技能到注册表
func RegisterAll(registry *skills.SkillRegistry) {
	// 备份技能
	registry.Register(BackupSkill())
	registry.Register(PostgresBackupSkill())
	registry.Register(MySQLBackupSkill())

	// 健康检查技能
	registry.Register(HealthcheckSkill())
	registry.Register(SystemHealthSkill())
	registry.Register(DockerHealthSkill())
	registry.Register(ServiceHealthSkill())

	// 服务管理技能
	registry.Register(RestartServiceSkill())
	registry.Register(ScaleServiceSkill())
	registry.Register(RollingRestartSkill())

	// 清理技能
	registry.Register(CleanupSkill())
	registry.Register(DockerCleanupSkill())
	registry.Register(LogCleanupSkill())

	// 系统信息技能
	registry.Register(SystemInfoSkill())

	// 网络诊断技能
	registry.Register(NetworkDiagSkill())
	registry.Register(PortCheckSkill())
	registry.Register(NetworkStatsSkill())

	// 日志查看技能
	registry.Register(LogTailSkill())
	registry.Register(LogSearchSkill())

	// 服务重启技能
	registry.Register(ServiceRestartSkill())
	registry.Register(NginxReloadSkill())

	// SSL 证书技能
	registry.Register(SSLCheckSkill())
	registry.Register(SSLRenewSkill())

	// 防火墙技能
	registry.Register(FirewallStatusSkill())
	registry.Register(FirewallAllowSkill())
	registry.Register(FirewallDenySkill())

	// Docker 统计技能
	registry.Register(DockerStatsSkill())
	registry.Register(DockerInspectSkill())
	registry.Register(DockerLogsAnalyzeSkill())

	// Caddy 技能
	registry.Register(CaddyReloadSkill())
	registry.Register(CaddyStatusSkill())
	registry.Register(CaddyLogsSkill())

	// Nginx 扩展技能
	registry.Register(NginxStatusSkill())
	registry.Register(NginxLogsSkill())
}

// ListBuiltinSkills 返回所有内置技能列表
func ListBuiltinSkills() []*skills.Skill {
	return []*skills.Skill{
		// 备份
		BackupSkill(),
		PostgresBackupSkill(),
		MySQLBackupSkill(),
		// 健康检查
		HealthcheckSkill(),
		SystemHealthSkill(),
		DockerHealthSkill(),
		ServiceHealthSkill(),
		// 服务管理
		RestartServiceSkill(),
		ScaleServiceSkill(),
		RollingRestartSkill(),
		// 清理
		CleanupSkill(),
		DockerCleanupSkill(),
		LogCleanupSkill(),
		// 系统信息
		SystemInfoSkill(),
		// 网络诊断
		NetworkDiagSkill(),
		PortCheckSkill(),
		NetworkStatsSkill(),
		// 日志
		LogTailSkill(),
		LogSearchSkill(),
		// 服务重启
		ServiceRestartSkill(),
		NginxReloadSkill(),
		// SSL
		SSLCheckSkill(),
		SSLRenewSkill(),
		// 防火墙
		FirewallStatusSkill(),
		FirewallAllowSkill(),
		FirewallDenySkill(),
		// Docker
		DockerStatsSkill(),
		DockerInspectSkill(),
		DockerLogsAnalyzeSkill(),
		// Caddy
		CaddyReloadSkill(),
		CaddyStatusSkill(),
		CaddyLogsSkill(),
		// Nginx 扩展
		NginxStatusSkill(),
		NginxLogsSkill(),
	}
}

// GetBuiltinSkill 根据名称获取内置技能
func GetBuiltinSkill(name string) *skills.Skill {
	for _, skill := range ListBuiltinSkills() {
		if skill.Name == name {
			return skill
		}
	}
	return nil
}
