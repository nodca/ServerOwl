package memory

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/pgvector/pgvector-go"
)

//负责 Episode 和 Knowledge 的存储与语义检索

// 完整的问题解决过程
type Episode struct {
	ID             string     `json:"id"`
	SessionID      string     `json:"session_id"`
	UserID         string     `json:"user_id"`
	TriggerType    string     `json:"trigger_type"` // user_request | alert | scheduled
	TriggerSummary string     `json:"trigger_summary"`
	Steps          []TaskStep `json:"steps"`
	Outcome        string     `json:"outcome"` // success | partial | failed
	OutcomeSummary string     `json:"outcome_summary"`
	Embedding      []float32  `json:"-"` //json序列化时，忽略此字段
	Importance     float64    `json:"importance"`
	AccessCount    int        `json:"access_count"` //这条Episode被检索命中的次数
	LastAccessedAt *time.Time `json:"last_accessed_at"`
	Pinned         bool       `json:"pinned"` //手动标记为 永不删除
	Target         string     `json:"target"`
	Tags           []string   `json:"tags"`
	CreatedAt      time.Time  `json:"created_at"`
}

// 整合后的通用知识
type Knowledge struct {
	ID             string     `json:"id"`
	Topic          string     `json:"topic"`
	Content        string     `json:"content"`
	KeyPoints      []string   `json:"key_points"`
	SourceEpisodes []string   `json:"source_episodes"`
	Embedding      []float32  `json:"-"`
	Confidence     float64    `json:"confidence"` //置信度
	AccessCount    int        `json:"access_count"`
	LastAccessedAt *time.Time `json:"last_accessed_at"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type LongTermStore struct {
	db        *sql.DB
	embedding *EmbeddingService
}

func NewLongTermStore(db *sql.DB, embedding *EmbeddingService) *LongTermStore {
	return &LongTermStore{
		db:        db,
		embedding: embedding,
	}
}

// 保存 Episode
func (s *LongTermStore) SaveEpisode(ep *Episode) error {
	if ep.ID == "" {
		ep.ID = uuid.New().String()
	}
	if ep.CreatedAt.IsZero() {
		ep.CreatedAt = time.Now()
	}
	if ep.Importance == 0 {
		ep.Importance = 0.5
	}
	//生成embedding
	if s.embedding != nil && len(ep.Embedding) == 0 {
		text := ep.TriggerSummary
		if ep.OutcomeSummary != "" {
			text += " " + ep.OutcomeSummary
		}
		if vec, err := s.embedding.Embed(text); err == nil {
			ep.Embedding = vec
		}
	}

	stepsJSON, _ := json.Marshal(ep.Steps)

	var embeddingVal any
	if len(ep.Embedding) > 0 {
		embeddingVal = pgvector.NewVector(ep.Embedding)
	}

	_, err := s.db.Exec(`
                INSERT INTO episodes (id, session_id, user_id, trigger_type, trigger_summary, steps, outcome, outcome_summary,embedding, importance, access_count, pinned, target, tags, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
        `, ep.ID, ep.SessionID, ep.UserID, ep.TriggerType, ep.TriggerSummary, stepsJSON, ep.Outcome, ep.OutcomeSummary, embeddingVal, ep.Importance, ep.AccessCount, ep.Pinned, ep.Target, pq.Array(ep.Tags), ep.CreatedAt)

	return err
}

func (s *LongTermStore) searchEpisodesByEmbedding(query string, limit int) ([]Episode, error) {
	if s.embedding == nil {
		return nil, fmt.Errorf("embedding service not configured")
	}

	queryVec, err := s.embedding.Embed(query)
	if err != nil {
		return nil, fmt.Errorf("failed to embed query: %w", err)
	}

	rows, err := s.db.Query(`
	SELECT id, session_id, user_id, trigger_type, trigger_summary, steps, outcome, outcome_summary, importance,access_count, last_accessed_at, pinned, target, tags, created_at,
1 - (embedding <=> $1) as similarity
	FROM episodes
	WHERE embedding IS NOT NULL
	ORDER BY embedding <=> $1
	LIMIT $2
`, pgvector.NewVector(queryVec), limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search episodes: %w", err)
	}
	defer rows.Close()

	var episodes []Episode
	for rows.Next() {
		var ep Episode
		var stepsJSON []byte
		var tags pq.StringArray
		var similarity float64

		err := rows.Scan(
			&ep.ID, &ep.SessionID, &ep.UserID, &ep.TriggerType, &ep.TriggerSummary,
			&stepsJSON, &ep.Outcome, &ep.OutcomeSummary, &ep.Importance, &ep.AccessCount,
			&ep.LastAccessedAt, &ep.Pinned, &ep.Target, &tags, &ep.CreatedAt, &similarity,
		)
		if err != nil {
			continue
		}

		json.Unmarshal(stepsJSON, &ep.Steps)
		ep.Tags = []string(tags)
		episodes = append(episodes, ep)
	}

	return episodes, nil
}

// 语义检索 Episodes
func (s *LongTermStore) SearchEpisodes(query string, limit int) ([]Episode, error) {
	// 1. 先召回更多候选 (limit * 3)
	candidates, err := s.searchEpisodesByEmbedding(query, limit*3)
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return nil, nil
	}
	//重排序
	docs := make([]string, len(candidates))
	for i, ep := range candidates {
		docs[i] = ep.TriggerSummary + " " + ep.OutcomeSummary
	}

	indices, err := s.embedding.Rerank(query, docs)
	if err != nil {
		// 重排序失败，降级返回原结果
		if len(candidates) > limit {
			candidates = candidates[:limit]
		}
		return candidates, nil
	}
	//返回 TOP-K
	results := make([]Episode, 0, limit)
	for i, idx := range indices {
		if i >= limit {
			break
		}
		results = append(results, candidates[idx])
		go s.IncrementAccessCount(candidates[idx].ID)
	}
	return results, nil
}

// IncrementAccessCount 增加访问计数
func (s *LongTermStore) IncrementAccessCount(episodeID string) error {
	_, err := s.db.Exec(`
			UPDATE episodes
			SET access_count = access_count + 1, last_accessed_at = NOW()
			WHERE id = $1
	`, episodeID)
	return err
}

// GetEpisode 获取单个 Episode
func (s *LongTermStore) GetEpisode(id string) (*Episode, error) {
	var ep Episode
	var stepsJSON []byte
	var tags pq.StringArray

	err := s.db.QueryRow(`
			SELECT id, session_id, user_id, trigger_type, trigger_summary, steps, outcome, outcome_summary, importance,access_count, last_accessed_at, pinned, target, tags, created_at FROM episodes WHERE id = $1
	`, id).Scan(
		&ep.ID, &ep.SessionID, &ep.UserID, &ep.TriggerType, &ep.TriggerSummary,
		&stepsJSON, &ep.Outcome, &ep.OutcomeSummary, &ep.Importance, &ep.AccessCount,
		&ep.LastAccessedAt, &ep.Pinned, &ep.Target, &tags, &ep.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	json.Unmarshal(stepsJSON, &ep.Steps)
	ep.Tags = []string(tags)
	return &ep, nil
}

// 按目标获取 Episodes
func (s *LongTermStore) GetEpisodesByTarget(target string, limit int) ([]Episode, error) {
	rows, err := s.db.Query(`
	SELECT id, session_id, user_id, trigger_type, trigger_summary, steps, outcome, outcome_summary, importance,access_count, last_accessed_at, pinned, target, tags, created_at FROM episodes WHERE target = $1 ORDER BY created_at DESC LIMIT $2
	`, target, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []Episode
	for rows.Next() {
		var episode Episode
		var stepsJSON []byte
		var tags pq.StringArray
		err := rows.Scan(&episode.ID, &episode.SessionID, &episode.UserID, &episode.TriggerType, &episode.TriggerSummary, &stepsJSON, &episode.Outcome, &episode.OutcomeSummary, &episode.Importance, &episode.AccessCount, &episode.LastAccessedAt, &episode.Pinned, &episode.Target, &tags, &episode.CreatedAt)
		if err != nil {
			continue
		}
		episode.Tags = []string(tags)
		json.Unmarshal(stepsJSON, &episode.Steps)
		result = append(result, episode)
	}
	return result, nil
}

// 保存 Knowledge
func (s *LongTermStore) SaveKnowledge(k *Knowledge) error {
	if k.ID == "" {
		k.ID = uuid.New().String()
	}
	now := time.Now()
	if k.CreatedAt.IsZero() {
		k.CreatedAt = now
	}
	k.UpdatedAt = now

	// 生成 embedding
	if s.embedding != nil && len(k.Embedding) == 0 {
		text := k.Topic + " " + k.Content
		if vec, err := s.embedding.Embed(text); err == nil {
			k.Embedding = vec
		}
	}

	_, err := s.db.Exec(`
	INSERT INTO knowledge (id, topic, content, key_points, source_episodes, embedding, confidence, access_count,created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $1)
	`, k.ID, k.Topic, k.Content, pq.Array(k.KeyPoints), pq.Array(k.SourceEpisodes), pgvector.NewVector(k.Embedding), k.Confidence, k.AccessCount, k.CreatedAt, k.UpdatedAt)
	return err
}

// 语义检索Knowledge
func (s *LongTermStore) SearchKnowledge(query string, limit int) ([]Knowledge, error) {
	if s.embedding == nil {
		return nil, fmt.Errorf("embedding service not configured")
	}

	queryVec, err := s.embedding.Embed(query)
	if err != nil {
		return nil, fmt.Errorf("failed to embed query: %w", err)
	}

	rows, err := s.db.Query(`
                SELECT id, topic, content, key_points, source_episodes, confidence, access_count, last_accessed_at, created_at,updated_at, 1 - (embedding <=> $1) as similarity
                FROM knowledge
                WHERE embedding IS NOT NULL
                ORDER BY embedding <=> $1
                LIMIT $2
        `, pgvector.NewVector(queryVec), limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search knowledge: %w", err)
	}
	defer rows.Close()

	var results []Knowledge
	for rows.Next() {
		var k Knowledge
		var sourceEpisodesJSON []byte
		var keyPoints pq.StringArray
		var similarity float64

		err := rows.Scan(
			&k.ID, &k.Topic, &k.Content, &keyPoints, &sourceEpisodesJSON,
			&k.Confidence, &k.AccessCount, &k.LastAccessedAt, &k.CreatedAt, &k.UpdatedAt, &similarity,
		)
		if err != nil {
			continue
		}

		k.KeyPoints = keyPoints
		json.Unmarshal(sourceEpisodesJSON, &k.SourceEpisodes)
		results = append(results, k)
	}

	return results, nil
}
