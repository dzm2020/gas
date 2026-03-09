package nats

import (
	"errors"
	"strings"
	"sync"

	"github.com/dzm2020/gas/pkg/lib/stopper"
	"github.com/nats-io/nats.go"
)

func NewPool(cfg *Config) *ConnPool {
	servers := strings.Join(cfg.Servers, ",")
	natsOpts := toOptions(cfg)
	poolSize := cfg.PoolSize
	if poolSize <= 0 {
		poolSize = 10
	}
	pool := &ConnPool{
		conns:   make([]*nats.Conn, poolSize),
		servers: servers,
		opts:    natsOpts,
		size:    poolSize,
	}
	for i := 0; i < poolSize; i++ {
		conn, err := pool.createConn()
		if err != nil {
			continue
		}
		pool.conns[i] = conn
	}
	return pool
}

// hashSubject 对 subject 做哈希，相同 subject 始终映射到同一连接下标，保证 Publish/Request 顺序性。
func hashSubject(subject string) uint32 {
	var h uint32 = 0
	for _, c := range subject {
		h = h*31 + uint32(c)
	}
	return h
}

// ConnPool 按 subject 固定到同一连接的池，保证同一 subject 的 Publish 与 Request 顺序。
type ConnPool struct {
	stopper.Stopper
	conns   []*nats.Conn
	mu      sync.Mutex
	servers string
	opts    []nats.Option
	size    int
}

// getConnBySubject 返回该 subject 固定绑定的连接，同一 subject 始终用同一 conn，保证消息顺序。
func (p *ConnPool) getConnBySubject(subject string) (*nats.Conn, error) {
	if p.IsStop() {
		return nil, errors.New("连接池已关闭")
	}
	idx := uint32(0)
	if p.size > 0 {
		idx = hashSubject(subject) % uint32(p.size)
	}
	p.mu.Lock()
	conn := p.conns[idx]
	if conn == nil || conn.IsClosed() {
		var err error
		conn, err = p.createConn()
		if err != nil {
			p.mu.Unlock()
			return nil, err
		}
		p.conns[idx] = conn
	}
	p.mu.Unlock()
	return conn, nil
}

// createConn 创建新连接
func (p *ConnPool) createConn() (*nats.Conn, error) {
	return nats.Connect(p.servers, p.opts...)
}

// close 关闭连接池中的所有连接
func (p *ConnPool) close() {
	if !p.Stop() {
		return
	}
	p.mu.Lock()
	for i, conn := range p.conns {
		if conn != nil && !conn.IsClosed() {
			conn.Close()
		}
		p.conns[i] = nil
	}
	p.mu.Unlock()
}
