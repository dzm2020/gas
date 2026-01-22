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
	// 创建连接池
	poolSize := cfg.PoolSize
	if poolSize <= 0 {
		poolSize = 10 // 默认大小
	}
	pool := &ConnPool{
		pool:    make(chan *nats.Conn, poolSize),
		servers: servers,
		opts:    natsOpts,
		size:    poolSize,
	}

	// 预创建连接池中的连接
	for i := 0; i < poolSize; i++ {
		conn, err := pool.createConn()
		if err != nil {
			// 如果预创建失败，继续尝试，连接会在使用时创建
			continue
		}
		pool.put(conn)
	}
	return pool
}

// ConnPool 连接池
type ConnPool struct {
	stopper.Stopper
	pool    chan *nats.Conn
	mu      sync.RWMutex
	servers string
	opts    []nats.Option
	size    int
}

// get 从连接池获取连接
func (p *ConnPool) get() (*nats.Conn, error) {
	if p.IsStop() {
		return nil, errors.New("连接池已关闭")
	}

	select {
	case conn := <-p.pool:
		// 检查连接是否有效
		if conn != nil && !conn.IsClosed() {
			return conn, nil
		}
		// 连接已关闭，创建新连接
		return p.createConn()
	default:
		// 连接池为空，创建新连接
		return p.createConn()
	}
}

// put 归还连接到连接池
func (p *ConnPool) put(conn *nats.Conn) {
	if p.IsStop() {
		conn.Close()
		return
	}
	select {
	case p.pool <- conn:
	default:
		conn.Close()
	}
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
	// 关闭所有连接
	for {
		select {
		case conn := <-p.pool:
			if conn != nil && !conn.IsClosed() {
				conn.Close()
			}
		default:
			close(p.pool)
			return
		}
	}
}
