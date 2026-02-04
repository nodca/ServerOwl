package metrics

import (
	"context"
	"sort"

	"github.com/shirou/gopsutil/v3/process"
)

// CollectProcesses 采集进程信息
func (c *defaultCollector) CollectProcesses(ctx context.Context, topN int) ([]ProcessInfo, error) {
	if topN <= 0 {
		topN = 10
	}

	procs, err := process.ProcessesWithContext(ctx)
	if err != nil {
		return nil, err
	}

	procInfos := make([]ProcessInfo, 0, len(procs))
	for _, p := range procs {
		info := collectProcessInfo(ctx, p, true)
		procInfos = append(procInfos, info)
	}

	sort.Slice(procInfos, func(i, j int) bool {
		return procInfos[i].CPUPercent > procInfos[j].CPUPercent
	})

	if len(procInfos) > topN {
		procInfos = procInfos[:topN]
	}

	return procInfos, nil
}

// collectProcessInfo 采集单个进程信息
func collectProcessInfo(ctx context.Context, p *process.Process, full bool) ProcessInfo {
	info := ProcessInfo{PID: p.Pid}

	info.Name, _ = p.NameWithContext(ctx)
	info.MemPercent, _ = p.MemoryPercentWithContext(ctx)

	if memInfo, err := p.MemoryInfoWithContext(ctx); err == nil && memInfo != nil {
		info.MemoryRSS = memInfo.RSS
	}

	if full {
		info.Username, _ = p.UsernameWithContext(ctx)
		info.CPUPercent, _ = p.CPUPercentWithContext(ctx)
		info.CreateTime, _ = p.CreateTimeWithContext(ctx)
		info.CommandLine, _ = p.CmdlineWithContext(ctx)
		if status, err := p.StatusWithContext(ctx); err == nil && len(status) > 0 {
			info.Status = status[0]
		}
	}

	return info
}

// GetTopProcessesByCPU 获取 CPU 使用率最高的进程
func GetTopProcessesByCPU(ctx context.Context, topN int) ([]ProcessInfo, error) {
	return Default().CollectProcesses(ctx, topN)
}

// GetTopProcessesByMemory 获取内存使用率最高的进程
func GetTopProcessesByMemory(ctx context.Context, topN int) ([]ProcessInfo, error) {
	if topN <= 0 {
		topN = 10
	}

	procs, err := process.ProcessesWithContext(ctx)
	if err != nil {
		return nil, err
	}

	procInfos := make([]ProcessInfo, 0, len(procs))
	for _, p := range procs {
		info := collectProcessInfo(ctx, p, false)
		procInfos = append(procInfos, info)
	}

	sort.Slice(procInfos, func(i, j int) bool {
		return procInfos[i].MemPercent > procInfos[j].MemPercent
	})

	if len(procInfos) > topN {
		procInfos = procInfos[:topN]
	}

	return procInfos, nil
}
