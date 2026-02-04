package commander

import (
	"fmt"
	"serverowl/internal/config"
	"serverowl/internal/monitor"
	"time"
)

type Executor struct {
	config *config.MonitorConfig
}

func NewExecutor(cfg *config.MonitorConfig) *Executor {
	return &Executor{config: cfg}
}

// 执行意图，返回结果文本
func (e *Executor) Execute(intent *Intent) string {
	switch intent.Type {
	case "status":
		return e.executeStatus()
	case "restart":
		return e.executeRestart(intent.Target)
	case "logs":
		return e.executeLogs(intent.Target)
	case "help":
		return e.executeHelp()
	default:
		return "抱歉，我没理解你的意思。试试：\n- 看看状态\n- 重启 容器名\n- 查看 容器名 的日志"
	}
}

// 查找容器 ID（提取公共逻辑）
func (e *Executor) findContainerID(target string) string {
	for _, c := range e.config.Containers {
		if c.Name == target || c.ID == target {
			return c.ID
		}
	}
	return ""
}

// 状态查询
func (e *Executor) executeStatus() string {
	var result string
	result += "📊 系统状态\n\n"

	// 容器状态
	result += "【容器】\n"
	for _, c := range e.config.Containers {
		check := monitor.CheckContainer(c.ID)
		if check.Running {
			result += fmt.Sprintf("✅ %s: 运行中\n", c.Name)
		} else {
			result += fmt.Sprintf("❌ %s: %s\n", c.Name, check.Status)
		}
	}

	// HTTP 检查
	if len(e.config.HTTPChecks) > 0 {
		result += "\n【HTTP 检查】\n"
		for _, h := range e.config.HTTPChecks {
			check := monitor.CheckHTTP(h.Name, h.URL, h.Timeout)
			if check.Healthy {
				result += fmt.Sprintf("✅ %s: %dms\n", h.Name, check.Latency.Milliseconds())
			} else {
				result += fmt.Sprintf("❌ %s: %s\n", h.Name, check.Error)
			}
		}
	}

	// 系统资源
	result += "\n【系统资源】\n"
	diskPercent, _, _ := monitor.CheckDisk(100)
	memPercent, _, _ := monitor.CheckMemory(100)
	result += fmt.Sprintf("💾 磁盘: %.1f%%\n", diskPercent)
	result += fmt.Sprintf("🧠 内存: %.1f%%\n", memPercent)

	return result
}

//重启容器
func (e *Executor) executeRestart(target string) string {
	if target == "" {
		return "请指定要重启的容器，例如：重启 nginx"
	}

	// 查找容器 ID
	containerID := e.findContainerID(target)
	if containerID == "" {
		return fmt.Sprintf("未找到容器：%s", target)
	}

	// 执行重启
	err := monitor.RestartContainer(containerID, 30)
	if err != nil {
		return fmt.Sprintf("重启失败：%s", err.Error())
	}

	// 等待容器稳定
	time.Sleep(3 * time.Second)

	// 检查状态
	check := monitor.CheckContainer(containerID)
	if !check.Running {
		return fmt.Sprintf("⚠️ %s 重启后未正常运行\n状态：%s", target, check.Status)
	}

	return fmt.Sprintf("✅ 已重启 %s\n状态：%s", target, check.Status)
}

// 查看日志
func (e *Executor) executeLogs(target string) string {
	if target == "" {
		return "请指定容器，例如：查看 nginx 的日志"
	}

	// 查找容器 ID
	containerID := e.findContainerID(target)
	if containerID == "" {
		return fmt.Sprintf("未找到容器：%s", target)
	}

	// 获取最近 100 行日志（增加行数）
	logs, err := monitor.GetContainerLogs(containerID, 100)
	if err != nil {
		return fmt.Sprintf("获取日志失败：%s", err.Error())
	}

	// 微信消息限制约 4096 字符，保留最后 3500 字符
	if len(logs) > 3500 {
		logs = "...(前面内容已省略)\n\n" + logs[len(logs)-3500:]
	}

	if logs == "" {
		return fmt.Sprintf("📜 %s 暂无日志", target)
	}

	return fmt.Sprintf("📜 %s 最近日志：\n\n%s", target, logs)
}

// 帮助

func (e *Executor) executeHelp() string {
	return `🦉 ServerOwl 运维助手

  我可以帮你：
  - 查看状态：「看看服务状态」
  - 重启容器：「重启 caddy」
  - 查看日志：「caddy 的日志」

  有问题随时问我！`
}
