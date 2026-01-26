package network

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"time"
)

var (
	// connIDCounter 连接ID生成器（当 snowflake 未初始化时使用）
	// 使用原子操作保证并发安全，从1开始递增
	connIDCounter atomic.Int64
)

// genConnID 生成连接ID（当 snowflake 未初始化时使用）
// 返回一个全局唯一的递增ID，使用原子操作保证并发安全
func genConnID() int64 {
	return connIDCounter.Add(1)
}

// ------------------------------ 核心接口 ------------------------------

// ICodec 协议编解码器接口
// 用于解耦TCP粘包处理、UDP透传、自定义协议等场景
// 用户需要根据实际业务需求实现该接口
type ICodec interface {
	// Encode 编码：将业务消息转为网络字节流
	// 参数 msg 可以是任意类型的业务消息
	// 返回编码后的字节流和可能的错误
	Encode(msg interface{}) ([]byte, error)
	// Decode 解码：从网络字节流中解析出完整业务消息
	// 参数 b 是接收到的字节流（可能包含多个消息或部分消息）
	// 返回值：
	//   - interface{}: 解析出的完整业务消息，如果数据不完整返回 nil
	//   - int: 已解析的字节数，用于从缓冲区中移除已处理的数据
	//   - error: 解析错误，如果数据不完整应该返回 nil
	Decode(b []byte) (interface{}, int, error)
}

// IHandler 业务处理回调接口
// 用户需实现该接口注入业务逻辑，处理连接的生命周期事件和消息
type IHandler interface {
	// OnConnect 连接建立时回调
	// TCP：客户端连接成功时调用
	// UDP：首次收到某远程地址数据包时调用（创建虚拟连接）
	// 如果返回错误，连接将被关闭
	OnConnect(conn IConnection) error
	// OnMessage 收到消息时回调
	// 消息已经过解码器解码，msg 是完整的业务消息
	// 如果返回错误，连接将被关闭
	OnMessage(conn IConnection, msg interface{}) error
	// OnClose 连接关闭时回调
	// TCP：连接断开时调用
	// UDP：超时无数据或主动关闭时调用
	// err 表示关闭原因，可能为 nil（正常关闭）
	OnClose(conn IConnection, err error)
}

// IConnection 统一连接接口（TCP/UDP连接的抽象）
// 提供了统一的连接操作接口，屏蔽了不同协议（TCP/UDP/WebSocket）的差异
type IConnection interface {
	// ID 返回连接的唯一ID
	ID() int64
	// Send 发送消息（线程安全）
	// 消息会经过编码器编码后发送，如果发送队列已满会返回错误
	Send(msg interface{}) error
	// Close 关闭连接（线程安全，可重复调用）
	// 参数 err 表示关闭原因，可以为 nil
	Close(err error) error
	// LocalAddr 返回本地地址字符串
	LocalAddr() string
	// RemoteAddr 返回远程地址字符串
	// TCP：客户端地址；UDP：当前数据包的远程地址
	RemoteAddr() string
	// IsStop 检查连接是否已关闭
	IsStop() bool
	// Type 返回连接类型（Accept 或 Connect）
	Type() ConnType
	// Context 返回连接的上下文对象，可用于存储用户自定义数据
	Context() interface{}
	// SetContext 设置连接的上下文对象
	SetContext(interface{})
	// SetReadBuffer 设置读缓冲区大小（字节数）
	SetReadBuffer(bytes int) error
	// SetWriteBuffer 设置写缓冲区大小（字节数）
	SetWriteBuffer(bytes int) error
	// SetLinger 设置 TCP 连接的 SO_LINGER 选项
	// enable: 是否启用；sec: 延迟关闭时间（秒）
	SetLinger(enable bool, sec int) error
	// SetNoDelay 设置 TCP 连接的 TCP_NODELAY 选项（禁用 Nagle 算法）
	SetNoDelay(noDelay bool) error
	// SetTCPKeepAlive 设置 TCP 连接的 KeepAlive 选项
	// enable: 是否启用；period: KeepAlive 检测周期
	SetTCPKeepAlive(enable bool, period time.Duration) error
}

// IServer 服务器接口（TCP/UDP/WebSocket服务器的抽象）
// 提供了统一的服务器操作接口，屏蔽了不同协议的差异
type IServer interface {
	// Start 启动服务器
	Start() error
	// Shutdown 优雅关闭服务器
	// ctx 用于控制关闭的超时时间，如果超时则强制关闭
	Shutdown(ctx context.Context)
	// Addr 返回服务器的监听地址字符串
	Addr() string
}

