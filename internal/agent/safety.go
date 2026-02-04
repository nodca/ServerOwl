package agent

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// 白名单配置
type WhitelistConfig struct {
	LocalShellWhitelist []CommandRule            `yaml:"local_shell_whitelist"`
	DockerExecWhitelist map[string][]CommandRule `yaml:"docker_exec_whitelist"`
	DangerousCommands   []string                 `yaml:"dangerous_commands"`

	// 默认策略：命令不在白名单时是否允许走“用户确认”流程（true=可确认，false=直接拒绝）
	// 用指针区分“未配置”(nil)与“显式配置 false”。
	LocalShellDefaultConfirm *bool `yaml:"local_shell_default_confirm"`
	DockerExecDefaultConfirm *bool `yaml:"docker_exec_default_confirm"`
}

// CommandRule 命令规则
type CommandRule struct {
	Pattern     string `yaml:"pattern"`
	Description string `yaml:"description"`
	Confirm     bool   `yaml:"confirm"`
}

// SafetyChecker 安全检查器
type SafetyChecker struct {
	config *WhitelistConfig
}

// NewSafetyChecker 创建安全检查器
func NewSafetyChecker(configPath string) (*SafetyChecker, error) {
	// 读取配置文件
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read whitelist config: %w", err)
	}

	// 解析 YAML
	var config WhitelistConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse whitelist config: %w", err)
	}

	// 默认值：符合你期望的 “黑名单拒绝 / 白名单直过 / 其他需确认”
	if config.LocalShellDefaultConfirm == nil {
		v := true
		config.LocalShellDefaultConfirm = &v
	}
	if config.DockerExecDefaultConfirm == nil {
		v := true
		config.DockerExecDefaultConfirm = &v
	}

	return &SafetyChecker{config: &config}, nil
}

// 检查是否在黑名单
func (s *SafetyChecker) CheckDangerousCommand(command string) error {
	cmd := normalizeCommand(command)
	for _, dangerous := range s.config.DangerousCommands {
		if strings.Contains(command, dangerous) {
			return fmt.Errorf("危险命令被拒绝: 包含 '%s'", dangerous)
		}
		if cmd != command && strings.Contains(cmd, dangerous) {
			return fmt.Errorf("危险命令被拒绝: 包含 '%s'", dangerous)
		}
	}
	return nil
}

// CheckLocalShell 检查本地 Shell 命令
func (s *SafetyChecker) CheckLocalShell(command string) (needConfirm bool, err error) {
	command = normalizeCommand(command)
	// 1. 先检查黑名单
	if err := s.CheckDangerousCommand(command); err != nil {
		return false, err
	}

	// 2. 检查白名单
	for _, rule := range s.config.LocalShellWhitelist {
		matched, err := regexp.MatchString(rule.Pattern, command)
		if err != nil {
			continue
		}
		if matched {
			return false, nil // 匹配白名单，允许执行
		}
	}

	// 3. 不在白名单中：默认可确认或拒绝
	if s.config.LocalShellDefaultConfirm != nil && *s.config.LocalShellDefaultConfirm {
		return true, nil
	}
	return false, fmt.Errorf("命令不在白名单中: %s", command)
}

// Docker命令
func (s *SafetyChecker) CheckDockerExec(container, command string) (needConfirm bool, err error) {
	command = normalizeCommand(command)
	// 1. 先检查黑名单
	if err := s.CheckDangerousCommand(command); err != nil {
		return false, err
	}
	//检查通用白名单
	if rules, ok := s.config.DockerExecWhitelist["common"]; ok {
		for _, rule := range rules {
			matched, err := regexp.MatchString(rule.Pattern, command)
			if err != nil {
				continue
			}
			if matched {
				return rule.Confirm, nil
			}
		}
	}
	//检查容器特定白名单
	// 尝试匹配容器名（postgres, postgresql, pg 都匹配 postgres 规则）
	containerTypes := []string{container}
	// 添加常见别名
	if strings.Contains(container, "postgres") || strings.Contains(container, "pg") {
		containerTypes = append(containerTypes, "postgres")
	}
	if strings.Contains(container, "redis") {
		containerTypes = append(containerTypes, "redis")
	}
	if strings.Contains(container, "mysql") {
		containerTypes = append(containerTypes, "mysql")
	}
	if strings.Contains(container, "kafka") {
		containerTypes = append(containerTypes, "kafka")
	}

	for _, cType := range containerTypes {
		if rules, ok := s.config.DockerExecWhitelist[cType]; ok {
			for _, rule := range rules {
				matched, err := regexp.MatchString(rule.Pattern, command)
				if err != nil {
					continue
				}
				if matched {
					return rule.Confirm, nil
				}
			}
		}
	}
	// 4. 不在白名单中，拒绝
	if s.config.DockerExecDefaultConfirm != nil && *s.config.DockerExecDefaultConfirm {
		return true, nil
	}
	return false, fmt.Errorf("命令不在白名单中: %s", command)
}

// normalizeCommand 尝试把常见的 shell 包装（sh -c "..."/bash -lc "...") 还原成内部命令，
// 避免 LLM 传入包装串导致无法命中白名单。
func normalizeCommand(command string) string {
	cmd := strings.TrimSpace(command)
	if cmd == "" {
		return cmd
	}
	// sh -c "..." / sh -c '...'
	for _, prefix := range []string{"sh -c", "bash -lc", "bash -c"} {
		if strings.HasPrefix(cmd, prefix) {
			rest := strings.TrimSpace(strings.TrimPrefix(cmd, prefix))
			if len(rest) >= 2 && ((rest[0] == '"' && rest[len(rest)-1] == '"') || (rest[0] == '\'' && rest[len(rest)-1] == '\'')) {
				inner := rest[1 : len(rest)-1]
				return strings.TrimSpace(inner)
			}
			return strings.TrimSpace(rest)
		}
	}
	return cmd
}
