// Package metrics 提供统一的系统指标采集功能。
// 使用 gopsutil 实现跨平台兼容，支持 Linux、Windows 和 macOS。
//
// 主要功能：
//   - CPU 使用率和负载采集
//   - 内存使用情况采集
//   - 磁盘空间和使用率采集
//   - 网络接口统计采集
//   - 系统信息采集
//   - 进程信息采集
//   - 阈值检查辅助函数
//
// 使用示例：
//
//	// 快速采集基本指标
//	metrics, err := metrics.Quick(ctx)
//
//	// 采集完整指标
//	metrics, err := metrics.Full(ctx)
//
//	// 检查磁盘使用率是否超过阈值
//	usage, exceeded, err := metrics.CheckDiskUsage(ctx, "/", 80.0)
package metrics

import (
	"time"
)

// SystemMetrics 系统指标汇总
type SystemMetrics struct {
	CPU       *CPUMetrics     `json:"cpu,omitempty"`
	Memory    *MemoryMetrics  `json:"memory,omitempty"`
	Disk      *DiskMetrics    `json:"disk,omitempty"`
	Network   *NetworkMetrics `json:"network,omitempty"`
	System    *SystemInfo     `json:"system,omitempty"`
	Processes []ProcessInfo   `json:"processes,omitempty"`
	Timestamp time.Time       `json:"timestamp"`
}

// CPUMetrics CPU 指标
type CPUMetrics struct {
	UsagePercent float64   `json:"usage_percent"`
	Cores        int       `json:"cores"`
	LoadAvg1     float64   `json:"load_avg_1"`
	LoadAvg5     float64   `json:"load_avg_5"`
	LoadAvg15    float64   `json:"load_avg_15"`
	Timestamp    time.Time `json:"timestamp"`
}

// MemoryMetrics 内存指标
type MemoryMetrics struct {
	Total        uint64    `json:"total"`
	Available    uint64    `json:"available"`
	Used         uint64    `json:"used"`
	UsagePercent float64   `json:"usage_percent"`
	SwapTotal    uint64    `json:"swap_total"`
	SwapUsed     uint64    `json:"swap_used"`
	SwapFree     uint64    `json:"swap_free"`
	Buffers      uint64    `json:"buffers"`
	Cached       uint64    `json:"cached"`
	Timestamp    time.Time `json:"timestamp"`
}

// DiskMetrics 磁盘指标
type DiskMetrics struct {
	Partitions []DiskPartition `json:"partitions"`
	Timestamp  time.Time       `json:"timestamp"`
}

// DiskPartition 磁盘分区信息
type DiskPartition struct {
	Device       string  `json:"device"`
	MountPoint   string  `json:"mount_point"`
	FSType       string  `json:"fs_type"`
	Total        uint64  `json:"total"`
	Used         uint64  `json:"used"`
	Free         uint64  `json:"free"`
	UsagePercent float64 `json:"usage_percent"`
}

// NetworkMetrics 网络指标
type NetworkMetrics struct {
	Interfaces []NetworkInterface `json:"interfaces"`
	Timestamp  time.Time          `json:"timestamp"`
}

// NetworkInterface 网络接口信息
type NetworkInterface struct {
	Name        string `json:"name"`
	BytesSent   uint64 `json:"bytes_sent"`
	BytesRecv   uint64 `json:"bytes_recv"`
	PacketsSent uint64 `json:"packets_sent"`
	PacketsRecv uint64 `json:"packets_recv"`
	ErrIn       uint64 `json:"err_in"`
	ErrOut      uint64 `json:"err_out"`
	DropIn      uint64 `json:"drop_in"`
	DropOut     uint64 `json:"drop_out"`
}

// SystemInfo 系统信息
type SystemInfo struct {
	Hostname        string    `json:"hostname"`
	OS              string    `json:"os"`
	Platform        string    `json:"platform"`
	PlatformVersion string    `json:"platform_version"`
	KernelVersion   string    `json:"kernel_version"`
	KernelArch      string    `json:"kernel_arch"`
	Uptime          uint64    `json:"uptime"`
	UptimeHuman     string    `json:"uptime_human"`
	BootTime        uint64    `json:"boot_time"`
	Timestamp       time.Time `json:"timestamp"`
}

// ProcessInfo 进程信息
type ProcessInfo struct {
	PID         int32   `json:"pid"`
	Name        string  `json:"name"`
	Username    string  `json:"username"`
	CPUPercent  float64 `json:"cpu_percent"`
	MemPercent  float32 `json:"mem_percent"`
	MemoryRSS   uint64  `json:"memory_rss"`
	Status      string  `json:"status"`
	CreateTime  int64   `json:"create_time"`
	CommandLine string  `json:"command_line"`
}

// CollectorOption 采集选项
type CollectorOption struct {
	IncludeCPU       bool     `json:"include_cpu"`
	IncludeMemory    bool     `json:"include_memory"`
	IncludeDisk      bool     `json:"include_disk"`
	IncludeNetwork   bool     `json:"include_network"`
	IncludeSystem    bool     `json:"include_system"`
	IncludeProcesses bool     `json:"include_processes"`
	DiskMountPoints  []string `json:"disk_mount_points"`
	TopNProcesses    int      `json:"top_n_processes"`
}

// DefaultOption 返回默认采集选项
func DefaultOption() *CollectorOption {
	return &CollectorOption{
		IncludeCPU:       true,
		IncludeMemory:    true,
		IncludeDisk:      true,
		IncludeNetwork:   false,
		IncludeSystem:    false,
		IncludeProcesses: false,
		DiskMountPoints:  []string{"/"},
		TopNProcesses:    10,
	}
}

// FullOption 返回完整采集选项
func FullOption() *CollectorOption {
	return &CollectorOption{
		IncludeCPU:       true,
		IncludeMemory:    true,
		IncludeDisk:      true,
		IncludeNetwork:   true,
		IncludeSystem:    true,
		IncludeProcesses: true,
		DiskMountPoints:  nil, // nil 表示所有分区
		TopNProcesses:    10,
	}
}
