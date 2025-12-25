package network

import (
	"context"
	"gas/pkg/glog"
	"gas/pkg/lib/grs"
	"net"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

type WebSocketConnection struct {
	*baseConnection // 嵌入基类
	conn            *websocket.Conn
	server          *WebSocketServer // 所属服务器
	sendChan        chan []byte      // 发送队列（读写分离核心）
}

func newWebSocketConnection(conn *websocket.Conn, typ ConnectionType, options *Options) *WebSocketConnection {
	wsConn := &WebSocketConnection{
		baseConnection: initBaseConnection(typ, options),
		sendChan:       make(chan []byte, options.sendChanSize),
		conn:           conn,
	}
	AddConnection(wsConn)

	grs.Go(func(ctx context.Context) {
		wsConn.readLoop(ctx)
	})

	grs.Go(func(ctx context.Context) {
		wsConn.writeLoop(ctx)
	})

	glog.Info("创建WebSocket连接", zap.Int64("connectionId", wsConn.ID()),
		zap.String("localAddr", wsConn.LocalAddr().String()),
		zap.String("remoteAddr", wsConn.RemoteAddr().String()))
	return wsConn
}

func (c *WebSocketConnection) LocalAddr() net.Addr  { return c.conn.LocalAddr() }
func (c *WebSocketConnection) RemoteAddr() net.Addr { return c.conn.RemoteAddr() }

// Send 发送消息（线程安全）
func (c *WebSocketConnection) Send(msg interface{}) error {
	if err := c.checkClosed(); err != nil {
		return err
	}
	// 编码消息
	data, err := c.codec.Encode(msg)
	if err != nil {
		glog.Error("WebSocket发送消息编码失败", zap.Int64("connectionId", c.ID()), zap.Error(err))
		return err
	}
	select {
	case c.sendChan <- data:
	default:
		glog.Error("WebSocket发送消息失败channel已满", zap.Int64("connectionId", c.ID()))
		return ErrWebSocketSendQueueFull
	}
	return nil
}

func (c *WebSocketConnection) readLoop(ctx context.Context) {
	var err error
	defer func() {
		_ = c.Close(err)
	}()

	if err = c.handler.OnConnect(c); err != nil {
		glog.Error("WebSocket连接回调错误", zap.Int64("connectionId", c.ID()), zap.Error(err))
		return
	}

	for {
		select {
		case <-ctx.Done():
			return // 主动关闭，无错误
		case <-c.closeChan:
			return
		default:
			if err = c.read(); err != nil {
				return
			}
		}
	}
}

func (c *WebSocketConnection) read() error {
	messageType, data, err := c.conn.ReadMessage()
	if err != nil {
		if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
			glog.Error("WebSocket读取消息失败", zap.Int64("connectionId", c.ID()), zap.Error(err))
		}
		return err
	}

	// 只处理文本和二进制消息
	if messageType != websocket.TextMessage && messageType != websocket.BinaryMessage {
		return nil
	}

	_, err = c.process(c, data)

	return nil
}

func (c *WebSocketConnection) writeLoop(ctx context.Context) {
	var err error
	defer func() {
		_ = c.Close(err)
	}()

	var timeoutChan <-chan time.Time
	if c.timeoutTicker != nil {
		timeoutChan = c.timeoutTicker.C
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.closeChan:
			return
		case data, ok := <-c.sendChan:
			if !ok {
				return // 通道已关闭
			}
			// 默认使用二进制消息类型
			err = c.conn.WriteMessage(websocket.BinaryMessage, data)
			if err != nil {
				glog.Error("WebSocket写入消息失败", zap.Int64("connectionId", c.ID()), zap.Error(err))
				return
			}
		case <-timeoutChan:
			if c.isTimeout() {
				err = ErrWebSocketConnectionKeepAlive
				glog.Warn("WebSocket心跳超时", zap.Int64("connectionId", c.ID()), zap.Error(err))
				return
			}
		}
	}
}

func (c *WebSocketConnection) Close(err error) error {
	if !c.closeBase() {
		return ErrWebSocketConnectionClosed
	}

	grs.Go(func(ctx context.Context) {
		_ = c.conn.Close()
		_ = c.baseConnection.Close(c, err)
	})

	glog.Info("WebSocket连接断开", zap.Int64("connectionId", c.ID()), zap.Error(err))
	return nil
}

