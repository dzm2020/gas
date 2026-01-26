package network

import (
	"context"
	"errors"
	"io"
	"net"
	"time"

	"github.com/dzm2020/gas/pkg/glog"
	"github.com/dzm2020/gas/pkg/lib/buffer"
	"github.com/dzm2020/gas/pkg/lib/netutil"
	"github.com/dzm2020/gas/pkg/lib/stopper"
	"go.uber.org/zap"
)

// newBaseConn 初始化基类连接
// 创建并初始化一个 baseConn 实例，设置所有必要的字段和缓冲区
//
// 参数:
//   - ctx: 上下文，用于控制连接的生命周期
//   - network: 网络类型（如 "tcp", "udp", "ws"）
//   - typ: 连接类型（Accept 或 Connect）
//   - conn: 底层的网络连接
//   - remoteAddr: 远程地址
//   - options: 连接选项配置
//
// 返回值:
//   - *baseConn: 初始化完成的连接基类实例
func newBaseConn(ctx context.Context, network string,
	typ ConnType, conn net.Conn, remoteAddr net.Addr, options *Options) *baseConn {
	bc := &baseConn{
		id:          genConnID(),
		network:     network,
		options:     options,
		conn:        conn,
		remoteAddr:  remoteAddr,
		lastActive:  time.Now(),
		typ:         typ,
		sendChan:    make(chan interface{}, options.SendChanSize),
		writeBuffer: buffer.New(options.SendBufferSize),
		readBuffer:  buffer.New(options.ReadBufSize),
	}
	bc.ctx, bc.cancel = context.WithCancel(ctx)
	glog.Info("新建网络连接", zap.Int64("connectionId", bc.ID()),
		zap.String("network", bc.Network()),
		zap.Int("typ", bc.Type()),
		zap.String("localAddr", bc.LocalAddr()),
		zap.String("remoteAddr", bc.RemoteAddr()))
	return bc
}

// baseConn 连接基类，包含所有连接的通用逻辑
// 所有具体连接类型（TCP/UDP/WebSocket）都嵌入此结构体
// 提供了连接管理、消息发送、心跳检测等通用功能
type baseConn struct {
	stopper.Stopper              // 提供停止状态管理
	id          int64            // 连接唯一ID，全局唯一
	network     string           // 网络类型（"tcp", "udp", "ws" 等）
	handler     IHandler         // 业务处理回调接口
	options     *Options         // 连接选项配置
	conn        net.Conn         // 底层的原生网络连接
	lastActive  time.Time        // 最后活动时间，用于心跳超时检测
	typ         ConnType         // 连接类型（Accept 或 Connect）
	user        interface{}      // 用户自定义上下文数据（通过 SetContext 设置）
	sendChan    chan interface{} // 发送消息的通道，用于异步发送
	ctx         context.Context  // 连接的上下文，用于控制生命周期
	cancel      context.CancelFunc // 取消函数，用于关闭连接
	remoteAddr  net.Addr         // 远程地址
	writeBuffer buffer.IBuffer   // 写缓冲区，用于批量写入优化
	readBuffer  buffer.IBuffer   // 读缓冲区，用于处理粘包
}

// ID 返回连接的唯一ID
// 该ID在连接创建时生成，全局唯一
func (b *baseConn) ID() int64 {
	return b.id
}

// Type 返回连接类型
// 返回 Accept（服务端接受的连接）或 Connect（客户端主动建立的连接）
func (b *baseConn) Type() ConnType {
	return b.typ
}

// Network 返回网络类型
// 返回连接使用的网络协议类型，如 "tcp", "udp", "ws" 等
func (b *baseConn) Network() string {
	return b.network
}

// LocalAddr 返回本地地址字符串
// 格式为 "IP:端口"，例如 "127.0.0.1:8080"
func (b *baseConn) LocalAddr() string {
	return b.conn.LocalAddr().String()
}

// RemoteAddr 返回远程地址字符串
// 格式为 "IP:端口"，例如 "192.168.1.100:54321"
func (b *baseConn) RemoteAddr() string {
	return b.remoteAddr.String()
}

// Context 返回连接的上下文对象
// 注意：这里返回的是 context.Context，不是用户自定义数据
// 如需获取用户自定义数据，应使用其他方式
func (b *baseConn) Context() interface{} {
	return b.ctx
}

