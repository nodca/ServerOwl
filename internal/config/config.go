package config

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server        ServerConfig        `yaml:"server"`
	Monitor       MonitorConfig       `yaml:"monitor"`
	WeChat        WeChatConfig        `yaml:"wechat"`
	LLM           LLMConfig           `yaml:"llm"`
	Memory        MemoryConfig        `yaml:"memory"`
	Agent         AgentConfig         `yaml:"agent"`
	Skills        SkillsConfig        `yaml:"skills"`
	Scheduler     SchedulerConfig     `yaml:"scheduler"`
	Logging       LoggingConfig       `yaml:"logging"`
	Web           WebConfig           `yaml:"web"`
	MCP           MCPConfig           `yaml:"mcp"`
	MasterCluster MasterClusterConfig `yaml:"master_cluster"`
}

type ServerConfig struct {
	Port int `yaml:"port"`
}

type MonitorConfig struct {
	Interval   string      `yaml:"interval"`    //检查间隔，如 "15m"
	Containers []Container `yaml:"containers"`  //容器列表
	HTTPChecks []HTTPCheck `yaml:"http_checks"` //HTTP检查列表
	Databases  Database    `yaml:"databases"`   //数据库连接信息
	Thresholds Threshold   `yaml:"thresholds"`  //磁盘和内存阈值百分比
}

type Container struct {
	ID   string `yaml:"id"`
	Name string `yaml:"name"`
}
type HTTPCheck struct {
	Name    string        `yaml:"name"`
	URL     string        `yaml:"url"`
	Timeout time.Duration `yaml:"timeout"`
}
type Database struct {
	Postgres Postgres `yaml:"postgres"`
	Redis    Redis    `yaml:"redis"`
}
type Postgres struct {
	Dsn     string        `yaml:"dsn"`
	Timeout time.Duration `yaml:"timeout"`
}
type Redis struct {
	Addr     string        `yaml:"addr"`
	Password string        `yaml:"password"`
	Timeout  time.Duration `yaml:"timeout"`
}
type Threshold struct {
	DiskPercent   float64 `yaml:"disk_percent"`
	MemoryPercent float64 `yaml:"memory_percent"`
}

type WeChatConfig struct {
	CorpID         string `yaml:"corp_id"`
	AgentID        int64  `yaml:"agent_id"`
	Secret         string `yaml:"secret"`
	Token          string `yaml:"token"`
	EncodingAESKey string `yaml:"encoding_aes_key"`
	DefaultUser    string `yaml:"default_user"`
}

type LLMConfig struct {
	Provider string `yaml:"provider"`
	ApiKey   string `yaml:"api_key"`
	BaseURL  string `yaml:"base_url"`
	Model    string `yaml:"model"`
	Timeout  time.Duration `yaml:"timeout"`
	Retries  int           `yaml:"retries"`
}

type MemoryConfig struct {
	Postgres  PostgresConfig  `yaml:"postgres"`   // PostgreSQL 配置
	ShortTerm ShortTermConfig `yaml:"short_term"`
	LongTerm  LongTermConfig  `yaml:"long_term"`
	Embedding EmbeddingConfig `yaml:"embedding"`
	Forget    ForgetConfig    `yaml:"forget"`
	Summary   SummaryConfig   `yaml:"summary"`
}

type PostgresConfig struct {
	DSN      string `yaml:"dsn"`
	MaxConns int    `yaml:"max_conns"`
}

type ShortTermConfig struct {
	MaxMessages int    `yaml:"max_messages"`
	MaxTurns    int    `yaml:"max_turns"` // 新增
	MaxAge      string `yaml:"max_age"`   // 如 "30m"
}

type LongTermConfig struct {
	MaxAge      string `yaml:"max_age"`      // 如 "720h" (30天)
	MaxEpisodes int    `yaml:"max_episodes"` // 最大 Episode 数量
}

type EmbeddingConfig struct {
	Provider string `yaml:"provider"` // siliconflow
	APIKey   string `yaml:"api_key"`
	BaseURL  string `yaml:"base_url"`
	Model    string `yaml:"model"`     // BAAI/bge-large-zh-v1.5
	Dimension int   `yaml:"dimension"` // 1024
}

