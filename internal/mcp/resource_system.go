package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"time"

	"serverowl/internal/metrics"
)

// SystemMetricsProvider 系统指标资源提供者
type SystemMetricsProvider struct{}

// NewSystemMetricsProvider 创建系统指标提供者
func NewSystemMetricsProvider() *SystemMetricsProvider {
	return &SystemMetricsProvider{}
}

// ListResources 列出系统指标资源
func (p *SystemMetricsProvider) ListResources() []Resource {
	return []Resource{
		{
			URI:         "system://metrics/cpu",
			Name:        "CPU 使用率",
			Description: "当前 CPU 使用率和负载信息",
			MimeType:    "application/json",
		},
		{
			URI:         "system://metrics/memory",
			Name:        "内存使用",
			Description: "内存使用情况",
			MimeType:    "application/json",
		},
		{
			URI:         "system://metrics/disk",
			Name:        "磁盘使用",
			Description: "磁盘空间使用情况",
			MimeType:    "application/json",
		},
		{
			URI:         "system://metrics/network",
			Name:        "网络状态",
			Description: "网络接口和连接状态",
			MimeType:    "application/json",
		},
		{
			URI:         "system://metrics/overview",
			Name:        "系统概览",
			Description: "系统整体状态概览",
			MimeType:    "application/json",
		},
	}
}

// ReadResource 读取系统指标资源
func (p *SystemMetricsProvider) ReadResource(uri string) (*ResourceContents, error) {
	switch uri {
	case "system://metrics/cpu":
		return p.getCPUMetrics()
	case "system://metrics/memory":
		return p.getMemoryMetrics()
	case "system://metrics/disk":
		return p.getDiskMetrics()
	case "system://metrics/network":
		return p.getNetworkMetrics()
	case "system://metrics/overview":
		return p.getOverview()
	default:
		return nil, fmt.Errorf("未知资源: %s", uri)
	}
}

func (p *SystemMetricsProvider) getCPUMetrics() (*ResourceContents, error) {
	ctx := context.Background()
	cpuMetrics, err := metrics.Default().CollectCPU(ctx)
	if err != nil {
		return nil, err
	}

	result := map[string]any{
		"timestamp":     time.Now().Format(time.RFC3339),
		"cores":         runtime.NumCPU(),
		"usage_percent": cpuMetrics.UsagePercent,
		"load_1m":       cpuMetrics.LoadAvg1,
		"load_5m":       cpuMetrics.LoadAvg5,
		"load_15m":      cpuMetrics.LoadAvg15,
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	return &ResourceContents{
		URI:      "system://metrics/cpu",
		MimeType: "application/json",
		Text:     string(data),
	}, nil
}

func (p *SystemMetricsProvider) getMemoryMetrics() (*ResourceContents, error) {
	ctx := context.Background()
	memMetrics, err := metrics.Default().CollectMemory(ctx)
	if err != nil {
		return nil, err
	}

	result := map[string]any{
		"timestamp":     time.Now().Format(time.RFC3339),
		"total_kb":      memMetrics.Total / 1024,
		"free_kb":       (memMetrics.Total - memMetrics.Used) / 1024,
		"available_kb":  memMetrics.Available / 1024,
		"buffers_kb":    memMetrics.Buffers / 1024,
		"cached_kb":     memMetrics.Cached / 1024,
		"swap_total_kb": memMetrics.SwapTotal / 1024,
		"swap_free_kb":  memMetrics.SwapFree / 1024,
		"usage_percent": memMetrics.UsagePercent,
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	return &ResourceContents{
		URI:      "system://metrics/memory",
		MimeType: "application/json",
		Text:     string(data),
	}, nil
}

func (p *SystemMetricsProvider) getDiskMetrics() (*ResourceContents, error) {
	ctx := context.Background()
	diskMetrics, err := metrics.Default().CollectDisk(ctx, nil)
	if err != nil {
		return nil, err
	}

	disks := make([]map[string]any, 0, len(diskMetrics.Partitions))
	for _, p := range diskMetrics.Partitions {
		disk := map[string]any{
			"filesystem":  p.Device,
			"size":        metrics.FormatBytesSimple(p.Total),
			"used":        metrics.FormatBytesSimple(p.Used),
			"available":   metrics.FormatBytesSimple(p.Free),
			"use_percent": fmt.Sprintf("%.1f%%", p.UsagePercent),
			"mount":       p.MountPoint,
		}
		disks = append(disks, disk)
	}

	result := map[string]any{
		"timestamp": time.Now().Format(time.RFC3339),
		"disks":     disks,
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	return &ResourceContents{
		URI:      "system://metrics/disk",
		MimeType: "application/json",
		Text:     string(data),
	}, nil
}

func (p *SystemMetricsProvider) getNetworkMetrics() (*ResourceContents, error) {
	ctx := context.Background()
	netMetrics, err := metrics.Default().CollectNetwork(ctx)
	if err != nil {
		return nil, err
	}

	interfaces := make([]map[string]any, 0, len(netMetrics.Interfaces))
	for _, iface := range netMetrics.Interfaces {
		interfaces = append(interfaces, map[string]any{
			"name":     iface.Name,
			"rx_bytes": iface.BytesRecv,
			"tx_bytes": iface.BytesSent,
		})
	}

	result := map[string]any{
		"timestamp":  time.Now().Format(time.RFC3339),
		"interfaces": interfaces,
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	return &ResourceContents{
		URI:      "system://metrics/network",
		MimeType: "application/json",
		Text:     string(data),
	}, nil
}

func (p *SystemMetricsProvider) getOverview() (*ResourceContents, error) {
	ctx := context.Background()
	sysInfo, err := metrics.Default().CollectSystem(ctx)
	if err != nil {
		return nil, err
	}

	memMetrics, _ := metrics.Default().CollectMemory(ctx)

	overview := map[string]any{
		"timestamp":      time.Now().Format(time.RFC3339),
		"hostname":       sysInfo.Hostname,
		"os":             sysInfo.Platform + " " + sysInfo.PlatformVersion,
		"cpu_cores":      runtime.NumCPU(),
		"uptime_seconds": sysInfo.Uptime,
		"uptime_human":   sysInfo.UptimeHuman,
	}

	if memMetrics != nil {
		overview["memory_total_mb"] = memMetrics.Total / 1024 / 1024
	}

	data, _ := json.MarshalIndent(overview, "", "  ")
	return &ResourceContents{
		URI:      "system://metrics/overview",
		MimeType: "application/json",
		Text:     string(data),
	}, nil
}
