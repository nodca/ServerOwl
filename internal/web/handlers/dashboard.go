package handlers

import (
	"net/http"
	"runtime"
	"time"

	"serverowl/internal/memory"
	"serverowl/internal/monitor"
	"serverowl/internal/scheduler"
	"serverowl/internal/skills"
)

// DashboardHandler Dashboard API 处理器
type DashboardHandler struct {
	monitor       *monitor.Monitor
	scheduler     *scheduler.Scheduler
	skillManager  *skills.Manager
	memoryManager *memory.MemoryManager
	startTime     time.Time
}

// NewDashboardHandler 创建 Dashboard 处理器
func NewDashboardHandler(
	mon *monitor.Monitor,
	sched *scheduler.Scheduler,
	skillMgr *skills.Manager,
	memMgr *memory.MemoryManager,
) *DashboardHandler {
	return &DashboardHandler{
		monitor:       mon,
		scheduler:     sched,
		skillManager:  skillMgr,
		memoryManager: memMgr,
		startTime:     time.Now(),
	}
}

// SystemStats 系统状态统计
type SystemStats struct {
	Uptime        string         `json:"uptime"`
	UptimeSeconds int64          `json:"uptime_seconds"`
	GoVersion     string         `json:"go_version"`
	NumGoroutine  int            `json:"num_goroutine"`
	NumCPU        int            `json:"num_cpu"`
	MemStats      MemoryStats    `json:"memory_stats"`
	TaskStats     TaskStats      `json:"task_stats"`
	SkillStats    SkillStats     `json:"skill_stats"`
	Timestamp     time.Time      `json:"timestamp"`
}

// MemoryStats 内存统计
type MemoryStats struct {
	Alloc      uint64 `json:"alloc"`       // 当前分配的内存 (bytes)
	TotalAlloc uint64 `json:"total_alloc"` // 累计分配的内存 (bytes)
	Sys        uint64 `json:"sys"`         // 从系统获取的内存 (bytes)
	NumGC      uint32 `json:"num_gc"`      // GC 次数
}

// TaskStats 任务统计
type TaskStats struct {
	TotalTasks  int `json:"total_tasks"`
	ActiveTasks int `json:"active_tasks"`
	PausedTasks int `json:"paused_tasks"`
}

// SkillStats 技能统计
type SkillStats struct {
	TotalSkills int `json:"total_skills"`
}

// GetStats 获取系统状态统计
// GET /api/v1/dashboard/stats
func (h *DashboardHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}

	// 计算运行时间
	uptime := time.Since(h.startTime)

	// 获取内存统计
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// 获取任务统计
	taskStats := TaskStats{}
	if h.scheduler != nil {
		tasks := h.scheduler.ListTasks()
		taskStats.TotalTasks = len(tasks)
		for _, t := range tasks {
			switch t.Status {
			case scheduler.TaskStatusActive:
				taskStats.ActiveTasks++
			case scheduler.TaskStatusPaused:
				taskStats.PausedTasks++
			}
		}
	}

	// 获取技能统计
	skillStats := SkillStats{}
	if h.skillManager != nil {
		skillStats.TotalSkills = h.skillManager.SkillCount()
	}

	stats := SystemStats{
		Uptime:        uptime.String(),
		UptimeSeconds: int64(uptime.Seconds()),
		GoVersion:     runtime.Version(),
		NumGoroutine:  runtime.NumGoroutine(),
		NumCPU:        runtime.NumCPU(),
		MemStats: MemoryStats{
			Alloc:      memStats.Alloc,
			TotalAlloc: memStats.TotalAlloc,
			Sys:        memStats.Sys,
			NumGC:      memStats.NumGC,
		},
		TaskStats:  taskStats,
		SkillStats: skillStats,
		Timestamp:  time.Now(),
	}

	writeSuccess(w, stats)
}

// HealthStatus 健康检查状态
type HealthStatus struct {
	Status     string            `json:"status"` // healthy, degraded, unhealthy
	Components map[string]string `json:"components"`
	Timestamp  time.Time         `json:"timestamp"`
}

// GetHealth 获取健康检查状态
// GET /api/v1/dashboard/health
func (h *DashboardHandler) GetHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}

	components := make(map[string]string)
	overallStatus := "healthy"

	// 检查调度器
	if h.scheduler != nil {
		components["scheduler"] = "healthy"
	} else {
		components["scheduler"] = "not_configured"
	}

	// 检查技能管理器
	if h.skillManager != nil {
		components["skill_manager"] = "healthy"
	} else {
		components["skill_manager"] = "not_configured"
	}

	// 检查记忆管理器
	if h.memoryManager != nil {
		components["memory_manager"] = "healthy"
	} else {
		components["memory_manager"] = "not_configured"
	}

	// 检查监控器
	if h.monitor != nil {
		components["monitor"] = "healthy"
	} else {
		components["monitor"] = "not_configured"
	}

	// 判断整体状态
	unhealthyCount := 0
	for _, status := range components {
		if status == "unhealthy" {
			unhealthyCount++
		}
	}

	if unhealthyCount > 0 {
		if unhealthyCount >= len(components)/2 {
			overallStatus = "unhealthy"
		} else {
			overallStatus = "degraded"
		}
	}

	health := HealthStatus{
		Status:     overallStatus,
		Components: components,
		Timestamp:  time.Now(),
	}

	writeSuccess(w, health)
}
