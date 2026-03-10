// 本文件实现 iface.ISessionFactory，由 Gate 注册到 System，在 actor 处理会话消息时把 *pb.Session 包装成带 Transport 的 Session。
package session

import (
	"github.com/dzm2020/gas/internal/iface"
	"github.com/dzm2020/gas/internal/pb"
)

// Factory 实现 iface.ISessionFactory，将 *pb.Session 与 ctx 包装成可 Response/Push/Close 的 Session。
type Factory struct{}

// FromRaw 用 raw 与当前 ctx 构造带 transport 的 Session（transport 目标为 raw.Agent），供 actor 路由使用。
func (m *Factory) FromRaw(ctx iface.IContext, raw *pb.Session) iface.ISession {
	return New(raw, ctx)
}
