package metrics

import (
	"context"
	"sync"
	"time"
)

// Collector 指标采集器接口
type Collector interface {
	// Collect 采集所有指标
	Collect(ctx context.Context, opt *CollectorOption) (*SystemMetrics, error)
	// CollectCPU 采集 CPU 指标
	CollectCPU(ctx context.Context) (*CPUMetrics, error)
	// CollectMemory 采集内存指标
	CollectMemory(ctx context.Context) (*MemoryMetrics, error)
	// CollectDisk 采集磁盘指标
	CollectDisk(ctx context.Context, mountPoints []string) (*DiskMetrics, error)
	// CollectNetwork 采集网络指标
	CollectNetwork(ctx context.Context) (*NetworkMetrics, error)
	// CollectSystem 采集系统信息
	CollectSystem(ctx context.Context) (*SystemInfo, error)
	// CollectProcesses 采集进程信息
	CollectProcesses(ctx context.Context, topN int) ([]ProcessInfo, error)
}

// defaultCollector 默认采集器实现
type defaultCollector struct{}

var (
	instance Collector
	once     sync.Once
)

// Default 返回默认采集器单例
func Default() Collector {
	once.Do(func() {
		instance = &defaultCollector{}
	})
	return instance
}

// Collect 采集所有指标
func (c *defaultCollector) Collect(ctx context.Context, opt *CollectorOption) (*SystemMetrics, error) {
	if opt == nil {
		opt = DefaultOption()
	}

	metrics := &SystemMetrics{
		Timestamp: time.Now(),
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	errCh := make(chan error, 6)

	if opt.IncludeCPU {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cpu, err := c.CollectCPU(ctx)
			if err != nil {
				errCh <- err
				return
			}
			mu.Lock()
			metrics.CPU = cpu
			mu.Unlock()
		}()
	}

	if opt.IncludeMemory {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mem, err := c.CollectMemory(ctx)
			if err != nil {
				errCh <- err
				return
			}
			mu.Lock()
			metrics.Memory = mem
			mu.Unlock()
		}()
	}

	if opt.IncludeDisk {
		wg.Add(1)
		go func() {
			defer wg.Done()
			disk, err := c.CollectDisk(ctx, opt.DiskMountPoints)
			if err != nil {
				errCh <- err
				return
			}
			mu.Lock()
			metrics.Disk = disk
			mu.Unlock()
		}()
	}

	if opt.IncludeNetwork {
		wg.Add(1)
		go func() {
			defer wg.Done()
			net, err := c.CollectNetwork(ctx)
			if err != nil {
				errCh <- err
				return
			}
			mu.Lock()
			metrics.Network = net
			mu.Unlock()
		}()
	}

	if opt.IncludeSystem {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sys, err := c.CollectSystem(ctx)
			if err != nil {
				errCh <- err
				return
			}
			mu.Lock()
			metrics.System = sys
			mu.Unlock()
		}()
	}

	if opt.IncludeProcesses {
		wg.Add(1)
		go func() {
			defer wg.Done()
			procs, err := c.CollectProcesses(ctx, opt.TopNProcesses)
			if err != nil {
				errCh <- err
				return
			}
			mu.Lock()
			metrics.Processes = procs
			mu.Unlock()
		}()
	}

	wg.Wait()
	close(errCh)

	// 返回第一个错误（如果有）
	for err := range errCh {
		if err != nil {
			return metrics, err
		}
	}

	return metrics, nil
}

// Quick 快速采集基本指标（CPU、内存、磁盘）
func Quick(ctx context.Context) (*SystemMetrics, error) {
	return Default().Collect(ctx, DefaultOption())
}

// Full 采集完整指标
func Full(ctx context.Context) (*SystemMetrics, error) {
	return Default().Collect(ctx, FullOption())
}
