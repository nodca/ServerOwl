package metrics

import (
	"context"
	"time"

	"github.com/shirou/gopsutil/v3/mem"
)

// CollectMemory 采集内存指标
func (c *defaultCollector) CollectMemory(ctx context.Context) (*MemoryMetrics, error) {
	vmStat, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		return nil, err
	}

	metrics := &MemoryMetrics{
		Total:        vmStat.Total,
		Available:    vmStat.Available,
		Used:         vmStat.Used,
		UsagePercent: vmStat.UsedPercent,
		Buffers:      vmStat.Buffers,
		Cached:       vmStat.Cached,
		Timestamp:    time.Now(),
	}

	if swapStat, err := mem.SwapMemoryWithContext(ctx); err == nil && swapStat != nil {
		metrics.SwapTotal = swapStat.Total
		metrics.SwapUsed = swapStat.Used
		metrics.SwapFree = swapStat.Free
	}

	return metrics, nil
}

// GetMemoryUsage 获取内存使用率
func GetMemoryUsage(ctx context.Context) (float64, error) {
	vmStat, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		return 0, err
	}
	return vmStat.UsedPercent, nil
}

// GetMemoryInfo 获取内存详细信息
func GetMemoryInfo(ctx context.Context) (total, available, used uint64, usagePercent float64, err error) {
	vmStat, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	return vmStat.Total, vmStat.Available, vmStat.Used, vmStat.UsedPercent, nil
}
