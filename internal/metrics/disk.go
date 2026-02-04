package metrics

import (
	"context"
	"time"

	"github.com/shirou/gopsutil/v3/disk"
)

// CollectDisk 采集磁盘指标
func (c *defaultCollector) CollectDisk(ctx context.Context, mountPoints []string) (*DiskMetrics, error) {
	metrics := &DiskMetrics{
		Partitions: make([]DiskPartition, 0),
		Timestamp:  time.Now(),
	}

	if len(mountPoints) > 0 {
		for _, mp := range mountPoints {
			if usage, err := disk.UsageWithContext(ctx, mp); err == nil {
				metrics.Partitions = append(metrics.Partitions, DiskPartition{
					Device:       usage.Fstype,
					MountPoint:   usage.Path,
					FSType:       usage.Fstype,
					Total:        usage.Total,
					Used:         usage.Used,
					Free:         usage.Free,
					UsagePercent: usage.UsedPercent,
				})
			}
		}
		return metrics, nil
	}

	partitions, err := disk.PartitionsWithContext(ctx, false)
	if err != nil {
		return nil, err
	}

	for _, p := range partitions {
		if usage, err := disk.UsageWithContext(ctx, p.Mountpoint); err == nil {
			metrics.Partitions = append(metrics.Partitions, DiskPartition{
				Device:       p.Device,
				MountPoint:   p.Mountpoint,
				FSType:       p.Fstype,
				Total:        usage.Total,
				Used:         usage.Used,
				Free:         usage.Free,
				UsagePercent: usage.UsedPercent,
			})
		}
	}

	return metrics, nil
}

// GetDiskUsage 获取指定挂载点的磁盘使用率
func GetDiskUsage(ctx context.Context, mountPoint string) (float64, error) {
	usage, err := disk.UsageWithContext(ctx, mountPoint)
	if err != nil {
		return 0, err
	}
	return usage.UsedPercent, nil
}

// GetDiskInfo 获取指定挂载点的磁盘详细信息
func GetDiskInfo(ctx context.Context, mountPoint string) (total, used, free uint64, usagePercent float64, err error) {
	usage, err := disk.UsageWithContext(ctx, mountPoint)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	return usage.Total, usage.Used, usage.Free, usage.UsedPercent, nil
}

// GetAllDiskUsage 获取所有磁盘分区使用情况
func GetAllDiskUsage(ctx context.Context) ([]DiskPartition, error) {
	metrics, err := Default().CollectDisk(ctx, nil)
	if err != nil {
		return nil, err
	}
	return metrics.Partitions, nil
}
