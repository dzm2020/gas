package network

import (
	"sync"
	"sync/atomic"
)

var (
	mu          sync.RWMutex
	connections map[int64]IConnection // key: 连接ID, value: 连接对象
	count       atomic.Int64          // 连接数量（原子操作，用于快速查询）
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