// ------------------------------ 基础常量与默认配置 ------------------------------

// ConnType 连接类型
// 用于标识连接是服务器接受的连接还是客户端主动建立的连接
type ConnType = int

const (
	Accept  ConnType = iota // Accept 表示服务器接受的连接（服务端）
	Connect ConnType = iota // Connect 表示客户端主动建立的连接（客户端）
)

// ------------------------------ 空实现（避免用户未实现接口报错） ------------------------------

// EmptyHandler 空Handler实现
// 用户可嵌入该结构体，只重写需要的方法，避免实现所有接口方法
// 所有方法都是空实现，不会产生任何副作用
type EmptyHandler struct{}

// OnConnect 连接建立时的空实现
func (e *EmptyHandler) OnConnect(conn IConnection) error { return nil }

// OnMessage 收到消息时的空实现
func (e *EmptyHandler) OnMessage(conn IConnection, msg interface{}) error { return nil }

// OnClose 连接关闭时的空实现
func (e *EmptyHandler) OnClose(conn IConnection, err error) {}

// 确保 EmptyCodec 实现了 ICodec 接口
var _ ICodec = (*EmptyCodec)(nil)

// EmptyCodec 空编解码器实现
// 所有方法都是空实现，不会进行任何编解码操作
// 主要用于测试或不需要编解码的场景
type EmptyCodec struct{}

// Encode 编码的空实现，总是返回 nil
func (e *EmptyCodec) Encode(msg interface{}) ([]byte, error) {
	return nil, nil
}

// Decode 解码的空实现，总是返回 nil
// 参数 n 表示已解析的字节数，这里返回 0
func (e *EmptyCodec) Decode(b []byte) (interface{}, int, error) {
	return nil, 0, nil
}

// NewServer 创建网络服务器
// 根据协议地址自动创建对应类型的服务器（TCP/UDP/WebSocket）
//
// 参数:
//   - handler: 业务处理回调接口，不能为 nil
//   - protoAddr: 协议地址，格式为 "协议://地址[:端口][/路径]"
//     例如: "tcp://127.0.0.1:8080", "udp://0.0.0.0:9999", "ws://0.0.0.0:8080/ws"
//   - option: 可选的配置项，用于自定义服务器行为
//
// 返回值:
//   - IServer: 创建的服务器实例
//   - error: 如果 handler 为 nil 或协议不支持，返回错误
//
// 示例:
//
//	server, err := NewServer(handler, "tcp://127.0.0.1:8080", WithCodec(codec))
//	if err != nil {
//	    log.Fatal(err)
//	}
//	server.Start()
func NewServer(handler IHandler, protoAddr string, option ...Option) (IServer, error) {
	if handler == nil {
		return nil, errors.New("handler is nil")
	}

	network, address := parseProtoAddr(protoAddr)
	path, address := parseWsAddr(address)
	base := newBaseServer(network, address, handler, option...)
	switch network {
	case "tcp", "tcp4", "tcp6":
		return NewTCPServer(base), nil
	case "udp", "udp4", "udp6":
		return NewUDPServer(base), nil
	case "ws", "wss":
		return NewWebSocketServer(base, path), nil
	default:
		return nil, ErrUnsupportedProtocol(network)
	}
}

// parseWsAddr 解析 WebSocket 地址，分离路径和地址部分
// 例如: "127.0.0.1:8080/ws" -> ("/ws", "127.0.0.1:8080")
// 如果没有路径，默认返回 "/"
//
// 参数:
//   - address: 包含路径的地址字符串
//
// 返回值:
//   - string: WebSocket 路径（如 "/ws"）
//   - string: 地址部分（如 "127.0.0.1:8080"）
func parseWsAddr(address string) (string, string) {
	// 解析地址和路径
	path := "/"
	if idx := strings.Index(address, "/"); idx >= 0 {
		path = address[idx:]
		address = address[:idx]
	}
	return path, address
}

// parseProtoAddr 解析协议地址，分离协议类型和地址部分
// 例如: "tcp://127.0.0.1:8080" -> ("tcp", "127.0.0.1:8080")
//
// 参数:
//   - addr: 完整的协议地址字符串，格式为 "协议://地址"
//
// 返回值:
//   - string: 协议类型（如 "tcp", "udp", "ws"）
//   - string: 地址部分（如 "127.0.0.1:8080"）
//     如果格式不正确，返回空字符串
func parseProtoAddr(addr string) (string, string) {
	data := strings.Split(addr, "://")
	if len(data) != 2 {
		return "", ""
	}
	return data[0], data[1]
}
