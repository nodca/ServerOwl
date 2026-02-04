package skills

import (
	"time"
)

// Skill 表示一个可执行的技能
type Skill struct {
	Name        string            `yaml:"name"`
	Description string            `yaml:"description"`
	Version     string            `yaml:"version"`
	Author      string            `yaml:"author"`
	Tags        []string          `yaml:"tags"`
	Parameters  []SkillParameter  `yaml:"parameters"`
	Steps       []SkillStep       `yaml:"steps"`
	Timeout     time.Duration     `yaml:"timeout"`
	NeedConfirm bool              `yaml:"need_confirm"`
}

// SkillParameter 技能参数定义
type SkillParameter struct {
	Name        string `yaml:"name"`
	Type        string `yaml:"type"` // string, int, bool
	Description string `yaml:"description"`
	Required    bool   `yaml:"required"`
	Default     any    `yaml:"default"`
}

// SkillStep 技能执行步骤
type SkillStep struct {
	Name      string         `yaml:"name"`
	Tool      string         `yaml:"tool"`      // docker, local_shell, remote_shell
	Action    string         `yaml:"action"`    // for docker tool
	Args      map[string]any `yaml:"args"`
	Condition string         `yaml:"condition"` // 条件表达式
	OnError   string         `yaml:"on_error"`  // continue, stop, retry
	Retries   int            `yaml:"retries"`
}

// SkillResult 技能执行结果
type SkillResult struct {
	SkillName string
	Success   bool
	Steps     []StepResult
	StartTime time.Time
	EndTime   time.Time
	Error     string
}

// StepResult 步骤执行结果
type StepResult struct {
	StepName string
	Tool     string
	Success  bool
	Output   string
	Error    string
	Duration time.Duration
}

// SkillRegistry 技能注册表
type SkillRegistry struct {
	skills map[string]*Skill
}

func NewSkillRegistry() *SkillRegistry {
	return &SkillRegistry{
		skills: make(map[string]*Skill),
	}
}

func (r *SkillRegistry) Register(skill *Skill) {
	r.skills[skill.Name] = skill
}

func (r *SkillRegistry) Get(name string) (*Skill, bool) {
	skill, ok := r.skills[name]
	return skill, ok
}

func (r *SkillRegistry) List() []*Skill {
	skills := make([]*Skill, 0, len(r.skills))
	for _, s := range r.skills {
		skills = append(skills, s)
	}
	return skills
}

func (r *SkillRegistry) ListByTag(tag string) []*Skill {
	var skills []*Skill
	for _, s := range r.skills {
		for _, t := range s.Tags {
			if t == tag {
				skills = append(skills, s)
				break
			}
		}
	}
	return skills
}

func (r *SkillRegistry) Unregister(name string) {
	delete(r.skills, name)
}

func (r *SkillRegistry) Count() int {
	return len(r.skills)
}
