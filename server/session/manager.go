package session

import (
	"context"
	"time"

	"github.com/ThinkInAIXYZ/go-mcp/pkg"
)

type Manager struct {
	sessions pkg.SyncMap[*State]

	stopHeartbeat chan struct{}

	detection   func(ctx context.Context, sessionID string) error
	maxIdleTime time.Duration
}

func NewManager(detection func(ctx context.Context, sessionID string) error) *Manager {
	return &Manager{
		detection:     detection,
		stopHeartbeat: make(chan struct{}),
	}
}

func (m *Manager) SetMaxIdleTime(d time.Duration) {
	m.maxIdleTime = d
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

func (m *Manager) GetSessionSendChan(sessionID string) (chan []byte, bool) {
	state, has := m.GetSession(sessionID)
	if !has {
		return nil, false
	}

	return state.SendChan, true
}

func (m *Manager) UpdateSessionLastActiveAt(sessionID string) {
	state, ok := m.sessions.Load(sessionID)
	if !ok {
		return
	}
	state.LastActiveAt = time.Now()
}

func (m *Manager) CloseSession(sessionID string) {
	state, ok := m.sessions.LoadAndDelete(sessionID)
	if !ok {
		return
	}
	close(state.SendChan)
}

func (m *Manager) CloseAllSessions() {
	m.sessions.Range(func(sessionID string, _ *State) bool {
		// Here we load the session again to prevent concurrency conflicts with CloseSession, which may cause repeated close chan
		state, ok := m.sessions.LoadAndDelete(sessionID)
		if !ok {
			return true
		}
		close(state.SendChan)
		return true
	})
}

func (m *Manager) StartHeartbeatAndCleanInvalidSessions() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	select {
	case <-m.stopHeartbeat:
		return
	case <-ticker.C:
		now := time.Now()
		m.sessions.Range(func(sessionID string, state *State) bool {
			if m.maxIdleTime != 0 && now.Sub(state.LastActiveAt) > m.maxIdleTime {
				m.CloseSession(sessionID)
				return true
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			for i := 0; i < 3; i++ {
				if err := m.detection(ctx, sessionID); err == nil {
					return true
				}
			}
			m.CloseSession(sessionID)
			return true
		})
	}
}

func (m *Manager) StopHeartbeat() {
	close(m.stopHeartbeat)
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