// SetContext 设置用户自定义上下文数据
// 用于在连接上存储业务相关的数据，如用户信息、会话信息等
// 注意：参数名 ctx 容易与 context.Context 混淆，实际存储的是用户数据
func (b *baseConn) SetContext(ctx interface{}) {
	b.user = ctx
}
// SetReadBuffer 设置读缓冲区大小
// 参数 bytes 指定缓冲区大小（字节数）
// 仅对 TCP 连接有效，UDP 和 WebSocket 可能不支持
func (b *baseConn) SetReadBuffer(bytes int) error {
	return netutil.SetRcvBuffer(b.conn, bytes)
}

// SetWriteBuffer 设置写缓冲区大小
// 参数 bytes 指定缓冲区大小（字节数）
// 仅对 TCP 连接有效，UDP 和 WebSocket 可能不支持
func (b *baseConn) SetWriteBuffer(bytes int) error {
	return netutil.SetSndBuffer(b.conn, bytes)
}

// SetLinger 设置 TCP 连接的 SO_LINGER 选项
// 控制关闭连接时的行为：是否等待未发送的数据发送完成
// 参数:
//   - enable: 是否启用 SO_LINGER
//   - sec: 延迟关闭时间（秒），仅在 enable=true 时有效
// 仅对 TCP 连接有效
func (b *baseConn) SetLinger(enable bool, sec int) error {
	return netutil.SetTCPLinger(b.conn, enable, sec)
}

// SetNoDelay 设置 TCP 连接的 TCP_NODELAY 选项
// 禁用 Nagle 算法，减少延迟但可能增加网络包数量
// 参数 noDelay: true 表示禁用 Nagle 算法（立即发送），false 表示启用
// 仅对 TCP 连接有效
func (b *baseConn) SetNoDelay(noDelay bool) error {
	return netutil.SetTCPNoDelay(b.conn, noDelay)
}

// SetTCPKeepAlive 设置 TCP 连接的 KeepAlive 选项
// 用于检测连接是否仍然有效，防止"半开连接"
// 参数:
//   - enable: 是否启用 KeepAlive
//   - period: KeepAlive 检测周期
// 仅对 TCP 连接有效
func (b *baseConn) SetTCPKeepAlive(enable bool, period time.Duration) error {
	return netutil.SetTCPKeepAlive(b.conn, enable, period)
}

// SetHandler 设置业务处理回调接口
// 用于动态更换连接的处理逻辑
// 参数 handler: 业务处理回调接口，不能为 nil
func (b *baseConn) SetHandler(handler IHandler) {
	b.handler = handler
}

// heartLoop 心跳检测循环
// 定期检查连接的最后活动时间，如果超过超时时间则关闭连接
// 检测频率为超时时间的一半，例如超时时间为 10 秒，则每 5 秒检测一次
//
// 参数:
//   - connection: 连接接口，用于关闭连接
//
// 工作流程:
//   1. 创建定时器，每 timeout/2 秒触发一次
//   2. 检查距离上次活动时间是否超过 timeout
//   3. 如果超时，设置错误并关闭连接
//   4. 如果连接已停止或上下文取消，退出循环
func (b *baseConn) heartLoop(connection IConnection) {
	var err error
	timeout := b.options.HeartTimeout
	ticker := time.NewTicker(timeout / 2)
	defer func() {
		ticker.Stop()
		if closeErr := connection.Close(err); closeErr != nil && !errors.Is(closeErr, ErrConnectionClosed) {
			glog.Error("心跳循环关闭连接时出错", zap.Int64("connectionId", connection.ID()), zap.Error(closeErr))
		}
	}()

	for !b.IsStop() {
		select {
		case <-b.ctx.Done():
			return
		case <-ticker.C:
			if time.Since(b.lastActive) > timeout {
				err = ErrConnHeartTimeout
				return
			}
		}
	}
}

// encode 编码消息
// 使用配置的编解码器将业务消息编码为字节流
// 参数 msg: 待编码的业务消息
// 返回值: 编码后的字节流和可能的错误
func (b *baseConn) encode(msg interface{}) (bin []byte, err error) {
	return b.options.Codec.Encode(msg)
}

// onConnect 处理连接建立事件
// 调用业务处理器的 OnConnect 回调
// 参数 connection: 连接接口
// 返回值: 如果返回错误，连接将被关闭
func (b *baseConn) onConnect(connection IConnection) error {
	return b.handler.OnConnect(connection)
}

// OnMessage 处理收到的消息
// 调用业务处理器的 OnMessage 回调
// 参数:
//   - conn: 连接接口
//   - msg: 已解码的业务消息
// 返回值: 如果返回错误，连接将被关闭
func (b *baseConn) OnMessage(conn IConnection, msg interface{}) error {
	return b.handler.OnMessage(conn, msg)
}

