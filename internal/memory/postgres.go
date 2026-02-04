package memory

import (
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
)

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(dsn string) (*PostgresStore, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to postgres: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping postgres: %w", err)
	}
	store := &PostgresStore{db: db}
	if err := store.initTables(); err != nil {
		return nil, err
	}

	return store, nil
}
func (s *PostgresStore) initTables() error {
	// 建表 SQL
	schema := `
	-- 1. 启用 pgvector 扩展
	CREATE EXTENSION IF NOT EXISTS vector;
	-- 2. 工作记忆：当前任务状态
	CREATE TABLE IF NOT EXISTS task_states (
		id TEXT PRIMARY KEY,
		session_id TEXT NOT NULL,
		user_id TEXT NOT NULL,
		original_request TEXT NOT NULL,
		summary TEXT,
		steps JSONB DEFAULT '[]',
		current_step INTEGER DEFAULT 0,
		context JSONB DEFAULT '{}',
		status TEXT DEFAULT 'in_progress', -- in_progress | completed | failed
		started_at TIMESTAMPTZ DEFAULT NOW(),
        updated_at TIMESTAMPTZ DEFAULT NOW()
	);
	CREATE INDEX IF NOT EXISTS idx_task_states_session ON task_states(session_id);
  	CREATE INDEX IF NOT EXISTS idx_task_states_status ON task_states(status);
	 -- 3. 长期记忆：Episodes (完整的问题解决过程)
  CREATE TABLE IF NOT EXISTS episodes (
      id TEXT PRIMARY KEY,
      session_id TEXT,
      user_id TEXT,

      -- 触发条件
      trigger_type TEXT NOT NULL,      -- user_request | alert | scheduled
      trigger_summary TEXT NOT NULL,

      -- 过程记录
      steps JSONB NOT NULL,            -- [{tool, args, result, duration_ms}]

      -- 结果
      outcome TEXT NOT NULL,           -- success | partial | failed
      outcome_summary TEXT,

      -- 向量检索
      embedding vector(1024),          -- BGE-large-zh 维度

      -- 重要性评分 (遗忘门)
      importance REAL DEFAULT 0.5,
      access_count INTEGER DEFAULT 0,
      last_accessed_at TIMESTAMPTZ,
      pinned BOOLEAN DEFAULT FALSE,

      -- 元数据
      target TEXT,
      tags TEXT[] DEFAULT '{}',
      consolidated_to TEXT,            -- 整合到的 Knowledge ID
      created_at TIMESTAMPTZ DEFAULT NOW()
  );

  CREATE INDEX IF NOT EXISTS idx_episodes_target ON episodes(target);
  CREATE INDEX IF NOT EXISTS idx_episodes_trigger ON episodes(trigger_type);
  CREATE INDEX IF NOT EXISTS idx_episodes_importance ON episodes(importance DESC);
  CREATE INDEX IF NOT EXISTS idx_episodes_created ON episodes(created_at);
  -- 向量索引 (需要有数据后才能创建 ivfflat，先用 hnsw 或跳过)
  -- CREATE INDEX idx_episodes_embedding ON episodes USING hnsw (embedding vector_cosine_ops);

  -- 4. 长期记忆：Knowledge (整合后的通用知识)
  CREATE TABLE IF NOT EXISTS knowledge (
      id TEXT PRIMARY KEY,
      topic TEXT NOT NULL,
      content TEXT NOT NULL,
      key_points TEXT[] DEFAULT '{}',
      source_episodes TEXT[] DEFAULT '{}',
      embedding vector(1024),
      confidence REAL DEFAULT 0.5,
      access_count INTEGER DEFAULT 0,
      last_accessed_at TIMESTAMPTZ,
      created_at TIMESTAMPTZ DEFAULT NOW(),
      updated_at TIMESTAMPTZ DEFAULT NOW()
  );

  CREATE INDEX IF NOT EXISTS idx_knowledge_topic ON knowledge(topic);
  CREATE INDEX IF NOT EXISTS idx_knowledge_confidence ON knowledge(confidence DESC);

  -- 5. 待确认操作
  CREATE TABLE IF NOT EXISTS pending_actions (
      id TEXT PRIMARY KEY,
      user_id TEXT NOT NULL,
      session_id TEXT NOT NULL,
      user_input TEXT,
      tool_name TEXT NOT NULL,
      args_json TEXT NOT NULL,
      prompt TEXT,
      status TEXT NOT NULL,           -- pending | confirmed | cancelled | expired | executed | failed
      error_msg TEXT,
      created_at TIMESTAMPTZ DEFAULT NOW(),
      expires_at TIMESTAMPTZ
  );

  CREATE INDEX IF NOT EXISTS idx_pending_actions_session ON pending_actions(session_id);
  CREATE INDEX IF NOT EXISTS idx_pending_actions_status ON pending_actions(status);
  CREATE INDEX IF NOT EXISTS idx_pending_actions_expires ON pending_actions(expires_at);
	`
	_, err := s.db.Exec(schema)
	return err
}

func (s *PostgresStore) Close() error {
	return s.db.Close()
}
