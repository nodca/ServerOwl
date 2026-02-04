package memory

import (
	"sync"
	"time"
)

//短期记忆是内存中的对话历史管理，不持久化到数据库。

// 对话消息
type ChatMessage struct {
	Role      string         `json:"role"` // "user" | "assistant"
	Content   string         `json:"content"`
	Timestamp time.Time      `json:"timestamp"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

// 会话上下文
type SessionContext struct {
	SessionID  string
	UserID     string
	Messages   []ChatMessage
	LastActive time.Time
}

// 短期记忆管理
type ShortTermMemory struct {
	mu       sync.RWMutex
	sessions map[string]*SessionContext
	maxTurns int           //me==每个会话最多保留轮数 (一轮 = user + assistant)
	ttl      time.Duration //会话不活跃后清理时间
}

// NewShortTermMemory 创建短期记忆管理器
func NewShortTermMemory(maxTurns int, ttl time.Duration) *ShortTermMemory {
	stm := &ShortTermMemory{
		sessions: make(map[string]*SessionContext),
		maxTurns: maxTurns,
		ttl:      ttl,
	}

	// 启动清理协程
	go stm.cleanupLoop()

	return stm
}

// 定期清理过期会话
func (m *ShortTermMemory) cleanupLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		m.cleanupExpired()
	}
}

// 清理过期会话
func (m *ShortTermMemory) cleanupExpired() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for sessionId, session := range m.sessions {
		if now.Sub(session.LastActive) > m.ttl {
			delete(m.sessions, sessionId)
		}
	}
}

// 保存消息
func (m *ShortTermMemory) SaveMessage(sessionID, userID, role, content string, metadata map[string]any) {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, exists := m.sessions[sessionID]
	if !exists {
		session = &SessionContext{
			SessionID: sessionID,
			UserID:    userID,
			Messages:  make([]ChatMessage, 0),
		}
		m.sessions[sessionID] = session
	}

	session.Messages = append(session.Messages, ChatMessage{
		Role:      role,
		Content:   content,
		Timestamp: time.Now(),
		Metadata:  metadata,
	})
	session.LastActive = time.Now()

	// 限制消息数量 (maxTurns * 2，因为一轮包含 user + assistant)
	maxMessages := m.maxTurns * 2
	if len(session.Messages) > maxMessages {
		session.Messages = session.Messages[len(session.Messages)-maxMessages:]
	}
}

// 获取会话所有消息
func (m *ShortTermMemory) GetMessage(sessionID string) []ChatMessage {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, e := m.sessions[sessionID]
	if !e {
		return nil
	}
	// 返回副本，避免外部修改
	messages := make([]ChatMessage, len(session.Messages))
	copy(messages, session.Messages)
	return messages
}

// GetSession 获取会话上下文
func (m *ShortTermMemory) GetSession(sessionID string) *SessionContext {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, exists := m.sessions[sessionID]
	if !exists {
		return nil
	}

	// 返回副本
	copySession := &SessionContext{
		SessionID:  session.SessionID,
		UserID:     session.UserID,
		LastActive: session.LastActive,
		Messages:   make([]ChatMessage, len(session.Messages)),
	}
	copy(copySession.Messages, session.Messages)
	return copySession
}

// ClearSession 清除指定会话
func (m *ShortTermMemory) ClearSession(sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.sessions, sessionID)
}
