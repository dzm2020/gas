package gate

import (
	"sync"
	"sync/atomic"
)

type ISessionManager interface {
	Add(s *Session)
	Remove(id uint64)
	Get(id uint64) (*Session, bool)
}

type SessionManager struct {
	sessions  sync.Map // key: sessionID, value: *Session
	sessionId atomic.Uint64
}

func NewSessionManager() *SessionManager {
	return &SessionManager{}
}

func (m *SessionManager) Add(s *Session) {
	m.sessions.Store(s.ID, s)
}

func (m *SessionManager) Remove(id uint64) {
	m.sessions.Delete(id)
}

func (m *SessionManager) Get(id uint64) (*Session, bool) {
	val, ok := m.sessions.Load(id)
	if !ok {
		return nil, false
	}
	return val.(*Session), true
}
