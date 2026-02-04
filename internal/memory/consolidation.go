package memory

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// LLMClient LLM 调用接口
type LLMClient interface {
	Chat(prompt string) (string, error)
}

// Consolidator 记忆整合器，将相似 Episodes 整合为 Knowledge
type Consolidator struct {
	db              *sql.DB
	longTerm        *LongTermStore
	llm             LLMClient
	minEpisodes     int           // 触发整合的最小 Episode 数量
	similarityThres float64       // 相似度阈值
	checkInterval   time.Duration // 检查间隔
}

// ConsolidatorConfig 整合器配置
type ConsolidatorConfig struct {
	MinEpisodes     int           // 默认 5
	SimilarityThres float64       // 默认 0.75
	CheckInterval   time.Duration // 默认 24 小时
}

func NewConsolidator(db *sql.DB, longTerm *LongTermStore, llm LLMClient, cfg *ConsolidatorConfig) *Consolidator {
	if cfg == nil {
		cfg = &ConsolidatorConfig{}
	}
	if cfg.MinEpisodes == 0 {
		cfg.MinEpisodes = 5
	}
	if cfg.SimilarityThres == 0 {
		cfg.SimilarityThres = 0.75
	}
	if cfg.CheckInterval == 0 {
		cfg.CheckInterval = 24 * time.Hour
	}

	return &Consolidator{
		db:              db,
		longTerm:        longTerm,
		llm:             llm,
		minEpisodes:     cfg.MinEpisodes,
		similarityThres: cfg.SimilarityThres,
		checkInterval:   cfg.CheckInterval,
	}
}

// ConsolidationResult 整合结果
type ConsolidationResult struct {
	Topic      string   `json:"topic"`
	Content    string   `json:"content"`
	KeyPoints  []string `json:"key_points"`
	Confidence float64  `json:"confidence"`
}

// Run 执行一次整合检查
func (c *Consolidator) Run() (int, error) {
	// 1. 找出未整合的 Episodes 按 target 分组
	groups, err := c.findConsolidationCandidates()
	if err != nil {
		return 0, fmt.Errorf("failed to find candidates: %w", err)
	}

	consolidated := 0
	for target, episodeIDs := range groups {
		if len(episodeIDs) < c.minEpisodes {
			continue
		}

		// 2. 获取这些 Episodes 的详细信息
		episodes, err := c.getEpisodesByIDs(episodeIDs)
		if err != nil {
			log.Printf("[Consolidator] failed to get episodes for target %s: %v", target, err)
			continue
		}

		// 3. 调用 LLM 生成知识摘要
		result, err := c.generateKnowledge(target, episodes)
		if err != nil {
			log.Printf("[Consolidator] failed to generate knowledge for target %s: %v", target, err)
			continue
		}

		// 4. 保存 Knowledge
		knowledge := &Knowledge{
			ID:             uuid.New().String(),
			Topic:          result.Topic,
			Content:        result.Content,
			KeyPoints:      result.KeyPoints,
			SourceEpisodes: episodeIDs,
			Confidence:     result.Confidence,
		}

		if err := c.longTerm.SaveKnowledge(knowledge); err != nil {
			log.Printf("[Consolidator] failed to save knowledge: %v", err)
			continue
		}

		// 5. 标记 Episodes 已整合
		if err := c.markEpisodesConsolidated(episodeIDs, knowledge.ID); err != nil {
			log.Printf("[Consolidator] failed to mark episodes consolidated: %v", err)
		}

		consolidated++
		log.Printf("[Consolidator] created knowledge %s from %d episodes (target: %s)", knowledge.ID, len(episodeIDs), target)
	}

	return consolidated, nil
}

