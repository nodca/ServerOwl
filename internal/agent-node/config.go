package agentnode

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// AgentConfig Agent 配置
type AgentConfig struct {
	Agent     AgentSettings     `yaml:"agent"`
	Masters   []MasterNode      `yaml:"masters"`
	Rules     []AutoRule        `yaml:"rules"`
	Metrics   MetricsSettings   `yaml:"metrics"`
	WebSocket WebSocketSettings `yaml:"websocket"`
}

// AgentSettings Agent 设置
type AgentSettings struct {
	ID                string            `yaml:"id"`
	Name              string            `yaml:"name"`
	Tags              []string          `yaml:"tags"`
	Labels            map[string]string `yaml:"labels"`
	DataDir           string            `yaml:"data_dir"`
	LogLevel          string            `yaml:"log_level"`
	HeartbeatInterval time.Duration     `yaml:"heartbeat_interval"`
	ReconnectInterval time.Duration     `yaml:"reconnect_interval"`
	CommandTimeout    time.Duration     `yaml:"command_timeout"`
}

// MetricsSettings 指标收集设置
type MetricsSettings struct {
	Enabled          bool          `yaml:"enabled"`
	CollectInterval  time.Duration `yaml:"collect_interval"`
	ReportInterval   time.Duration `yaml:"report_interval"`
	IncludeDisk      bool          `yaml:"include_disk"`
	IncludeNetwork   bool          `yaml:"include_network"`
	IncludeProcesses bool          `yaml:"include_processes"`
}

// WebSocketSettings WebSocket 设置
type WebSocketSettings struct {
	Enabled           bool          `yaml:"enabled"`
	PingInterval      time.Duration `yaml:"ping_interval"`
	ReconnectInterval time.Duration `yaml:"reconnect_interval"`
	WriteTimeout      time.Duration `yaml:"write_timeout"`
	ReadTimeout       time.Duration `yaml:"read_timeout"`
}

// DefaultAgentConfig 返回默认配置
func DefaultAgentConfig() *AgentConfig {
	hostname, _ := os.Hostname()
	return &AgentConfig{
		Agent: AgentSettings{
			ID:                "", // 自动生成
			Name:              hostname,
			Tags:              []string{},
			Labels:            map[string]string{},
			DataDir:           "/var/lib/owl-agent",
			LogLevel:          "info",
			HeartbeatInterval: 10 * time.Second,
			ReconnectInterval: 5 * time.Second,
			CommandTimeout:    5 * time.Minute,
		},
		Masters: []MasterNode{},
		Rules:   []AutoRule{},
		Metrics: MetricsSettings{
			Enabled:          true,
			CollectInterval:  10 * time.Second,
			ReportInterval:   30 * time.Second,
			IncludeDisk:      true,
			IncludeNetwork:   false,
			IncludeProcesses: false,
		},
		WebSocket: WebSocketSettings{
			Enabled:           true,
			PingInterval:      30 * time.Second,
			ReconnectInterval: 5 * time.Second,
			WriteTimeout:      10 * time.Second,
			ReadTimeout:       60 * time.Second,
		},
	}
}

// LoadAgentConfig 从文件加载配置
func LoadAgentConfig(path string) (*AgentConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	config := DefaultAgentConfig()
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, err
	}

	return config, nil
}

// Save 保存配置到文件
func (c *AgentConfig) Save(path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// GetActiveMaster 获取当前活跃的主节点
func (c *AgentConfig) GetActiveMaster() *MasterNode {
	for i := range c.Masters {
		if c.Masters[i].IsActive {
			return &c.Masters[i]
		}
	}
	// 返回优先级最高的
	if len(c.Masters) > 0 {
		highest := &c.Masters[0]
		for i := range c.Masters {
			if c.Masters[i].Priority > highest.Priority {
				highest = &c.Masters[i]
			}
		}
		return highest
	}
	return nil
}

// GetEnabledRules 获取启用的规则
func (c *AgentConfig) GetEnabledRules() []AutoRule {
	var rules []AutoRule
	for _, r := range c.Rules {
		if r.Enabled {
			rules = append(rules, r)
		}
	}
	return rules
}
