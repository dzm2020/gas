// Package gate 实现网关组件：监听 TCP/UDP、管理连接、解包协议消息，
// 并将客户端数据通过 Actor 提交给 Agent 处理；同时提供 Session/Transport 用于响应与集群下发。
package gate

import (
	"context"
	"errors"

	"github.com/dzm2020/gas/internal/component/gate/agent"
	"github.com/dzm2020/gas/internal/component/gate/codec"
	"github.com/dzm2020/gas/internal/component/gate/protocol"
	"github.com/dzm2020/gas/internal/component/gate/session"
	"github.com/dzm2020/gas/internal/iface"
	"github.com/dzm2020/gas/pkg/glog"
	"github.com/dzm2020/gas/pkg/network"

	"go.uber.org/atomic"
	"go.uber.org/zap"
)

var (
	// ErrNoAgent 表示连接上未绑定 Agent Pid，无法投递消息（如连接尚未完成 OnConnect 或已异常）。
	ErrNoAgent = errors.New("gate: no agent bound to connection")
)

// Gate 网关核心：实现连接建立/消息回调/关闭，并委托 Agent Actor 处理业务。
// 连接数由 count 原子计数，超过 MaxConn 时拒绝新连接。
type Gate struct {
	network.EmptyHandler
	Address string           // 监听地址，如 tcp://127.0.0.1:9000
	Options []network.Option // 网络选项（KeepAlive、缓冲区等）
	Factory agent.Factory    // 创建 IHandler，用于每个连接对应一个 Agent
	MaxConn int64            // 最大连接数，超过则 OnConnect 返回错误
	server  network.IServer
	count   atomic.Int64 // 当前连接数
	system  iface.ISystem
}

// Start
//
//	@Description: 启动网关：注入 system、创建网络服务、注册 Session 工厂并启动监听。
//	@receiver g
//	@param ctx
//	@param system
//	@return err
func (g *Gate) Start(ctx context.Context, system iface.ISystem) (err error) {
	g.system = system
	g.server, err = network.NewServer(g, g.Address, g.Options...)
	if err != nil {
		return
	}
	system.SetSessionFactory(&session.Factory{})
	return g.server.Start()
}

// OnConnect 新连接建立时：检查 MaxConn，计数+1，创建并 Spawn Agent，将 Agent 的 Pid 绑定到 entity.Context。
func (g *Gate) OnConnect(entity network.IConnection) error {
	if g.count.Load() > g.MaxConn {
		return errors.New("too many connections")
	}
	g.count.Add(1)
	pid := g.system.Spawn(agent.New(entity, g.Factory()))
	entity.SetContext(pid)
	glog.Debug("网关:创建Agent", zap.Int64("entityId", entity.ID()), zap.Any("pid", pid))
	return nil
}

// OnMessage 收到原始字节：循环解包，每解出一个完整 Message 则提交给 process 投递到对应 Agent 处理；返回已处理的字节数。
func (g *Gate) OnMessage(entity network.IConnection, data []byte) (n int, err error) {
	var msg *protocol.Message
	var processN int
	for len(data) > 0 {
		msg, processN, err = codec.Decode(data)
		if err != nil {
			return
		}
		if processN == 0 {
			return
		}
		n += processN
		data = data[processN:]
		if err = g.process(entity, msg); err != nil {
			return
		}
	}
	return
}

// process 从 entity 取出绑定的 Agent Pid，将 msg 通过 SubmitTask 投递到该 Agent 的 OnData 处理；若未绑定 Pid 则返回 ErrNoAgent。
func (g *Gate) process(entity network.IConnection, msg *protocol.Message) (err error) {
	pid, _ := entity.Context().(*iface.Pid)
	if pid == nil {
		return ErrNoAgent
	}
	return g.system.SubmitTask(pid, func(ctx iface.IContext) error {
		a := ctx.Actor().(*agent.Agent)
		return a.OnData(ctx, msg)
	})
}

// OnClose 连接关闭时：连接数-1，若 entity 上绑定了 Agent Pid 则关闭该进程（主动关闭时由 Agent.Shutdown 触发，被动关闭时由网络层触发）。
func (g *Gate) OnClose(entity network.IConnection, wrong error) {
	g.count.Add(-1)
	pid, _ := entity.Context().(*iface.Pid)
	if pid == nil {
		return
	}
	_ = g.system.ShutdownProcess(pid)
}

// Stop 关闭网络服务。
func (g *Gate) Stop(ctx context.Context) error {
	if g.server == nil {
		return nil
	}
	g.server.Shutdown(ctx)
	return nil
}
