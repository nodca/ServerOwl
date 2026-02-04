package skills

import (
	"context"
	"fmt"
	"sync"
)

// Manager 技能管理器，提供统一的技能管理接口
type Manager struct {
	registry *SkillRegistry
	loader   *Loader
	executor *Executor
	mu       sync.RWMutex
}

// NewManager 创建新的技能管理器
func NewManager(skillDirs ...string) *Manager {
	registry := NewSkillRegistry()
	loader := NewLoader(skillDirs...)

	return &Manager{
		registry: registry,
		loader:   loader,
		executor: NewExecutor(registry),
	}
}

// Registry 返回技能注册表
func (m *Manager) Registry() *SkillRegistry {
	return m.registry
}

// Loader 返回技能加载器
func (m *Manager) Loader() *Loader {
	return m.loader
}

// Executor 返回技能执行器
func (m *Manager) Executor() *Executor {
	return m.executor
}

// LoadSkills 从配置的目录加载所有技能
func (m *Manager) LoadSkills() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.loader.Reload(m.registry)
}

// LoadSkillFile 从指定文件加载技能
func (m *Manager) LoadSkillFile(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	skill, err := m.loader.LoadFile(path)
	if err != nil {
		return err
	}

	m.registry.Register(skill)
	return nil
}

// RegisterSkill 注册技能
func (m *Manager) RegisterSkill(skill *Skill) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.registry.Register(skill)
}

// GetSkill 获取技能
func (m *Manager) GetSkill(name string) (*Skill, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.registry.Get(name)
}

// ListSkills 列出所有技能
func (m *Manager) ListSkills() []*Skill {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.registry.List()
}

// ListSkillsByTag 按标签列出技能
func (m *Manager) ListSkillsByTag(tag string) []*Skill {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.registry.ListByTag(tag)
}

// Execute 执行技能
func (m *Manager) Execute(ctx context.Context, skillName string, params map[string]any) (*SkillResult, error) {
	m.mu.RLock()
	skill, ok := m.registry.Get(skillName)
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("skill not found: %s", skillName)
	}

	return m.executor.ExecuteSkill(ctx, skill, params)
}

// ExecuteWithConfirm 执行需要确认的技能
func (m *Manager) ExecuteWithConfirm(ctx context.Context, skillName string, params map[string]any, confirmFn func(*Skill) bool) (*SkillResult, error) {
	m.mu.RLock()
	skill, ok := m.registry.Get(skillName)
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("skill not found: %s", skillName)
	}

	if skill.NeedConfirm && confirmFn != nil {
		if !confirmFn(skill) {
			return &SkillResult{
				SkillName: skillName,
				Success:   false,
				Error:     "execution cancelled by user",
			}, nil
		}
	}

	return m.executor.ExecuteSkill(ctx, skill, params)
}

// ValidateParams 验证技能参数
func (m *Manager) ValidateParams(skillName string, params map[string]any) error {
	m.mu.RLock()
	skill, ok := m.registry.Get(skillName)
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("skill not found: %s", skillName)
	}

	return m.executor.ValidateParams(skill, params)
}

// AddSkillDir 添加技能目录
func (m *Manager) AddSkillDir(dir string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.loader.AddDir(dir)
}

// Reload 重新加载所有技能
func (m *Manager) Reload() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 清空现有技能
	m.registry = NewSkillRegistry()
	m.executor = NewExecutor(m.registry)

	return m.loader.Reload(m.registry)
}

// SkillCount 返回已注册技能数量
func (m *Manager) SkillCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.registry.Count()
}

// SkillInfo 技能信息摘要
type SkillInfo struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Version     string   `json:"version"`
	Tags        []string `json:"tags"`
	NeedConfirm bool     `json:"need_confirm"`
	StepCount   int      `json:"step_count"`
	ParamCount  int      `json:"param_count"`
}

// GetSkillInfo 获取技能信息摘要
func (m *Manager) GetSkillInfo(name string) (*SkillInfo, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	skill, ok := m.registry.Get(name)
	if !ok {
		return nil, false
	}

	return &SkillInfo{
		Name:        skill.Name,
		Description: skill.Description,
		Version:     skill.Version,
		Tags:        skill.Tags,
		NeedConfirm: skill.NeedConfirm,
		StepCount:   len(skill.Steps),
		ParamCount:  len(skill.Parameters),
	}, true
}

// ListSkillInfos 列出所有技能信息摘要
func (m *Manager) ListSkillInfos() []SkillInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	skills := m.registry.List()
	infos := make([]SkillInfo, len(skills))

	for i, skill := range skills {
		infos[i] = SkillInfo{
			Name:        skill.Name,
			Description: skill.Description,
			Version:     skill.Version,
			Tags:        skill.Tags,
			NeedConfirm: skill.NeedConfirm,
			StepCount:   len(skill.Steps),
			ParamCount:  len(skill.Parameters),
		}
	}

	return infos
}