type ForgetConfig struct {
	Schedule  string  `yaml:"schedule"`  // cron 表达式
	MinAge    string  `yaml:"min_age"`   // 最小保留期，如 "168h" (7天)
	Threshold float64 `yaml:"threshold"` // 遗忘阈值，默认 0.25
}

type SummaryConfig struct {
	Retention           string `yaml:"retention"`            // 如 "2160h" (90天)
	DailySchedule       string `yaml:"daily_schedule"`       // cron 表达式
	ConsolidateMinEpisodes int `yaml:"consolidate_min_episodes"` // 触发整合的最小 Episode 数
}

type AgentConfig struct {
	MaxIterations      int    `yaml:"max_iterations"`
	Timeout            string `yaml:"timeout"`
	WhitelistPath      string `yaml:"whitelist_path"`
	LogRetentionDays   int    `yaml:"log_retention_days"`
	LogCleanupSchedule string `yaml:"log_cleanup_schedule"`
}

// SkillsConfig 技能系统配置
type SkillsConfig struct {
	Enabled   bool     `yaml:"enabled"`
	Dirs      []string `yaml:"dirs"`       // 技能目录列表
	AutoLoad  bool     `yaml:"auto_load"`  // 启动时自动加载
}

// SchedulerConfig 调度器配置
type SchedulerConfig struct {
	Enabled bool            `yaml:"enabled"`
	Tasks   []ScheduledTask `yaml:"tasks"` // 预定义任务
}

// ScheduledTask 预定义定时任务
type ScheduledTask struct {
	ID          string            `yaml:"id"`
	Name        string            `yaml:"name"`
	Description string            `yaml:"description"`
	Schedule    string            `yaml:"schedule"`    // cron 表达式
	Type        string            `yaml:"type"`        // skill, command, workflow
	SkillName   string            `yaml:"skill_name"`  // 技能名称
	SkillParams map[string]any    `yaml:"skill_params"`
	Command     string            `yaml:"command"`     // 命令
	Enabled     bool              `yaml:"enabled"`
}

// LoggingConfig 日志配置
type LoggingConfig struct {
	Level      string `yaml:"level"`       // debug, info, warn, error
	Format     string `yaml:"format"`      // json, console
	Output     string `yaml:"output"`      // stdout, stderr, file path
	Sanitize   bool   `yaml:"sanitize"`    // 敏感信息脱敏
}

// WebConfig Web 管理面板配置
type WebConfig struct {
	Enabled   bool           `yaml:"enabled"`
	Port      int            `yaml:"port"`
	StaticDir string         `yaml:"static_dir"`
	Auth      WebAuthConfig  `yaml:"auth"`
	CORS      WebCORSConfig  `yaml:"cors"`
}

// WebAuthConfig Web 认证配置
type WebAuthConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Token    string `yaml:"token"`    // Bearer Token
}

// WebCORSConfig CORS 配置
type WebCORSConfig struct {
	Enabled        bool     `yaml:"enabled"`
	AllowedOrigins []string `yaml:"allowed_origins"`
}

// MCPConfig MCP 协议配置
type MCPConfig struct {
	Enabled   bool   `yaml:"enabled"`
	Transport string `yaml:"transport"`  // stdio, http
	HTTPPort  int    `yaml:"http_port"`
}

// MasterClusterConfig 多主集群配置（Agent 模式）
type MasterClusterConfig struct {
	Enabled      bool                `yaml:"enabled"`
	ID           string              `yaml:"id"`            // 本节点 ID
	Name         string              `yaml:"name"`          // 本节点名称
	Addr         string              `yaml:"addr"`          // 本节点地址 (host:port)
	APIPort      int                 `yaml:"api_port"`      // Agent API 端口
	SyncInterval string              `yaml:"sync_interval"` // 主节点同步间隔
	Peers        []MasterPeerConfig  `yaml:"peers"`         // 对等主节点
}

// MasterPeerConfig 对等主节点配置
type MasterPeerConfig struct {
	ID   string `yaml:"id"`
	Name string `yaml:"name"`
	Addr string `yaml:"addr"` // host:port
}

func Load(path string) (*Config, error) {
	dataBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var config Config
	if err := yaml.Unmarshal(dataBytes, &config); err != nil {
		return nil, err
	}
	if config.Server.Port == 0 {
		config.Server.Port = 9090
	}
	return &config, nil
}
