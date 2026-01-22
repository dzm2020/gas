package network

import (
	"sync"
	"sync/atomic"
)

var (
	mu          sync.RWMutex
	connections = make(map[int64]IConnection) // key: 连接ID, value: 连接对象
	count       atomic.Int64                   // 连接数量（原子操作，用于快速查询）

	// UDP连接管理
	udpConnections = make(map[string]*UDPConnection) // key: 地址字符串, value: UDP连接对象
	udpConnMutex   sync.RWMutex                      // 保护udpConnections并发
)

func AddConnection(conn IConnection) {
	if conn == nil {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	if _, exists := connections[conn.ID()]; !exists {
		connections[conn.ID()] = conn
		count.Add(1)
	}
}

func RemoveConnection(conn IConnection) {
	if conn == nil {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	if _, exists := connections[conn.ID()]; exists {
		delete(connections, conn.ID())
		count.Add(-1)
	}
}

func GetConnection(id int64) IConnection {
	mu.RLock()
	defer mu.RUnlock()
	return connections[id]
}

func ConnectionCount() int64 {
	return count.Load()
}

func GetAllConnections() []IConnection {
	mu.RLock()
	defer mu.RUnlock()
	conns := make([]IConnection, 0, len(connections))
	for _, conn := range connections {
		conns = append(conns, conn)
	}
	return conns
}

func RangeConnections(fn func(conn IConnection) bool) {
	mu.RLock()
	defer mu.RUnlock()
	for _, conn := range connections {
		if !fn(conn) {
			break
		}
	}
}

func ClearConnections() {
	mu.Lock()
	defer mu.Unlock()
	connections = make(map[int64]IConnection)
	count.Store(0)
}

// ------------------------------ UDP连接管理 ------------------------------

// AddUDPConnection 添加UDP连接，如果已存在则返回已存在的连接和false，否则添加并返回true
func AddUDPConnection(connKey string, conn *UDPConnection) (*UDPConnection, bool) {
	if conn == nil {
		return nil, false
	}
	udpConnMutex.Lock()
	defer udpConnMutex.Unlock()
	if existing, exists := udpConnections[connKey]; exists {
		return existing, false
	}
	udpConnections[connKey] = conn
	return conn, true
}

// RemoveUDPConnection 移除UDP连接
func RemoveUDPConnection(connKey string) {
	udpConnMutex.Lock()
	defer udpConnMutex.Unlock()
	delete(udpConnections, connKey)
}

// GetUDPConnection 获取UDP连接
func GetUDPConnection(connKey string) (*UDPConnection, bool) {
	udpConnMutex.RLock()
	defer udpConnMutex.RUnlock()
	conn, ok := udpConnections[connKey]
	return conn, ok
}
