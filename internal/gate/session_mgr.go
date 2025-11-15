package gate

import (
	"sync"
	"sync/atomic"
)

type ISessionManager interface {
	Add(s *Session)
	Remove(id uint64)
	Get(id uint64) (*Session, bool)
	Count() int32
}

type SessionManager struct {
	sessions sync.Map // key: sessionID, value: *Session
	count    atomic.Int32
}

func NewSessionManager() *SessionManager {
	return &SessionManager{}
}

func (m *SessionManager) Add(s *Session) {
	m.sessions.Store(s.ID(), s)
	m.count.Add(1)
}

func (m *SessionManager) Remove(id uint64) {
	m.sessions.Delete(id)
	m.count.Add(-1)
}

func (m *SessionManager) Get(id uint64) (*Session, bool) {
	val, ok := m.sessions.Load(id)
	if !ok {
		return nil, false
	}
	return val.(*Session), true
}

func (m *SessionManager) Count() int32 {
	return m.count.Load()
}
