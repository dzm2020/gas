package network

import (
	"context"
	"errors"
	"io"
	"net"

	"github.com/dzm2020/gas/pkg/glog"
	"go.uber.org/zap"
)

// maxBatchWriteSize 单次批量写入的最大消息条数，避免内存占用过高
const maxBatchWriteSize = 100

type TCPConnection struct {
	*baseConn   // 嵌入基类
	conn     *net.TCPConn
	tmpBuf   []byte
	msgBatch [][]byte // 写循环内复用，避免 batchWriteMsg 每次分配
}

func newTCPConnection(ctx context.Context, conn *net.TCPConn, typ ConnType, options *Options, connMgr *ConnManager) *TCPConnection {
	base := newBaseConn(ctx, "tcp", typ, conn, conn.RemoteAddr(), options, connMgr)
	tcpConn := &TCPConnection{
		baseConn: base,
		tmpBuf:   make([]byte, options.ReadBufSize),
		conn:     conn,
	}
	return tcpConn
}

// NewTCPConnection 从已有的 *net.TCPConn 创建 TCPConnection，供测试或自定义 listener 集成使用。
// connMgr 可为 nil（测试时）；非 nil 时连接关闭会从该 manager 移除。
// 调用方如需完整读写需自行启动 readLoop/writeLoop；仅用于回调测试时可只调用 OnConnect/OnMessage/OnClose。
func NewTCPConnection(ctx context.Context, conn *net.TCPConn, typ ConnType, opts ...Option) *TCPConnection {
	return newTCPConnection(ctx, conn, typ, loadOptions(opts...), nil)
}

func (c *TCPConnection) readLoop() {
	var err error
	var n int
	defer func() {
		_ = c.Close(err)
	}()

	if err = c.onConnect(c); err != nil {
		return
	}
	for !c.IsStop() {
		n, err = c.conn.Read(c.tmpBuf)
		if err != nil {
			// 区分 EOF 和其他错误
			if err == io.EOF {
				return
			}
			// 对于其他错误，记录日志后返回
			if !errors.Is(err, net.ErrClosed) {
				glog.Error("TCP连接读取错误", zap.Int64("connectionId", c.ID()), zap.Error(err))
			}
			return
		}
		if n == 0 {
			err = io.EOF
			return
		}
		_, err = c.process(c, c.tmpBuf[:n])
		if err != nil {
			return
		}
	}
}

func (c *TCPConnection) writeLoop() {
	var err error
	defer func() {
		_ = c.batchWriteMsg(nil)
		_ = c.conn.Close()
		_ = c.Close(err)
	}()

	for !c.IsStop() {
		select {
		case <-c.ctx.Done():
			return
		case msg, ok := <-c.sendChan:
			if !ok {
				// channel 已关闭
				return
			}
			if err = c.batchWriteMsg(msg); err != nil {
				return
			}
		}
	}
}

func (c *TCPConnection) batchWriteMsg(msg []byte) error {
	if c.msgBatch == nil {
		c.msgBatch = make([][]byte, 0, maxBatchWriteSize)
	}
	c.msgBatch = c.msgBatch[:0]
	if msg != nil {
		c.msgBatch = append(c.msgBatch, msg)
	}
	for len(c.sendChan) > 0 && len(c.msgBatch) < maxBatchWriteSize-1 {
		select {
		case m := <-c.sendChan:
			c.msgBatch = append(c.msgBatch, m)
		default:
			break
		}
	}
	return c.write(c.conn, c.msgBatch...)
}

func (c *TCPConnection) Close(err error) (w error) {
	if !c.Stop() {
		return ErrConnectionClosed
	}

	c.baseConn.Close(c, err)

	glog.Info("TCP连接断开", zap.Int64("connectionId", c.ID()), zap.Error(err))
	return
}
