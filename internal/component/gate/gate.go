// Package gate 实现网关组件：监听 TCP/UDP、管理连接、解包协议消息，
// 并将客户端数据通过 Actor 提交给 Agent 处理；同时提供 Session/Transport 用于响应与集群下发。
package gate

import (
	"context"
	"errors"

	"github.com/dzm2020/gas/internal/component/gate/agent"
	"github.com/dzm2020/gas/internal/component/gate/codec"
	"github.com/dzm2020/gas/internal/component/gate/gateiface"
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

// 网络层回调处理
var _ network.IHandler = (*handler)(nil)

type handler struct {
	gate *Gate
}

func (h *handler) OnConnect(conn network.IConnection) error {
	return h.gate.onConnect(conn)
}

func (h *handler) OnMessage(conn network.IConnection, data []byte) (int, error) {
	return h.gate.onMessage(conn, data)
}

func (h *handler) OnClose(conn network.IConnection, err error) {
	h.gate.onClose(conn, err)
}

// Gate 网关核心：实现连接建立/消息回调/关闭，并委托 Agent Actor 处理业务。
// 连接数由 count 原子计数，超过 MaxConn 时拒绝新连接。
type Gate struct {
	network.EmptyHandler
	address       string                        // 监听地址，如 tcp://127.0.0.1:9000
	options       []network.Option              // 网络选项（KeepAlive、缓冲区等）
	factory       gateiface.AgentHandlerFactory // 创建 IHandler，用于每个连接对应一个 Agent
	maximumOfConn int64                         // 最大连接数，超过则 OnConnect 返回错误
	count         atomic.Int64                  // 当前连接数
	system        iface.ISystem
	server        network.IServer
}

// AppendOptions
//
//	@Description: AppendOptions 设置网络选项（KeepAlive、缓冲区等）。
//	@receiver g
//	@param options
func (g *Gate) AppendOptions(options ...network.Option) {
	g.options = append(g.options, options...)
}

// SetMaximumOfConn
//
//	@Description: SetMaximumOfConn 设置最大连接数，超过则 OnConnect 返回错误。
//	@receiver g
//	@param n
func (g *Gate) SetMaximumOfConn(n int64) {
	g.maximumOfConn = n
}

// GetConnectionCount
//
//	@Description: 获取连接数
//	@receiver g
//	@return int64
func (g *Gate) GetConnectionCount() int64 {
	return g.count.Load()
}

// SetSystem
//
//	@Description: SetSystem 设置 Actor 系统，需在 Start 前调用。
//	@receiver g
//	@param system
func (g *Gate) SetSystem(system iface.ISystem) {
	g.system = system
}

// SetAgentHandlerFactory
//
//	@Description: SetAgentHandlerFactory 设置每连接对应的 IHandler 工厂。
//	@receiver g
//	@param f
func (g *Gate) SetAgentHandlerFactory(f gateiface.AgentHandlerFactory) {
	g.factory = f
}

// SetAddress
//
//	@Description: 设置监听地址
//	@receiver g
//	@param address
func (g *Gate) SetAddress(address string) {
	g.address = address
}

// Start
//
//	@Description: 启动网关，注册 Session 工厂并监听。
//	@receiver g
//	@param ctx
//	@param system
//	@return err
func (g *Gate) Start(ctx context.Context) (err error) {
	g.server, err = network.NewServer(&handler{gate: g}, g.address, g.options...)
	if err != nil {
		return
	}
	g.system.SetSessionFactory(&session.Factory{})
	return g.server.Start()
}

// onConnect
//
//	@Description: 新连接时创建 Agent 并绑定 Pid。
//	@receiver g
//	@param entity
//	@return error
func (g *Gate) onConnect(entity network.IConnection) error {
	if g.maximumOfConn > 0 && g.count.Load() >= g.maximumOfConn {
		return errors.New("too many connections")
	}
	g.count.Add(1)
	pid := g.system.Spawn(agent.New(entity, g.factory()))
	entity.SetContext(pid)
	glog.Debug("网关:创建Agent", zap.Int64("entityId", entity.ID()), zap.Any("pid", pid))
	return nil
}

// onMessage
//
//	@Description: 解包字节流并投递到对应 Agent，返回已处理字节数。
//	@receiver g
//	@param entity
//	@param data
//	@return n
//	@return err
func (g *Gate) onMessage(entity network.IConnection, data []byte) (n int, err error) {
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

// process
//
//	@Description: 将 msg 投递到 entity 绑定的 Agent 的 OnData。
//	@receiver g
//	@param entity
//	@param msg
//	@return err
func (g *Gate) process(entity network.IConnection, msg *protocol.Message) (err error) {
	pid, _ := entity.Context().(*iface.Pid)
	if pid == nil {
		return ErrNoAgent
	}
	return g.system.SubmitTask(pid, func(ctx iface.IContext) error {
		a := ctx.Actor().(*agent.Agent)
		return a.OnData(msg)
	})
}

// onClose
//
//	@Description: 连接关闭时减计数并关闭绑定的 Agent 进程。
//	@receiver g
//	@param entity
//	@param wrong
func (g *Gate) onClose(entity network.IConnection, wrong error) {
	g.count.Add(-1)
	pid, _ := entity.Context().(*iface.Pid)
	if pid == nil {
		return
	}
	_ = g.system.ShutdownProcess(pid)
}

// Stop
//
//	@Description: 关闭网络服务。
//	@receiver g
//	@param ctx
//	@return error
func (g *Gate) Stop(ctx context.Context) error {
	if g.server == nil {
		return nil
	}
	g.server.Shutdown(ctx)
	return nil
}
