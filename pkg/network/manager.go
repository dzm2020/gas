package network

import (
	"sync"
	"sync/atomic"
)

var (
	// 全局连接管理器
	globalManager = &Manager{
		connections: make(map[int64]IConnection),
	}
)

// Manager 连接管理器，管理所有连接（TCP和UDP）
type Manager struct {
	mu          sync.RWMutex
	connections map[int64]IConnection // key: 连接ID, value: 连接对象
	count       atomic.Int64          // 连接数量（原子操作，用于快速查询）
}

func (m *Manager) Add(conn IConnection) {
	if conn == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.connections[conn.ID()]; !exists {
		m.connections[conn.ID()] = conn
		m.count.Add(1)
	}
}

func (m *Manager) Remove(conn IConnection) {
	if conn == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.connections[conn.ID()]; exists {
		delete(m.connections, conn.ID())
		m.count.Add(-1)
	}
}

func (m *Manager) Get(id int64) IConnection {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.connections[id]
}

func (m *Manager) Count() int64 {
	return m.count.Load()
}

func (m *Manager) GetAll() []IConnection {
	m.mu.RLock()
	defer m.mu.RUnlock()
	conns := make([]IConnection, 0, len(m.connections))
	for _, conn := range m.connections {
		conns = append(conns, conn)
	}
	return conns
}

func (m *Manager) Range(fn func(conn IConnection) bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, conn := range m.connections {
		if !fn(conn) {
			break
		}
	}
}

func (m *Manager) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connections = make(map[int64]IConnection)
	m.count.Store(0)
}

func AddConnection(conn IConnection)                  { globalManager.Add(conn) }
func RemoveConnection(conn IConnection)               { globalManager.Remove(conn) }
func GetConnection(id int64) IConnection              { return globalManager.Get(id) }
func ConnectionCount() int64                          { return globalManager.Count() }
func GetAllConnections() []IConnection                { return globalManager.GetAll() }
func RangeConnections(fn func(conn IConnection) bool) { globalManager.Range(fn) }
func ClearConnections()                               { globalManager.Clear() }
