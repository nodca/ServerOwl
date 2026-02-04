package scheduler

import (
  "database/sql"
  "encoding/json"
  "fmt"
  "time"
)

type Store interface {
  SaveTask(task *ScheduledTask) error
  LoadTasks() ([]*ScheduledTask, error)
  DeleteTask(taskID string) error
  SaveTaskRun(taskID string, result *TaskResult) error
  GetTaskHistory(taskID string, limit int) ([]*TaskResult, error)
}

type PostgresStore struct {
  db *sql.DB
}

func NewPostgresStore(db *sql.DB) (*PostgresStore, error) {
  store := &PostgresStore{db: db}
  if err := store.initSchema(); err != nil {
    return nil, err
  }
  return store, nil
}

func (s *PostgresStore) initSchema() error {
  schema := `
  CREATE TABLE IF NOT EXISTS scheduled_tasks (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    schedule VARCHAR(64) NOT NULL,
    type VARCHAR(32) NOT NULL,
    config JSONB NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    tags TEXT[],
    last_run TIMESTAMP,
    next_run TIMESTAMP,
    run_count BIGINT DEFAULT 0,
    fail_count BIGINT DEFAULT 0,
    last_result TEXT,
    last_error TEXT,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
  );

  CREATE TABLE IF NOT EXISTS task_runs (
    id SERIAL PRIMARY KEY,
    task_id VARCHAR(64) REFERENCES scheduled_tasks(id) ON DELETE CASCADE,
    success BOOLEAN NOT NULL,
    output TEXT,
    error TEXT,
    start_time TIMESTAMP NOT NULL,
    end_time TIMESTAMP NOT NULL,
    duration_ms BIGINT NOT NULL
  );

  CREATE INDEX IF NOT EXISTS idx_task_runs_task_id ON task_runs(task_id);
  CREATE INDEX IF NOT EXISTS idx_task_runs_start_time ON task_runs(start_time);
  `
  _, err := s.db.Exec(schema)
  return err
}

func (s *PostgresStore) SaveTask(task *ScheduledTask) error {
  configJSON, err := json.Marshal(task.Config)
  if err != nil {
    return err
  }

  query := `
  INSERT INTO scheduled_tasks (id, name, description, schedule, type, config, status, tags, last_run, next_run, run_count, fail_count, last_result, last_error, updated_at)
  VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, NOW())
  ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    schedule = EXCLUDED.schedule,
    type = EXCLUDED.type,
    config = EXCLUDED.config,
    status = EXCLUDED.status,
    tags = EXCLUDED.tags,
    last_run = EXCLUDED.last_run,
    next_run = EXCLUDED.next_run,
    run_count = EXCLUDED.run_count,
    fail_count = EXCLUDED.fail_count,
    last_result = EXCLUDED.last_result,
    last_error = EXCLUDED.last_error,
    updated_at = NOW()
  `

  _, err = s.db.Exec(query,
    task.ID, task.Name, task.Description, task.Schedule,
    task.Type, configJSON, task.Status, task.Tags,
    task.LastRun, task.NextRun, task.RunCount, task.FailCount,
    task.LastResult, task.LastError,
  )
  return err
}

func (s *PostgresStore) LoadTasks() ([]*ScheduledTask, error) {
  query := `SELECT id, name, description, schedule, type, config, status, tags, last_run, next_run, run_count, fail_count, last_result, last_error, created_at, updated_at FROM scheduled_tasks`

  rows, err := s.db.Query(query)
  if err != nil {
    return nil, err
  }
  defer rows.Close()

  var tasks []*ScheduledTask
  for rows.Next() {
    task := &ScheduledTask{}
    var configJSON []byte
    var lastRun, nextRun, createdAt, updatedAt sql.NullTime

    err := rows.Scan(
      &task.ID, &task.Name, &task.Description, &task.Schedule,
      &task.Type, &configJSON, &task.Status, &task.Tags,
      &lastRun, &nextRun, &task.RunCount, &task.FailCount,
      &task.LastResult, &task.LastError, &createdAt, &updatedAt,
    )
    if err != nil {
      return nil, err
    }

    if err := json.Unmarshal(configJSON, &task.Config); err != nil {
      return nil, fmt.Errorf("unmarshal config for task %s: %w", task.ID, err)
    }

    if lastRun.Valid {
      task.LastRun = lastRun.Time
    }
    if nextRun.Valid {
      task.NextRun = nextRun.Time
    }
    if createdAt.Valid {
      task.CreatedAt = createdAt.Time
    }
    if updatedAt.Valid {
      task.UpdatedAt = updatedAt.Time
    }

    tasks = append(tasks, task)
  }

  return tasks, nil
}

func (s *PostgresStore) DeleteTask(taskID string) error {
  _, err := s.db.Exec("DELETE FROM scheduled_tasks WHERE id = $1", taskID)
  return err
}

func (s *PostgresStore) SaveTaskRun(taskID string, result *TaskResult) error {
  if result == nil {
    return nil
  }

  query := `INSERT INTO task_runs (task_id, success, output, error, start_time, end_time, duration_ms) VALUES ($1, $2, $3, $4, $5, $6, $7)`
  _, err := s.db.Exec(query,
    taskID, result.Success, result.Output, result.Error,
    result.StartTime, result.EndTime, result.Duration.Milliseconds(),
  )
  return err
}

func (s *PostgresStore) GetTaskHistory(taskID string, limit int) ([]*TaskResult, error) {
  query := `SELECT task_id, success, output, error, start_time, end_time, duration_ms FROM task_runs WHERE task_id = $1 ORDER BY start_time DESC LIMIT $2`

  rows, err := s.db.Query(query, taskID, limit)
  if err != nil {
    return nil, err
  }
  defer rows.Close()

  var results []*TaskResult
  for rows.Next() {
    r := &TaskResult{}
    var durationMs int64
    err := rows.Scan(&r.TaskID, &r.Success, &r.Output, &r.Error, &r.StartTime, &r.EndTime, &durationMs)
    if err != nil {
      return nil, err
    }
    r.Duration = time.Duration(durationMs) * time.Millisecond
    results = append(results, r)
  }

  return results, nil
}