// findConsolidationCandidates 找出可整合的 Episodes，按 target 分组
func (c *Consolidator) findConsolidationCandidates() (map[string][]string, error) {
	rows, err := c.db.Query(`
		SELECT id, target
		FROM episodes
		WHERE consolidated_to IS NULL
		  AND outcome = 'success'
		  AND target IS NOT NULL
		  AND target != ''
		ORDER BY target, created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	groups := make(map[string][]string)
	for rows.Next() {
		var id, target string
		if err := rows.Scan(&id, &target); err != nil {
			continue
		}
		groups[target] = append(groups[target], id)
	}

	return groups, rows.Err()
}

// getEpisodesByIDs 批量获取 Episodes
func (c *Consolidator) getEpisodesByIDs(ids []string) ([]Episode, error) {
	rows, err := c.db.Query(`
		SELECT id, trigger_type, trigger_summary, steps, outcome, outcome_summary, target, tags, created_at
		FROM episodes
		WHERE id = ANY($1)
		ORDER BY created_at DESC
	`, pq.Array(ids))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var episodes []Episode
	for rows.Next() {
		var ep Episode
		var stepsJSON []byte
		var tags pq.StringArray

		err := rows.Scan(&ep.ID, &ep.TriggerType, &ep.TriggerSummary, &stepsJSON, &ep.Outcome, &ep.OutcomeSummary, &ep.Target, &tags, &ep.CreatedAt)
		if err != nil {
			continue
		}

		json.Unmarshal(stepsJSON, &ep.Steps)
		ep.Tags = []string(tags)
		episodes = append(episodes, ep)
	}

	return episodes, rows.Err()
}

// generateKnowledge 调用 LLM 生成知识摘要
func (c *Consolidator) generateKnowledge(target string, episodes []Episode) (*ConsolidationResult, error) {
	// 构建 prompt
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("请根据以下关于 \"%s\" 的 %d 条操作记录，整合生成一条通用知识。\n\n", target, len(episodes)))

	for i, ep := range episodes {
		sb.WriteString(fmt.Sprintf("### 记录 %d\n", i+1))
		sb.WriteString(fmt.Sprintf("- 触发: %s\n", ep.TriggerSummary))
		sb.WriteString(fmt.Sprintf("- 结果: %s\n", ep.OutcomeSummary))
		if len(ep.Steps) > 0 {
			sb.WriteString("- 步骤: ")
			stepNames := make([]string, len(ep.Steps))
			for j, step := range ep.Steps {
				stepNames[j] = step.ToolName
			}
			sb.WriteString(strings.Join(stepNames, " → "))
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	sb.WriteString(`请以 JSON 格式输出，包含以下字段：
{
  "topic": "知识主题（简短）",
  "content": "整合后的知识内容（详细描述常见问题和解决方案）",
  "key_points": ["要点1", "要点2", "要点3"],
  "confidence": 0.8
}

注意：
1. topic 应简洁明了，如 "nginx 容器重启处理"
2. content 应总结共性规律，而非罗列具体事件
3. key_points 提取 3-5 个关键要点
4. confidence 根据记录一致性评估，范围 0.5-1.0
`)

	response, err := c.llm.Chat(sb.String())
	if err != nil {
		return nil, fmt.Errorf("LLM call failed: %w", err)
	}

	// 解析 JSON 响应
	result, err := c.parseKnowledgeResponse(response)
	if err != nil {
		return nil, fmt.Errorf("failed to parse LLM response: %w", err)
	}

	return result, nil
}

// parseKnowledgeResponse 解析 LLM 返回的 JSON
func (c *Consolidator) parseKnowledgeResponse(response string) (*ConsolidationResult, error) {
	// 尝试提取 JSON 块
	response = strings.TrimSpace(response)

	// 处理 markdown 代码块
	if strings.HasPrefix(response, "```json") {
		response = strings.TrimPrefix(response, "```json")
		if idx := strings.LastIndex(response, "```"); idx != -1 {
			response = response[:idx]
		}
	} else if strings.HasPrefix(response, "```") {
		response = strings.TrimPrefix(response, "```")
		if idx := strings.LastIndex(response, "```"); idx != -1 {
			response = response[:idx]
		}
	}

	response = strings.TrimSpace(response)

	var result ConsolidationResult
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w, response: %s", err, truncate(response, 200))
	}

	// 验证必填字段
	if result.Topic == "" {
		return nil, fmt.Errorf("missing topic in response")
	}
	if result.Content == "" {
		return nil, fmt.Errorf("missing content in response")
	}

	// 设置默认置信度
	if result.Confidence == 0 {
		result.Confidence = 0.7
	}
	if result.Confidence < 0.5 {
		result.Confidence = 0.5
	}
	if result.Confidence > 1.0 {
		result.Confidence = 1.0
	}

	return &result, nil
}

// markEpisodesConsolidated 标记 Episodes 已整合
func (c *Consolidator) markEpisodesConsolidated(episodeIDs []string, knowledgeID string) error {
	_, err := c.db.Exec(`
		UPDATE episodes
		SET consolidated_to = $1
		WHERE id = ANY($2)
	`, knowledgeID, pq.Array(episodeIDs))
	return err
}

// truncate 截断字符串
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
