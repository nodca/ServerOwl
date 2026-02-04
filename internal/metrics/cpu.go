package metrics

import (
	"context"
	"runtime"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/load"
)

// CollectCPU 采集 CPU 指标
func (c *defaultCollector) CollectCPU(ctx context.Context) (*CPUMetrics, error) {
	metrics := &CPUMetrics{
		Cores:     runtime.NumCPU(),
		Timestamp: time.Now(),
	}

	if percentages, err := cpu.PercentWithContext(ctx, 200*time.Millisecond, false); err == nil && len(percentages) > 0 {
		metrics.UsagePercent = percentages[0]
	}

	if loadAvg, err := load.AvgWithContext(ctx); err == nil && loadAvg != nil {
		metrics.LoadAvg1 = loadAvg.Load1
		metrics.LoadAvg5 = loadAvg.Load5
		metrics.LoadAvg15 = loadAvg.Load15
	}

	return metrics, nil
}

// GetCPUUsage 获取当前 CPU 使用率
func GetCPUUsage(ctx context.Context) (float64, error) {
	percentages, err := cpu.PercentWithContext(ctx, 200*time.Millisecond, false)
	if err != nil {
		return 0, err
	}
	if len(percentages) > 0 {
		return percentages[0], nil
	}
	return 0, nil
}

// GetLoadAvg 获取系统负载
func GetLoadAvg(ctx context.Context) (load1, load5, load15 float64, err error) {
	loadAvg, err := load.AvgWithContext(ctx)
	if err != nil {
		return 0, 0, 0, err
	}
	if loadAvg != nil {
		return loadAvg.Load1, loadAvg.Load5, loadAvg.Load15, nil
	}
	return 0, 0, 0, nil
}
