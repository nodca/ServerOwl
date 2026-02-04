package metrics

import (
	"context"
	"fmt"
)

// 字节单位常量
const (
	KB uint64 = 1024
	MB        = KB * 1024
	GB        = MB * 1024
	TB        = GB * 1024
)

// CheckDiskUsage 检查磁盘使用率是否超过阈值
func CheckDiskUsage(ctx context.Context, mountPoint string, threshold float64) (float64, bool, error) {
	usage, err := GetDiskUsage(ctx, mountPoint)
	if err != nil {
		return -1, false, err
	}
	return usage, usage > threshold, nil
}

// CheckMemoryUsage 检查内存使用率是否超过阈值
func CheckMemoryUsage(ctx context.Context, threshold float64) (float64, bool, error) {
	usage, err := GetMemoryUsage(ctx)
	if err != nil {
		return -1, false, err
	}
	return usage, usage > threshold, nil
}

// CheckCPUUsage 检查 CPU 使用率是否超过阈值
func CheckCPUUsage(ctx context.Context, threshold float64) (float64, bool, error) {
	usage, err := GetCPUUsage(ctx)
	if err != nil {
		return -1, false, err
	}
	return usage, usage > threshold, nil
}

// CheckLoadAvg 检查系统负载是否超过阈值（基于 1 分钟负载）
func CheckLoadAvg(ctx context.Context, threshold float64) (float64, bool, error) {
	load1, _, _, err := GetLoadAvg(ctx)
	if err != nil {
		return -1, false, err
	}
	return load1, load1 > threshold, nil
}

// ThresholdCheck 阈值检查结果
type ThresholdCheck struct {
	Name      string  `json:"name"`
	Current   float64 `json:"current"`
	Threshold float64 `json:"threshold"`
	Exceeded  bool    `json:"exceeded"`
	Error     string  `json:"error,omitempty"`
}

// CheckAllThresholds 检查所有阈值
func CheckAllThresholds(ctx context.Context, cpuThreshold, memThreshold, diskThreshold float64, diskMountPoint string) []ThresholdCheck {
	checks := []struct {
		name      string
		threshold float64
		checkFn   func() (float64, bool, error)
	}{
		{"cpu", cpuThreshold, func() (float64, bool, error) { return CheckCPUUsage(ctx, cpuThreshold) }},
		{"memory", memThreshold, func() (float64, bool, error) { return CheckMemoryUsage(ctx, memThreshold) }},
		{"disk", diskThreshold, func() (float64, bool, error) { return CheckDiskUsage(ctx, diskMountPoint, diskThreshold) }},
	}

	results := make([]ThresholdCheck, 0, len(checks))
	for _, c := range checks {
		usage, exceeded, err := c.checkFn()
		check := ThresholdCheck{
			Name:      c.name,
			Current:   usage,
			Threshold: c.threshold,
			Exceeded:  exceeded,
		}
		if err != nil {
			check.Error = err.Error()
		}
		results = append(results, check)
	}
	return results
}

// FormatBytes 格式化字节数为人类可读格式（保留 2 位小数）
func FormatBytes(bytes uint64) string {
	return formatBytesWithPrecision(bytes, 2)
}

// FormatBytesSimple 简单格式化字节数（保留 1 位小数）
func FormatBytesSimple(bytes uint64) string {
	return formatBytesWithPrecision(bytes, 1)
}

func formatBytesWithPrecision(bytes uint64, precision int) string {
	format := fmt.Sprintf("%%.%df %%s", precision)
	switch {
	case bytes >= TB:
		return fmt.Sprintf(format, float64(bytes)/float64(TB), "TB")
	case bytes >= GB:
		return fmt.Sprintf(format, float64(bytes)/float64(GB), "GB")
	case bytes >= MB:
		return fmt.Sprintf(format, float64(bytes)/float64(MB), "MB")
	case bytes >= KB:
		return fmt.Sprintf(format, float64(bytes)/float64(KB), "KB")
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
