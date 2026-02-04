package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Loader 负责从文件系统加载技能定义
type Loader struct {
	skillDirs []string
}

// NewLoader 创建新的技能加载器
func NewLoader(dirs ...string) *Loader {
	return &Loader{
		skillDirs: dirs,
	}
}

// LoadAll 从所有配置的目录加载技能
func (l *Loader) LoadAll() ([]*Skill, error) {
	var skills []*Skill

	for _, dir := range l.skillDirs {
		dirSkills, err := l.loadFromDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("load skills from %s: %w", dir, err)
		}
		skills = append(skills, dirSkills...)
	}

	return skills, nil
}

func (l *Loader) loadFromDir(dir string) ([]*Skill, error) {
	var skills []*Skill

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}

		path := filepath.Join(dir, name)
		skill, err := l.LoadFile(path)
		if err != nil {
			return nil, fmt.Errorf("load skill %s: %w", path, err)
		}

		skills = append(skills, skill)
	}

	return skills, nil
}

// LoadFile 从单个文件加载技能
func (l *Loader) LoadFile(path string) (*Skill, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var skill Skill
	if err := yaml.Unmarshal(data, &skill); err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}

	if err := l.validate(&skill); err != nil {
		return nil, fmt.Errorf("validate skill: %w", err)
	}

	return &skill, nil
}

func (l *Loader) validate(skill *Skill) error {
	if skill.Name == "" {
		return fmt.Errorf("skill name is required")
	}

	if len(skill.Steps) == 0 {
		return fmt.Errorf("skill must have at least one step")
	}

	validTools := map[string]bool{
		"docker":       true,
		"local_shell":  true,
		"remote_shell": true,
		"skill":        true,
	}

	for i, step := range skill.Steps {
		if step.Tool == "" {
			return fmt.Errorf("step %d: tool is required", i+1)
		}
		if !validTools[step.Tool] {
			return fmt.Errorf("step %d: invalid tool %q", i+1, step.Tool)
		}
	}

	return nil
}

// Reload 重新加载所有技能到注册表
func (l *Loader) Reload(registry *SkillRegistry) error {
	skills, err := l.LoadAll()
	if err != nil {
		return err
	}

	for _, skill := range skills {
		registry.Register(skill)
	}

	return nil
}

// AddDir 添加技能目录
func (l *Loader) AddDir(dir string) {
	l.skillDirs = append(l.skillDirs, dir)
}

// Dirs 返回所有技能目录
func (l *Loader) Dirs() []string {
	return l.skillDirs
}
