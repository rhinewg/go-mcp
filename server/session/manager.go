package session

import (
	"time"

	"github.com/ThinkInAIXYZ/go-mcp/pkg"
)

type Manager struct {
	sessions pkg.SyncMap[*State]
}

func NewManager() *Manager {
	return &Manager{}
}

// CreateSession 创建新的会话
func (m *Manager) CreateSession() (string, chan []byte) {
	state := NewState()

	m.sessions.Store(state.ID, state)

	return state.ID, state.MessageChan
}

// GetSession 获取会话状态
func (m *Manager) GetSession(sessionID string) (*State, bool) {
	state, has := m.sessions.Load(sessionID)
	if !has {
		return nil, false
	}
	return state, true
}

// GetSessionChan 获取会话消息通道
func (m *Manager) GetSessionChan(sessionID string) (chan []byte, bool) {
	state, has := m.GetSession(sessionID)
	if !has {
		return nil, false
	}

	// 更新最后活跃时间
	state.LastActiveAt = time.Now()

	return state.MessageChan, true
}

// CloseSession 关闭并删除会话
func (m *Manager) CloseSession(sessionID string) {
	state, ok := m.sessions.LoadAndDelete(sessionID)
	if !ok {
		return
	}
	close(state.MessageChan)
}

// CloseAllSessions 关闭所有会话
func (m *Manager) CloseAllSessions() {
	m.sessions.Range(func(sessionID string, state *State) bool {
		// 删除会话
		m.sessions.Delete(sessionID)
		// 关闭消息通道
		close(state.MessageChan)
		return true
	})
}

// CycleCleanSessions 清理过期会话
func (m *Manager) CycleCleanSessions(maxIdleTime time.Duration) {
	now := time.Now()
	var expiredCount int64

	m.sessions.Range(func(sessionID string, state *State) bool {
		if now.Sub(state.LastActiveAt) > maxIdleTime {
			m.CloseSession(sessionID)
			expiredCount++
		}
		return true
	})
}

// RangeSessions 遍历所有会话
func (m *Manager) RangeSessions(f func(sessionID string, state *State) bool) {
	m.sessions.Range(f)
}

func (m *Manager) IsEmpty() bool {
	isEmpty := true
	m.sessions.Range(func(string, *State) bool {
		isEmpty = false
		return false
	})
	return isEmpty
}
