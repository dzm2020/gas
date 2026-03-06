package session

import "github.com/dzm2020/gas/internal/iface"

// Factory 实现 iface.ISessionFactory，在 actor 处理消息时把 *iface.Session 包装成可写的 Session。
type Factory struct{}

// FromRaw 用 raw 与当前 ctx 构造带 transport 的 Session，供 actor 路由使用。
func (m *Factory) FromRaw(ctx iface.IContext, raw *iface.Session) iface.ISession {
	return NewWithData(raw, &transport{ctx: ctx})
}
