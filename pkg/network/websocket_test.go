package network

import (
	"context"
	"github.com/dzm2020/gas/pkg/glog"
	"net/url"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

var (
	wsHandler = &mockWSHandler{}
	wsCodec   = &mockCodec{} // 复用相同的编解码器
)

type wsClient struct {
	conn *websocket.Conn
	t    *testing.T
}

func (c *wsClient) Dial(addr string, path string) {
	// 构建 WebSocket URL
	u := url.URL{
		Scheme: "ws",
		Host:   addr,
		Path:   path,
	}

	// 连接 WebSocket 服务器
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		c.t.Fatalf("WebSocket客户端连接失败: %v", err)
	}
	c.conn = conn
}

func (c *wsClient) Send(msg string) {
	testMsg := msg
	if testMsg == "" {
		testMsg = "hello"
	}
	encoded, err := wsCodec.Encode(testMsg)
	if err != nil {
		c.t.Fatalf("编码消息失败: %v", err)
	}
	// WebSocket 发送二进制消息
	if err := c.conn.WriteMessage(websocket.BinaryMessage, encoded); err != nil {
		c.t.Fatalf("发送数据失败: %v", err)
	}
}

func (c *wsClient) Recv() interface{} {
	c.conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	messageType, data, err := c.conn.ReadMessage()
	if err != nil {
		c.t.Fatalf("接收数据失败: %v", err)
	}
	if messageType != websocket.BinaryMessage && messageType != websocket.TextMessage {
		c.t.Fatalf("接收到不支持的消息类型: %d", messageType)
	}
	glog.Info("收到消息", zap.String("msg", string(data)), zap.Int("messageType", messageType))
	return data
}

func (c *wsClient) Close() {
	if c.conn != nil {
		c.conn.Close()
	}
}

// mockWSHandler 简单的 mock WebSocket 处理器
type mockWSHandler struct {
	onConnectCalled bool
	onMessageCalled bool
	onCloseCalled   bool
}

func (m *mockWSHandler) OnConnect(conn IConnection) error {
	m.onConnectCalled = true
	glog.Debug("WebSocket OnConnect", zap.Int64("connectionId", conn.ID()))
	return nil
}

func (m *mockWSHandler) OnMessage(conn IConnection, msg interface{}) error {
	m.onMessageCalled = true
	glog.Debug("WebSocket OnMessage", zap.Int64("connectionId", conn.ID()), zap.Any("msg", msg))

	// 发送响应消息
	conn.Send("ws response message")

	// conn.Close(nil)
	return nil
}

func (m *mockWSHandler) OnClose(conn IConnection, err error) {
	m.onCloseCalled = true
	glog.Debug("WebSocket OnClose", zap.Int64("connectionId", conn.ID()), zap.Error(err))
}

// TestWebSocketServer_Listen 测试 WebSocket 服务器监听
func TestWebSocketServer_Listen(t *testing.T) {
	glog.SetLogLevel(zap.DebugLevel)

	server, err := NewServer(context.Background(), "ws", "127.0.0.1:9992/ws",
		WithHandler(wsHandler),
		WithCodec(wsCodec))
	if err != nil {
		t.Fatalf("创建WebSocket服务器失败: %v", err)
	}

	// 启动服务器
	if err := server.Start(); err != nil {
		t.Fatalf("启动WebSocket服务器失败: %v", err)
	}
	defer server.Shutdown(context.Background())

	time.Sleep(10 * time.Second)
}

// TestWebSocketServer_Close 测试 WebSocket 服务器关闭和消息收发
func TestWebSocketServer_Close(t *testing.T) {
	glog.SetLogLevel(zap.DebugLevel)

	server, err := NewServer(context.Background(), "ws", "127.0.0.1:9993/ws",
		WithHandler(wsHandler),
		WithCodec(wsCodec))
	if err != nil {
		t.Fatalf("创建WebSocket服务器失败: %v", err)
	}

	// 启动服务器
	if err := server.Start(); err != nil {
		t.Fatalf("启动WebSocket服务器失败: %v", err)
	}

	// 等待服务器启动
	time.Sleep(100 * time.Millisecond)

	// 创建客户端并测试
	client := wsClient{t: t}
	client.Dial("127.0.0.1:9993", "/ws")
	defer client.Close()

	// 发送多条消息
	client.Send("hello")
	time.Sleep(50 * time.Millisecond)
	client.Send("world")
	time.Sleep(50 * time.Millisecond)
	client.Send("ws test")
	time.Sleep(50 * time.Millisecond)

	// 接收响应
	client.Recv()
	time.Sleep(50 * time.Millisecond)
	client.Recv()
	time.Sleep(50 * time.Millisecond)

	// 关闭服务器
	server.Shutdown(context.Background())
	time.Sleep(100 * time.Millisecond)
}

// TestWebSocketServer_MultipleClients 测试多个WebSocket客户端
func TestWebSocketServer_MultipleClients(t *testing.T) {
	glog.SetLogLevel(zap.DebugLevel)

	server, err := NewServer(context.Background(), "ws", "127.0.0.1:9994/ws",
		WithHandler(wsHandler),
		WithCodec(wsCodec))
	if err != nil {
		t.Fatalf("创建WebSocket服务器失败: %v", err)
	}

	// 启动服务器
	if err := server.Start(); err != nil {
		t.Fatalf("启动WebSocket服务器失败: %v", err)
	}
	defer server.Shutdown(context.Background())

	// 等待服务器启动
	time.Sleep(100 * time.Millisecond)

	// 创建多个客户端
	client1 := wsClient{t: t}
	client1.Dial("127.0.0.1:9994", "/ws")
	defer client1.Close()

	client2 := wsClient{t: t}
	client2.Dial("127.0.0.1:9994", "/ws")
	defer client2.Close()

	// 客户端1发送消息
	client1.Send("client1 message")
	time.Sleep(50 * time.Millisecond)

	// 客户端2发送消息
	client2.Send("client2 message")
	time.Sleep(50 * time.Millisecond)

	// 接收响应
	client1.Recv()
	time.Sleep(50 * time.Millisecond)
	client2.Recv()
	time.Sleep(50 * time.Millisecond)
}

// TestWebSocketServer_CustomPath 测试自定义路径
func TestWebSocketServer_CustomPath(t *testing.T) {
	glog.SetLogLevel(zap.DebugLevel)

	server, err := NewServer(context.Background(), "ws", "127.0.0.1:9995/chat",
		WithHandler(wsHandler),
		WithCodec(wsCodec))
	if err != nil {
		t.Fatalf("创建WebSocket服务器失败: %v", err)
	}

	// 启动服务器
	if err := server.Start(); err != nil {
		t.Fatalf("启动WebSocket服务器失败: %v", err)
	}
	defer server.Shutdown(context.Background())

	// 等待服务器启动
	time.Sleep(100 * time.Millisecond)

	// 使用自定义路径连接
	client := wsClient{t: t}
	client.Dial("127.0.0.1:9995", "/chat")
	defer client.Close()

	// 发送和接收消息
	client.Send("custom path test")
	time.Sleep(50 * time.Millisecond)
	client.Recv()
	time.Sleep(50 * time.Millisecond)
}
