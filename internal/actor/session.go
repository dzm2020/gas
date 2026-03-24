package actor

import (
	"strconv"

	"github.com/dzm2020/gas/api/pb"
	"github.com/dzm2020/gas/internal/iface"
)

func NewSession(ctx iface.IContext, s *pb.Session) *Session {
	return &Session{
		ctx:     ctx,
		Session: s,
	}
}

var _ iface.ISession = (*Session)(nil)

type Session struct {
	*pb.Session
	ctx iface.IContext
}

func (m *Session) GetId() int64 {
	return m.Session.GetId()
}

func (m *Session) PB() *pb.Session {
	return m.Session
}

// Send
//
//	@Description: 推送消息到Session绑定的Agent
//	@receiver m
//	@param method
//	@param bin
//	@return error
func (m *Session) Send(method string, bin []byte) error {
	msg := iface.NewActorMessage(m.ctx.ID(), m.GetAgent(), method, bin)
	if iface.EqualPid(m.GetAgent(), m.ctx.ID()) {
		return m.ctx.InvokerMessage(msg)
	}
	return m.ctx.System().SendMessage(msg)
}

// SetString 设置 Values 中的字符串（不同步到对端）。
func (m *Session) SetString(key, value string) {
	m.ensure()
	m.Values[key] = value
}

// GetString 从 Values 取字符串，不存在返回空串。
func (m *Session) GetString(key string) string {
	if m == nil || m.Values == nil {
		return ""
	}
	return m.Values[key]
}

// SetUint64 在 Values 中存 uint64
func (m *Session) SetUint64(key string, value uint64) {
	m.ensure()
	m.Values[key] = strconv.FormatUint(value, 10)
}

// GetUint64 从 Values 取并解析为 uint64，
func (m *Session) GetUint64(key string) uint64 {
	valStr, ok := m.Values[key]
	if !ok {
		return 0
	}
	val, err := strconv.ParseUint(valStr, 10, 64)
	if err != nil {
		return 0
	}
	return val
}

// SetInt64 在 Values 中存 int64（不同步，需同步请调 SyncValues）。
func (m *Session) SetInt64(key string, value int64) {
	m.ensure()
	m.Values[key] = strconv.FormatInt(value, 10)
}

// GetInt64 从 Values 取并解析为 int64，
func (m *Session) GetInt64(key string) int64 {
	valStr, ok := m.Values[key]
	if !ok {
		return 0
	}
	val, err := strconv.ParseInt(valStr, 10, 64)
	if err != nil {
		return 0
	}
	return val
}
func (m *Session) ensure() {
	if m == nil {
		return
	}
	if m.Values == nil {
		m.Values = make(map[string]string)
	}
}
