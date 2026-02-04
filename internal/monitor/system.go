package monitor

import (
	"context"

	"serverowl/internal/metrics"
)

// 检查磁盘使用率，返回使用百分比和是否超过阈值
func CheckDisk(threshold float64) (percent float64, exceeded bool, err error) {
	return metrics.CheckDiskUsage(context.Background(), "/", threshold)
}

// 检查内存使用率，返回使用百分比和是否超过阈值
func CheckMemory(threshold float64) (percent float64, exceeded bool, err error) {
	return metrics.CheckMemoryUsage(context.Background(), threshold)
}
