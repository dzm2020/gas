package network

import (
	"context"
	"time"

	"github.com/dzm2020/gas/pkg/glog"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

type WebSocketConnection struct {
	*baseConnection // 嵌入基类
	conn            *websocket.Conn
	server          *WebSocketServer // 所属服务器
}

func newWebSocketConnection(ctx context.Context, conn *websocket.Conn, typ ConnectionType, options *Options) *WebSocketConnection {
	setConOptions(options, conn.NetConn())
	base := initBaseConnection(ctx, typ, conn.LocalAddr(), conn.RemoteAddr(), options)
	return &WebSocketConnection{
		baseConnection: base,
		conn:           conn,
	}
}

// Send 发送消息（线程安全）
// 注意：WebSocketConnection 使用基类的 Send 方法，不在此处编码
// 编码在 writeLoop 中进行，保持与其他连接类型的一致性

func (c *WebSocketConnection) readLoop() {
	var err error
	defer func() {
		_ = c.Close(err)
	}()

	if err = c.onConnect(c); err != nil {
		return
	}

	for !c.IsStop() {
		if err = c.read(); err != nil {
			return
		}
	}
}

func (c *WebSocketConnection) read() error {
	messageType, data, err := c.conn.ReadMessage()
	if err != nil {
		if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
			glog.Error("WebSocket读取消息失败", zap.Int64("connectionId", c.ID()), zap.Error(err))
			return err
		}
		return nil
	}
	// 只处理文本和二进制消息
	if messageType != websocket.TextMessage && messageType != websocket.BinaryMessage {
		return nil
	}
	_, err = c.process(c, data)
	return err
}

func (c *WebSocketConnection) writeLoop() {
	var err error
	defer func() {
		_ = c.Close(err)
	}()

	for !c.IsStop() {
		select {
		case <-c.ctx.Done():
			return
		case msg, _ := <-c.sendChan:
			data, w := c.encode(msg)
			if w != nil {
				err = w
				return
			}
			err = c.conn.WriteMessage(websocket.BinaryMessage, data)
			if err != nil {
				return
			}
		case <-c.getTimeoutChan():
			if err = c.checkTimeout(); err != nil {
				return
			}
		}
	}
}

func (c *WebSocketConnection) Close(err error) (w error) {
	if !c.Stop() {
		return ErrConnectionClosed
	}

	glog.Info("WebSocket连接断开", zap.Int64("connectionId", c.ID()), zap.Error(err))

	if c.conn != nil {
		// 优雅关闭连接（发送关闭帧，避免1006）
		timeout := time.Now().Add(1 * time.Second)
		w = c.conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), timeout)
		if w = c.conn.Close(); w != nil {
			return
		}
	}
	c.baseConnection.Close(c, err)
	return
}
