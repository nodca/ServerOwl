package memory

import (
	"database/sql"
	"encoding/json"
	"log"
	"time"

	"github.com/lib/pq"
)

// 重要性计算常量
const (
	baseImportance       = 0.5
	timeDecayPerMonth    = -0.1
	maxTimeDecay         = -0.3
	accessBoostPerCount  = 0.05
	maxAccessBoost       = 0.2
	successBonus         = 0.1
	failedPenalty        = -0.05
	alertTriggerBonus    = 0.15
	complexStepsBonus    = 0.1
	complexStepsThreshold = 3
)

type ForgetGate struct {
	db          *sql.DB
	threshold   float64       // 遗忘阈值，低于此值的记录会被删除
	maxEpisodes int           // 最大 Episode 数量
	minAge      time.Duration // 最小保留期
}

// ForgetConfig 遗忘配置
type ForgetConfig struct {
	Threshold   float64       // 默认 0.25
	MaxEpisodes int           // 默认 10000
	MinAge      time.Duration // 默认 7 天
}

func NewForgetGate(db *sql.DB, cfg *ForgetConfig) *ForgetGate {
	if cfg == nil {
		cfg = &ForgetConfig{}
	}
	if cfg.Threshold == 0 {
		cfg.Threshold = 0.25
	}
	if cfg.MaxEpisodes == 0 {
		cfg.MaxEpisodes = 10000
	}
	if cfg.MinAge == 0 {
		cfg.MinAge = 7 * 24 * time.Hour
	}

	return &ForgetGate{
		db:          db,
		threshold:   cfg.Threshold,
		maxEpisodes: cfg.MaxEpisodes,
		minAge:      cfg.MinAge,
	}
}

// CalculateImportance 计算 Episode 的重要性评分
func (f *ForgetGate) CalculateImportance(ep *Episode) float64 {
	score := baseImportance

	// 按时间衰减: 每30天 -0.1，最多 -0.3
	age := time.Since(ep.CreatedAt)
	months := age.Hours() / (24 * 30)
	timeFactor := timeDecayPerMonth * months
	if timeFactor < maxTimeDecay {
		timeFactor = maxTimeDecay
	}
	score += timeFactor

	// 访问频率：每次 +0.05，最多 +0.2
	accessFactor := float64(ep.AccessCount) * accessBoostPerCount
	if accessFactor > maxAccessBoost {
		accessFactor = maxAccessBoost
	}
	score += accessFactor

	// 结果因子
	switch ep.Outcome {
	case "success":
		score += successBonus
	case "failed":
		score += failedPenalty
	}

	// 类型因子：告警触发的更重要
	if ep.TriggerType == "alert" {
		score += alertTriggerBonus
	}

	// 复杂度因子：步骤多的更重要
	if len(ep.Steps) > complexStepsThreshold {
		score += complexStepsBonus
	}

	// 限制在 0-1 范围
	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}

	return score
}

// UpdateAllImportance 批量更新所有 Episode 的重要性评分
func (f *ForgetGate) UpdateAllImportance() error {
	rows, err := f.db.Query(`
		SELECT id, trigger_type, steps, outcome, access_count, created_at
		FROM episodes
		WHERE pinned = false
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	var ids []string
	var importances []float64

	for rows.Next() {
		var ep Episode
		var stepsJSON []byte

		err := rows.Scan(&ep.ID, &ep.TriggerType, &stepsJSON, &ep.Outcome, &ep.AccessCount, &ep.CreatedAt)
		if err != nil {
			log.Printf("[ForgetGate] failed to scan episode: %v", err)
			continue
		}

		var steps []TaskStep
		if err := json.Unmarshal(stepsJSON, &steps); err == nil {
			ep.Steps = steps
		}

		ids = append(ids, ep.ID)
		importances = append(importances, f.CalculateImportance(&ep))
	}

	if err := rows.Err(); err != nil {
		return err
	}

	if len(ids) == 0 {
		return nil
	}

	// 使用 unnest 批量更新
	_, err = f.db.Exec(`
		UPDATE episodes
		SET importance = tmp.importance
		FROM unnest($1::text[], $2::float8[]) AS tmp(id, importance)
		WHERE episodes.id = tmp.id
	`, pq.Array(ids), pq.Array(importances))

	return err
}

// Cleanup 清理低重要性的 Episode
func (f *ForgetGate) Cleanup() (int, error) {
	// 1. 更新所有重要性评分
	if err := f.UpdateAllImportance(); err != nil {
		return 0, err
	}

	// 2. 删除低于阈值的记录（排除 pinned 和最小保留期内的）
	minTime := time.Now().Add(-f.minAge)

	result, err := f.db.Exec(`
		DELETE FROM episodes
		WHERE pinned = false
		  AND created_at < $1
		  AND importance < $2
	`, minTime, f.threshold)
	if err != nil {
		return 0, err
	}

	deleted, _ := result.RowsAffected()

	// 3. 如果总数仍超过上限，按重要性删除最低的
	var count int
	if err := f.db.QueryRow(`SELECT COUNT(*) FROM episodes`).Scan(&count); err != nil {
		return int(deleted), err
	}

	if count > f.maxEpisodes {
		excess := count - f.maxEpisodes
		_, err = f.db.Exec(`
			DELETE FROM episodes
			WHERE id IN (
				SELECT id FROM episodes
				WHERE pinned = false AND created_at < $1
				ORDER BY importance ASC
				LIMIT $2
			)
		`, minTime, excess)
		if err != nil {
			return int(deleted), err
		}
		deleted += int64(excess)
	}

	if deleted > 0 {
		log.Printf("[ForgetGate] Cleaned up %d episodes", deleted)
	}

	return int(deleted), nil
}

// PinEpisode 标记 Episode 为永不删除
func (f *ForgetGate) PinEpisode(episodeID string) error {
	_, err := f.db.Exec(`UPDATE episodes SET pinned = true WHERE id = $1`, episodeID)
	return err
}

// UnpinEpisode 取消永不删除标记
func (f *ForgetGate) UnpinEpisode(episodeID string) error {
	_, err := f.db.Exec(`UPDATE episodes SET pinned = false WHERE id = $1`, episodeID)
	return err
}
