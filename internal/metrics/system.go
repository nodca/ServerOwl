package metrics

import (
	"context"
	"fmt"
	"time"

	"github.com/shirou/gopsutil/v3/host"
)

// CollectSystem 采集系统信息
func (c *defaultCollector) CollectSystem(ctx context.Context) (*SystemInfo, error) {
	hostInfo, err := host.InfoWithContext(ctx)
	if err != nil {
		return nil, err
	}

	return &SystemInfo{
		Hostname:        hostInfo.Hostname,
		OS:              hostInfo.OS,
		Platform:        hostInfo.Platform,
		PlatformVersion: hostInfo.PlatformVersion,
		KernelVersion:   hostInfo.KernelVersion,
		KernelArch:      hostInfo.KernelArch,
		Uptime:          hostInfo.Uptime,
		BootTime:        hostInfo.BootTime,
		UptimeHuman:     formatUptime(hostInfo.Uptime),
		Timestamp:       time.Now(),
	}, nil
}

// GetUptime 获取系统运行时间（秒）
func GetUptime(ctx context.Context) (uint64, error) {
	return host.UptimeWithContext(ctx)
}

// GetHostname 获取主机名
func GetHostname() (string, error) {
	info, err := host.Info()
	if err != nil {
		return "", err
	}
	return info.Hostname, nil
}

// GetOSInfo 获取操作系统信息
func GetOSInfo(ctx context.Context) (os, platform, platformVersion, kernelVersion string, err error) {
	info, err := host.InfoWithContext(ctx)
	if err != nil {
		return "", "", "", "", err
	}
	return info.OS, info.Platform, info.PlatformVersion, info.KernelVersion, nil
}

func formatUptime(seconds uint64) string {
	days := seconds / 86400
	hours := (seconds % 86400) / 3600
	minutes := (seconds % 3600) / 60

	switch {
	case days > 0:
		return fmt.Sprintf("%d 天 %d 小时 %d 分钟", days, hours, minutes)
	case hours > 0:
		return fmt.Sprintf("%d 小时 %d 分钟", hours, minutes)
	default:
		return fmt.Sprintf("%d 分钟", minutes)
	}
}