// OnClose 处理连接关闭事件
// 调用业务处理器的 OnClose 回调
// 参数:
//   - conn: 连接接口
//   - err: 关闭原因，可能为 nil（正常关闭）
func (b *baseConn) OnClose(conn IConnection, err error) {
	b.handler.OnClose(conn, err)
}

// Send 发送消息（线程安全）
// 将消息放入发送队列，由写循环异步发送
// 如果连接已关闭或发送队列已满，返回错误
//
// 参数:
//   - msg: 待发送的业务消息，如果为 nil 则直接返回（不发送）
//
// 返回值:
//   - error: 如果连接已关闭返回 ErrConnectionClosed
//            如果发送队列已满返回 ErrChannelFull
//            成功返回 nil
//
// 注意:
//   - 该方法是线程安全的，可以在多个 goroutine 中并发调用
//   - 消息会经过编码器编码后发送
func (b *baseConn) Send(msg interface{}) error {
	if b.IsStop() {
		return ErrConnectionClosed
	}
	if msg == nil {
		return nil
	}
	select {
	case b.sendChan <- msg:
	default:
		return ErrChannelFull
	}
	return nil
}

// write 批量写入多个消息到指定的 Writer
// 将多个消息编码后合并写入，用于批量发送场景，提高网络效率
//
// 工作流程:
//   1. 遍历所有消息，逐个编码并写入写缓冲区
//   2. 将缓冲区中的数据一次性写入底层连接
//   3. 如果一次写入未完成，继续写入直到缓冲区为空
//
// 参数:
//   - c: 目标 Writer，通常是网络连接
//   - msgList: 待发送的消息列表，nil 消息会被跳过
//
// 返回值:
//   - error: 编码或写入过程中的错误
//
// 注意:
//   - 该方法会合并多个消息的编码结果，减少系统调用次数
//   - 如果编码失败，会立即返回错误，不会继续处理后续消息
func (b *baseConn) write(c io.Writer, msgList ...interface{}) error {
	if len(msgList) == 0 {
		return nil // 没有消息需要写入，直接返回
	}
	for _, msg := range msgList {
		if msg == nil {
			continue
		}
		bytes, err := b.encode(msg)
		if err != nil {
			return err
		}
		// write buffer
		if _, err = b.writeBuffer.Write(bytes); err != nil {
			return err
		}
	}
	// write socket
	for b.writeBuffer.Len() > 0 {
		n, err := c.Write(b.writeBuffer.Bytes())
		if err != nil {
			return err
		}
		_ = b.writeBuffer.Skip(n)
	}
	return nil
}

// process 处理接收到的数据
// 将数据写入读缓冲区，尝试解码出完整消息，并调用业务处理器
//
// 工作流程:
//   1. 更新最后活动时间（用于心跳检测）
//   2. 将数据追加到读缓冲区
//   3. 尝试从缓冲区解码出完整消息
//   4. 如果解码成功，移除已处理的数据，调用 OnMessage
//   5. 如果解码失败（数据不完整），等待更多数据
//
// 参数:
//   - connection: 连接接口
//   - data: 接收到的原始字节数据
//
// 返回值:
//   - int: 已处理的字节数
//   - error: 解码或消息处理过程中的错误
func (b *baseConn) process(connection IConnection, data []byte) (int, error) {
	b.lastActive = time.Now()
	_, _ = b.readBuffer.Write(data)
	msg, n, err := b.options.Codec.Decode(b.readBuffer.Bytes())
	if err != nil {
		return n, err
	}
	_ = b.readBuffer.Skip(n)
	return n, b.OnMessage(connection, msg)
}

// Close 关闭连接（线程安全，可重复调用）
// 执行优雅关闭流程：调用业务回调、从管理器移除、取消上下文
//
// 参数:
//   - connection: 连接接口
//   - err: 关闭原因，可能为 nil（正常关闭）
//
// 工作流程:
//   1. 尝试停止连接（如果已停止则直接返回）
//   2. 调用业务处理器的 OnClose 回调
//   3. 从连接管理器中移除该连接
//   4. 取消上下文，通知所有等待的 goroutine
//
// 注意:
//   - 该方法是线程安全的，可以多次调用
//   - 只有第一次调用会真正执行关闭操作
func (b *baseConn) Close(connection IConnection, err error) {
	if !b.Stop() {
		return
	}
	b.OnClose(connection, err)
	RemoveConnection(connection)
	b.cancel()
	return
}
