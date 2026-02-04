package monitor

import (
	"fmt"
	"log"
	"serverowl/internal/config"
	"serverowl/internal/notifier"
	"time"

	"github.com/robfig/cron/v3"
)

type Monitor struct {
	config      *config.MonitorConfig
	notifier    *notifier.WeChatNotifier
	state       *AlertState
	defaultUser string
}

type AlertState struct {
	ContainerDown map[string]time.Time
	HTTPFailed    map[string]time.Time
	PostgresFail  bool
	RedisFail     bool
	DiskHigh      bool
	MemoryHigh    bool
}

func NewMonitor(cfg *config.MonitorConfig, notifier *notifier.WeChatNotifier, defaultUser string) *Monitor {
	return &Monitor{
		config:      cfg,
		notifier:    notifier,
		state:       NewAlertState(),
		defaultUser: defaultUser,
	}
}

// 执行一次完整检查
func (m *Monitor) RunChecks() error {
	// 1. 检查容器
	for _, c := range m.config.Containers {
		result := CheckContainer(c.ID)
		if !result.Running {
			if !m.state.HasContainer(c.ID) {
				msg := fmt.Sprintf("🔴 容器异常\n名称：%s\n状态：%s", c.Name, result.Status)
				m.notifier.SendText(m.defaultUser, msg)
				m.state.ContainerDown[c.ID] = time.Now()
			}
		} else {
			if m.state.HasContainer(c.ID) {
				m.notifier.SendText(m.defaultUser, fmt.Sprintf("✅ 容器已恢复\n名称：%s", c.Name))
				delete(m.state.ContainerDown, c.ID)
			}
		}
	}

	// 2. 检查 HTTP
	for _, check := range m.config.HTTPChecks {
		result := CheckHTTP(check.Name, check.URL, check.Timeout)
		if !result.Healthy {
			if !m.state.HasHTTP(check.URL) {
				msg := fmt.Sprintf("🔴 HTTP 检查失败\n服务：%s\nURL：%s\n错误：%s",
					check.Name, check.URL, result.Error)
				if err := m.notifier.SendText(m.defaultUser, msg); err != nil {
					log.Printf("failed to send alert: %v", err)
				}
				m.state.HTTPFailed[check.URL] = time.Now()
			}
		} else {
			if m.state.HasHTTP(check.URL) {
				m.notifier.SendText(m.defaultUser, fmt.Sprintf("✅ HTTP 已恢复\n服务：%s", check.Name))
				delete(m.state.HTTPFailed, check.URL)
			}
		}
	}

	// 3. 检查数据库
	pgResult := CheckPostgres(m.config.Databases.Postgres.Dsn, m.config.Databases.Postgres.Timeout)
	if !pgResult.Healthy {
		if !m.state.PostgresFail {
			msg := fmt.Sprintf("🔴 PostgreSQL 异常\n错误：%s", pgResult.Error)
			m.notifier.SendText(m.defaultUser, msg)
			m.state.PostgresFail = true
		}
	} else if m.state.PostgresFail {
		m.notifier.SendText(m.defaultUser, "✅ PostgreSQL 已恢复")
		m.state.PostgresFail = false
	}

	// 4. 检查 Redis
	redisResult := CheckRedis(m.config.Databases.Redis.Addr, m.config.Databases.Redis.Password, m.config.Databases.Redis.Timeout)
	if !redisResult.Healthy {
		if !m.state.RedisFail {
			msg := fmt.Sprintf("🔴 Redis 异常\n错误：%s", redisResult.Error)
			m.notifier.SendText(m.defaultUser, msg)
			m.state.RedisFail = true
		}
	} else if m.state.RedisFail {
		m.notifier.SendText(m.defaultUser, "✅ Redis 已恢复")
		m.state.RedisFail = false
	}

	// 5. 检查磁盘
	diskPercent, diskHigh, _ := CheckDisk(m.config.Thresholds.DiskPercent)
	if diskHigh {
		if !m.state.DiskHigh {
			msg := fmt.Sprintf("🟡 磁盘使用率过高\n当前：%.1f%%", diskPercent)
			m.notifier.SendText(m.defaultUser, msg)
			m.state.DiskHigh = true
		}
	} else if m.state.DiskHigh {
		m.notifier.SendText(m.defaultUser, "✅ 磁盘使用率已恢复")
		m.state.DiskHigh = false
	}

	// 6. 检查内存
	memPercent, memHigh, _ := CheckMemory(m.config.Thresholds.MemoryPercent)
	if memHigh {
		if !m.state.MemoryHigh {
			msg := fmt.Sprintf("🟡 内存使用率过高\n当前：%.1f%%", memPercent)
			m.notifier.SendText(m.defaultUser, msg)
			m.state.MemoryHigh = true
		}
	} else if m.state.MemoryHigh {
		m.notifier.SendText(m.defaultUser, "✅ 内存使用率已恢复")
		m.state.MemoryHigh = false
	}

	return nil
}

// 启动定时任务
func (m *Monitor) Start() error {
	c := cron.New()
	c.AddFunc("@every "+m.config.Interval, func() {
		m.RunChecks()
	})
	c.Start()
	return nil
}

// 初始化空的告警状态
func NewAlertState() *AlertState {
	return &AlertState{
		ContainerDown: make(map[string]time.Time),
		HTTPFailed:    make(map[string]time.Time),
	}
}

// 检查某个 key 是否已经在告警中（针对 map 类型）
func (s *AlertState) HasContainer(containerID string) bool {
	_, exists := s.ContainerDown[containerID]
	return exists
}

func (s *AlertState) HasHTTP(url string) bool {
	_, exists := s.HTTPFailed[url]
	return exists
}
