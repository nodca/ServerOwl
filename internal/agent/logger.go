package agent

import (
	"database/sql"
	"encoding/json"
	"log"
	"time"

	"github.com/robfig/cron/v3"
	_ "github.com/lib/pq"
)

/*记录所有 Agent 的工具调用，方便审计和调试。*/

type ActionLogger struct {
	db *sql.DB
}

// 操作日志
type ActionLog struct {
	ID         int64
	SessionID  string
	UserID     string
	RequestID  string
	ToolName   string
	Arguments  string //JSON
	Result     string
	Success    bool
	ErrorMsg   string
	DurationMs int64
	CreatedAt  time.Time
}

// NewActionLogger 创建 ActionLogger（使用 PostgreSQL DSN）
func NewActionLogger(dsn string) (*ActionLogger, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	logger := &ActionLogger{db: db}

	// 创建表
	if err := logger.createTable(); err != nil {
		return nil, err
	}

	return logger, nil
}

// createTable 创建日志表
func (l *ActionLogger) createTable() error {
	query := `
	CREATE TABLE IF NOT EXISTS agent_actions (
		id SERIAL PRIMARY KEY,
		session_id TEXT NOT NULL,
		user_id TEXT NOT NULL,
		request_id TEXT,
		tool_name TEXT NOT NULL,
		arguments TEXT NOT NULL,
		result TEXT,
		success BOOLEAN NOT NULL,
		error_msg TEXT,
		duration_ms INTEGER,
		created_at TIMESTAMPTZ DEFAULT NOW()
	)`

	_, err := l.db.Exec(query)
	return err
}

// 记录操作
func (l *ActionLogger) Log(sessionID, userID, requestID, toolName string, args map[string]any, result string, err error, durationMs int64) error {
	// 序列化参数
	argsJSON, _ := json.Marshal(args)

	success := (err == nil)
	errorMsg := ""
	if err != nil {
		errorMsg = err.Error()
	}

	query := `
	INSERT INTO agent_actions (session_id, user_id, request_id, tool_name, arguments, result, success, error_msg, duration_ms)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	_, execErr := l.db.Exec(query, sessionID, userID, requestID, toolName, string(argsJSON), result, success, errorMsg, durationMs)
	return execErr
}

func (l *ActionLogger) GetRecentLogs(limit int) ([]ActionLog, error) {
	query := `
	SELECT id, session_id, user_id, request_id, tool_name, arguments, result, success, error_msg, duration_ms, created_at
	FROM agent_actions
	ORDER BY created_at DESC
	LIMIT $1
	`

	rows, err := l.db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []ActionLog
	for rows.Next() {
		var log ActionLog
		err := rows.Scan(&log.ID, &log.SessionID, &log.UserID, &log.RequestID, &log.ToolName,
			&log.Arguments, &log.Result, &log.Success, &log.ErrorMsg, &log.DurationMs, &log.CreatedAt)
		if err != nil {
			continue
		}
		logs = append(logs, log)
	}

	return logs, nil
}

func (l *ActionLogger) CleanOldLogs(daysToKeep int) (int64, error) {
	query := `
	DELETE FROM agent_actions WHERE created_at < NOW() - INTERVAL '1 day' * $1
	`
	result, err := l.db.Exec(query, daysToKeep)
	if err != nil {
		return 0, err
	}

	rowsAffected, _ := result.RowsAffected()
	return rowsAffected, nil
}

// 定时清理日志
func (l *ActionLogger) StartAutoCleanup(daysToKeep int, cronExpr string) (*cron.Cron, error) {
	c := cron.New()

	_, err := c.AddFunc(cronExpr, func() {
		deleted, err := l.CleanOldLogs(daysToKeep)
		if err != nil {
			return
		}
		if deleted > 0 {
			log.Printf("自动清理了 %d 条旧日志", deleted)
		}
	})
	if err != nil {
		return nil, err
	}
	// 启动 cron
	c.Start()

	return c, nil
}
