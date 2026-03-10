// 限流插件：基于令牌桶，支持按连接限流或按消息 ID 限流，使用 golang.org/x/time/rate。
package middleware

import (
	"errors"
	"sync"

	"github.com/dzm2020/gas/internal/component/gate/gateiface"
	"github.com/dzm2020/gas/internal/component/gate/protocol"
	"golang.org/x/time/rate"
)

var (
	_ gateiface.IMiddleware = (*RateLimit)(nil)
)

var ErrRateLimitExceeded = errors.New("middleware: rate limit exceeded")

// RateLimitMode 限流维度：按整条连接，或按（单连接内）消息 ID。
type RateLimitMode int

const (
	// RateLimitByConnection 按连接限流：每个中间件实例对应一个桶，限制该连接整体频率。
	RateLimitByConnection RateLimitMode = iota
	// RateLimitByMessageID 按消息 ID 限流：限制的是**单条连接上**每个 Cmd/Act 的频率；实例必须每连接一个，不可多连接共享。
	RateLimitByMessageID
)

// RateLimit 限流中间件：使用官方 rate.Limiter（令牌桶），支持按连接或按消息 ID。
type RateLimit struct {
	mode      RateLimitMode
	limiter   *rate.Limiter // 按连接时使用
	byID      map[uint16]*rate.Limiter
	limit     rate.Limit
	burst     int
	mu        sync.Mutex
	messageId uint16
}

// NewRateLimitForConnection 创建按连接限流的中间件。每个连接（Agent）应使用独立实例。
// limit 为每秒令牌数（如 rate.PerSecond(100)），burst 为桶容量。
func NewRateLimitForConnection(limit rate.Limit, burst int) *RateLimit {
	if burst <= 0 {
		burst = 1
	}
	return &RateLimit{
		mode:    RateLimitByConnection,
		limiter: rate.NewLimiter(limit, burst),
		limit:   limit,
		burst:   burst,
	}
}

// NewRateLimitForMessageID 创建按消息 ID 限流的中间件，限制的是**单条连接上**某个特定消息 ID 的频率。
func NewRateLimitForMessageID(limit rate.Limit, burst int, messageId uint16) *RateLimit {
	if burst <= 0 {
		burst = 1
	}
	return &RateLimit{
		mode:      RateLimitByMessageID,
		byID:      make(map[uint16]*rate.Limiter),
		limit:     limit,
		burst:     burst,
		messageId: messageId,
	}
}

func (r *RateLimit) AfterDecode(_ gateiface.IAgent, msg *protocol.Message) (*protocol.Message, error) {
	if msg == nil {
		return nil, nil
	}
	var lim *rate.Limiter
	if r.mode == RateLimitByConnection {
		lim = r.limiter
	} else {
		lim = r.limiterForID(msg.ID())
		if lim == nil {
			return msg, nil
		}
	}
	if !lim.Allow() {
		return nil, ErrRateLimitExceeded
	}
	return msg, nil
}

func (r *RateLimit) limiterForID(id uint16) *rate.Limiter {
	r.mu.Lock()
	defer r.mu.Unlock()
	if l, ok := r.byID[id]; ok {
		return l
	}
	if r.messageId != id {
		return nil
	}
	l := rate.NewLimiter(r.limit, r.burst)
	r.byID[id] = l
	return l
}

func (r *RateLimit) BeforeEncode(_ gateiface.IAgent, msg *protocol.Message) (*protocol.Message, error) {
	return msg, nil
}
