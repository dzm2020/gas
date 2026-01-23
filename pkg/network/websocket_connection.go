package network

import (
	"context"
	"time"

	"github.com/dzm2020/gas/pkg/glog"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

type WebSocketConnection struct {
	*baseConn // 嵌入基类
	conn      *websocket.Conn
}

func newWebSocketConnection(ctx context.Context, conn *websocket.Conn, typ ConnType, options *Options) *WebSocketConnection {
	base := newBaseConn(ctx, "ws", typ, conn.NetConn(), conn.RemoteAddr(), options)
	return &WebSocketConnection{
		baseConn: base,
		conn:     conn,
	}
}

func (c *WebSocketConnection) readLoop() {
	var err error
	defer func() {
		_ = c.Close(err)
	}()

	if err = c.onConnect(c); err != nil {
		return
	}

	for !c.IsStop() {
		messageType, data, w := c.conn.ReadMessage()
		if w != nil {
			err = w
			return
		}
		// 只处理文本和二进制消息
		if messageType != websocket.TextMessage && messageType != websocket.BinaryMessage {
			continue
		}
		_, err = c.process(c, data)
		if err != nil {
			return
		}
	}
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
			if err = c.write(c, msg); err != nil {
				return
			}
		}
	}
}

func (c *WebSocketConnection) Write(p []byte) (n int, err error) {
	err = c.conn.WriteMessage(websocket.BinaryMessage, p)
	return len(p), err
}

func (c *WebSocketConnection) Close(err error) (w error) {
	if !c.Stop() {
		return ErrConnectionClosed
	}

	// 优雅关闭连接（发送关闭帧，避免1006）
	timeout := time.Now().Add(1 * time.Second)
	_ = c.conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), timeout)
	_ = c.conn.Close()

	c.baseConn.Close(c, err)

	glog.Info("WebSocket连接断开", zap.Int64("connectionId", c.ID()), zap.Error(err))
	return
}
