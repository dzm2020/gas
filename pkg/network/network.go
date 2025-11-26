package network

import (
	"fmt"
	"gas/pkg/utils/netx"
	"net"
	"sync/atomic"
	"time"
)

var (
	// 连接ID生成器（当 snowflake 未初始化时使用）
	connIDCounter atomic.Int64
)

// generateConnID 生成连接ID（当 snowflake 未初始化时使用）
func generateConnID() int64 {
	return connIDCounter.Add(1)
}

// ------------------------------ 核心接口 ------------------------------

// ICodec 协议编解码器接口（解耦TCP粘包/ UDP透传/ 自定义协议）
type ICodec interface {
	// Encode 编码：将业务消息转为网络字节流
	Encode(msg interface{}) ([]byte, error)
	// Decode 解码：从网络字节流中解析出完整业务消息
	// 返回：完整消息、剩余未解析数据、错误
	Decode(b []byte) (interface{}, int, error)
}

// IHandler 业务处理回调接口（用户需实现该接口注入业务逻辑）
type IHandler interface {
	// OnConnect 连接建立时回调（TCP：客户端连接成功；UDP：首次收到某远程地址数据包）
	OnConnect(conn IConnection) error
	// OnMessage 收到消息时回调（已解码为完整业务消息）
	OnMessage(conn IConnection, msg interface{}) error
	// OnClose 连接关闭时回调（TCP：连接断开；UDP：超时无数据或主动关闭）
	OnClose(conn IConnection, err error)
}

// IConnection 统一连接接口（TCP/UDP连接的抽象）
type IConnection interface {
	// ID 返回连接的唯一ID
	ID() int64
	// Send 发送消息（线程安全）
	Send(msg interface{}) error
	// Close 关闭连接（线程安全，可重复调用）
	Close(err error) error
	// LocalAddr 本地地址
	LocalAddr() net.Addr
	// RemoteAddr 远程地址（TCP：客户端地址；UDP：当前数据包的远程地址）
	RemoteAddr() net.Addr
	// IsClosed  是否关闭
	IsClosed() bool
	Type() ConnectionType
	Context() interface{}
	SetContext(interface{})
}

// IServer 服务器接口（TCP/UDP服务器的抽象）
type IServer interface {
	// Start 启动服务器（阻塞直到停止）
	Start() error
	// Stop 停止服务器（优雅关闭）
	Stop() error

	Addr() net.Addr
}

// ------------------------------ 基础常量与默认配置 ------------------------------
const (
	// 默认发送队列缓冲大小（避免发送阻塞）
	defaultSendChanBuf = 1024
	// 默认TCP读缓冲区大小
	defaultTCPReadBuf = 4096
	// UDP默认超时时间（无数据时自动关闭虚拟连接）
	defaultTimeout = 5 * time.Second
)

type ConnectionType int

const (
	Accept  ConnectionType = iota
	Connect ConnectionType = iota
)

// ------------------------------ 空实现（避免用户未实现接口报错） ------------------------------

// EmptyHandler 空Handler实现（用户可嵌入该结构体，只重写需要的方法）
type EmptyHandler struct{}

func (e *EmptyHandler) OnConnect(conn IConnection) error                  { return nil }
func (e *EmptyHandler) OnMessage(conn IConnection, msg interface{}) error { return nil }
func (e *EmptyHandler) OnClose(conn IConnection, err error)               {}

var _ ICodec = (*EmptyCodec)(nil)

type EmptyCodec struct{}

func (e *EmptyCodec) Encode(msg interface{}) ([]byte, error) {
	return nil, nil
}

func (e *EmptyCodec) Decode(b []byte) (interface{}, int, error) {
	return nil, 0, nil
}

func NewServer(protoAddr string, option ...Option) (IServer, error) {
	proto, addr, err := netx.ParseProtoAddr(protoAddr)
	if err != nil {
		return nil, err
	}
	switch proto {
	case "tcp", "tcp4", "tcp6":
		return NewTCPServer(proto, addr, option...), nil
	case "udp", "udp4", "udp6":
		return NewUDPServer(proto, addr, option...), nil
	default:
		return nil, fmt.Errorf("proto: %s is not support", proto)
	}
}
