package scheduler

import (
  "fmt"
  "log"
  "sync"
  "time"

  "github.com/robfig/cron/v3"
)

type Scheduler struct {
  cron     *cron.Cron
  tasks    map[string]*ScheduledTask
  mu       sync.RWMutex
  store    Store
  executor TaskExecutor
}

type TaskExecutor interface {
  ExecuteTask(task *ScheduledTask) (*TaskResult, error)
}

func New(store Store, executor TaskExecutor) *Scheduler {
  return &Scheduler{
    cron:     cron.New(cron.WithSeconds()),
    tasks:    make(map[string]*ScheduledTask),
    store:    store,
    executor: executor,
  }
}

func (s *Scheduler) Start() {
  s.cron.Start()
  log.Printf("[Scheduler] Started")
}

func (s *Scheduler) Stop() {
  ctx := s.cron.Stop()
  <-ctx.Done()
  log.Printf("[Scheduler] Stopped")
}

func (s *Scheduler) AddTask(task *ScheduledTask) error {
  s.mu.Lock()
  defer s.mu.Unlock()

  if _, exists := s.tasks[task.ID]; exists {
    return fmt.Errorf("task %s already exists", task.ID)
  }

  entryID, err := s.cron.AddFunc(task.Schedule, func() {
    s.runTask(task)
  })
  if err != nil {
    return fmt.Errorf("invalid schedule %q: %w", task.Schedule, err)
  }

  task.EntryID = entryID
  task.Status = TaskStatusActive
  s.tasks[task.ID] = task

  if s.store != nil {
    if err := s.store.SaveTask(task); err != nil {
      log.Printf("[Scheduler] Failed to save task %s: %v", task.ID, err)
    }
  }

  log.Printf("[Scheduler] Added task %s with schedule %s", task.ID, task.Schedule)
  return nil
}

func (s *Scheduler) RemoveTask(taskID string) error {
  s.mu.Lock()
  defer s.mu.Unlock()

  task, exists := s.tasks[taskID]
  if !exists {
    return fmt.Errorf("task %s not found", taskID)
  }

  s.cron.Remove(task.EntryID)
  delete(s.tasks, taskID)

  if s.store != nil {
    if err := s.store.DeleteTask(taskID); err != nil {
      log.Printf("[Scheduler] Failed to delete task %s: %v", taskID, err)
    }
  }

  log.Printf("[Scheduler] Removed task %s", taskID)
  return nil
}

func (s *Scheduler) GetTask(taskID string) (*ScheduledTask, bool) {
  s.mu.RLock()
  defer s.mu.RUnlock()
  task, ok := s.tasks[taskID]
  return task, ok
}

func (s *Scheduler) ListTasks() []*ScheduledTask {
  s.mu.RLock()
  defer s.mu.RUnlock()

  tasks := make([]*ScheduledTask, 0, len(s.tasks))
  for _, t := range s.tasks {
    tasks = append(tasks, t)
  }
  return tasks
}

func (s *Scheduler) runTask(task *ScheduledTask) {
  log.Printf("[Scheduler] Running task %s", task.ID)

  task.LastRun = time.Now()
  task.RunCount++

  result, err := s.executor.ExecuteTask(task)
  if err != nil {
    task.LastError = err.Error()
    task.FailCount++
    log.Printf("[Scheduler] Task %s failed: %v", task.ID, err)
  } else {
    task.LastError = ""
    task.LastResult = result.Output
    log.Printf("[Scheduler] Task %s completed successfully", task.ID)
  }

  task.NextRun = s.cron.Entry(task.EntryID).Next

  if s.store != nil {
    if err := s.store.SaveTaskRun(task.ID, result); err != nil {
      log.Printf("[Scheduler] Failed to save task run: %v", err)
    }
  }
}

func (s *Scheduler) LoadTasks() error {
  if s.store == nil {
    return nil
  }

  tasks, err := s.store.LoadTasks()
  if err != nil {
    return err
  }

  for _, task := range tasks {
    if task.Status == TaskStatusActive {
      if err := s.AddTask(task); err != nil {
        log.Printf("[Scheduler] Failed to load task %s: %v", task.ID, err)
      }
    }
  }

  return nil
}
