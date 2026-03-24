package network

import (
	"sync"
	"sync/atomic"
)

// ConnManager 按服务器实例管理连接，避免多 Server 共用包级全局导致互相干扰。
// 每个 baseServer 持有一个 ConnManager，创建连接时传入，关闭时从本实例的 manager 移除。
type ConnManager struct {
	mu          sync.RWMutex
	connections map[int64]IConnection
	count       atomic.Int64

	udpMu          sync.RWMutex
	udpConnections map[string]*UDPConnection
}

// NewConnManager 创建连接管理器，供 baseServer 使用。
func NewConnManager() *ConnManager {
	return &ConnManager{
		connections:    make(map[int64]IConnection),
		udpConnections: make(map[string]*UDPConnection),
	}
}

func (m *ConnManager) Add(conn IConnection) {
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

func (m *ConnManager) Remove(conn IConnection) {
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

func (m *ConnManager) Get(id int64) IConnection {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.connections[id]
}

func (m *ConnManager) ConnectionCount() int64 {
	return m.count.Load()
}

// GetAll 返回当前所有连接在单次读锁内的快照副本；持锁时间短，连接规模通常有限，语义为「某一时刻的一致性视图」。
func (m *ConnManager) GetAll() []IConnection {
	m.mu.RLock()
	defer m.mu.RUnlock()
	conns := make([]IConnection, 0, len(m.connections))
	for _, conn := range m.connections {
		if conn != nil {
			conns = append(conns, conn)
		}
	}
	return conns
}

func (m *ConnManager) Range(fn func(conn IConnection) bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, conn := range m.connections {
		if !fn(conn) {
			break
		}
	}
}

func (m *ConnManager) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connections = make(map[int64]IConnection)
	m.count.Store(0)
}

// AddUDP 添加 UDP 虚拟连接，若 key 已存在则返回已存在的连接和 false。
func (m *ConnManager) AddUDP(connKey string, conn *UDPConnection) (*UDPConnection, bool) {
	if conn == nil {
		return nil, false
	}
	m.udpMu.Lock()
	defer m.udpMu.Unlock()
	if existing, exists := m.udpConnections[connKey]; exists {
		return existing, false
	}
	m.udpConnections[connKey] = conn
	return conn, true
}

func (m *ConnManager) RemoveUDP(connKey string) {
	m.udpMu.Lock()
	defer m.udpMu.Unlock()
	delete(m.udpConnections, connKey)
}

func (m *ConnManager) GetUDP(connKey string) (*UDPConnection, bool) {
	m.udpMu.RLock()
	defer m.udpMu.RUnlock()
	conn, ok := m.udpConnections[connKey]
	return conn, ok
}
