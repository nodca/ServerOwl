package agentnode

import (
	"context"
	"os/exec"
	"strings"
	"time"

	"serverowl/internal/metrics"
)

// MetricsCollector 指标收集器
type MetricsCollector struct {
	config *MetricsSettings
}

// NewMetricsCollector 创建指标收集器
func NewMetricsCollector(config *MetricsSettings) *MetricsCollector {
	return &MetricsCollector{config: config}
}

// Collect 收集系统指标
func (c *MetricsCollector) Collect() *SystemMetrics {
	ctx := context.Background()
	result := &SystemMetrics{
		Timestamp: time.Now(),
	}

	// 使用统一的 metrics 包采集
	opt := &metrics.CollectorOption{
		IncludeCPU:      true,
		IncludeMemory:   true,
		IncludeDisk:     c.config.IncludeDisk,
		IncludeNetwork:  false,
		IncludeSystem:   true,
		DiskMountPoints: []string{"/"},
	}

	m, err := metrics.Default().Collect(ctx, opt)
	if err != nil {
		return result
	}

	// 转换为 agent-node 的 SystemMetrics 格式
	if m.CPU != nil {
		result.CPUUsage = m.CPU.UsagePercent
		result.LoadAvg1 = m.CPU.LoadAvg1
		result.LoadAvg5 = m.CPU.LoadAvg5
		result.LoadAvg15 = m.CPU.LoadAvg15
	}

	if m.Memory != nil {
		result.MemoryTotal = m.Memory.Total
		result.MemoryFree = m.Memory.Available
		result.MemoryUsage = m.Memory.UsagePercent
	}

	if m.Disk != nil && len(m.Disk.Partitions) > 0 {
		// 使用第一个分区（通常是根分区）
		p := m.Disk.Partitions[0]
		result.DiskTotal = p.Total
		result.DiskFree = p.Free
		result.DiskUsage = p.UsagePercent
	}

	if m.System != nil {
		result.Uptime = int64(m.System.Uptime)
	}

	return result
}

// CheckServiceStatus 检查服务状态（仅 Linux）
func CheckServiceStatus(serviceName string) (bool, error) {
	cmd := exec.Command("systemctl", "is-active", serviceName)
	output, err := cmd.Output()
	status := strings.TrimSpace(string(output))
	return status == "active", err
}

// CheckProcessExists 检查进程是否存在（仅 Linux）
func CheckProcessExists(processName string) (bool, error) {
	cmd := exec.Command("pgrep", "-x", processName)
	err := cmd.Run()
	return err == nil, nil
}
