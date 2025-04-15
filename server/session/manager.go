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

func (m *Manager) CreateSession(sessionID string) {
	state := NewState()
	m.sessions.Store(sessionID, state)
}

func (m *Manager) GetSession(sessionID string) (*State, bool) {
	state, has := m.sessions.Load(sessionID)
	if !has {
		return nil, false
	}
	return state, true
}

func (m *Manager) GetSessionChan(sessionID string) (chan []byte, bool) {
	state, has := m.GetSession(sessionID)
	if !has {
		return nil, false
	}

	// 更新最后活跃时间
	state.LastActiveAt = time.Now()

	return state.MessageChan, true
}

func (m *Manager) CloseSession(sessionID string) {
	state, ok := m.sessions.LoadAndDelete(sessionID)
	if !ok {
		return
	}
	close(state.MessageChan)
}

func (m *Manager) CloseAllSessions() {
	m.sessions.Range(func(sessionID string, state *State) bool {
		m.sessions.Delete(sessionID)
		close(state.MessageChan)
		return true
	})
}

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
