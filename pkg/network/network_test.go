package network

import (
	"net"
	"testing"
	"time"

	"github.com/panjf2000/gnet/v2"
)

// mockCodec 简单的 mock 编解码器
type mockCodec struct{}

func (m *mockCodec) Encode(msg interface{}) ([]byte, error) {
	if str, ok := msg.(string); ok {
		return []byte(str), nil
	}
	return []byte("test"), nil
}

func (m *mockCodec) Decode(b []byte) (interface{}, int, error) {
	if len(b) == 0 {
		return nil, 0, nil
	}
	return string(b), len(b), nil
}

// mockHandler 简单的 mock 处理器
type mockHandler struct {
	onConnectCalled bool
	onMessageCalled bool
	onCloseCalled   bool
}

func (m *mockHandler) OnConnect(conn IConnection) error {
	m.onConnectCalled = true
	return nil
}

func (m *mockHandler) OnMessage(conn IConnection, msg interface{}) error {
	m.onMessageCalled = true
	return nil
}

func (m *mockHandler) OnClose(conn IConnection, err error) error {
	m.onCloseCalled = true
	return nil
}

// TestTCPServer_Listen 测试 TCP 服务器监听
func TestTCPServer_Listen(t *testing.T) {
	handler := &mockHandler{}
	codec := &mockCodec{}

	server := NewTCPServer("tcp://127.0.0.1:0", "tcp", ":0",
		WithHandler(handler),
		WithCodec(codec),
	)

	// 启动服务器
	if err := server.Start(); err != nil {
		t.Fatalf("启动服务器失败: %v", err)
	}
	defer server.Shutdown()

	// 验证服务器地址
	addr := server.Addr()
	if addr == "" {
		t.Error("服务器地址不应该为空")
	}
	gnet.Engine{}.Stop()
	// 测试多次关闭（应该安全）
	server.Shutdown()
	server.Shutdown()
}

// TestTCPServer_Connection 测试 TCP 连接
func TestTCPServer_Connection(t *testing.T) {
	handler := &mockHandler{}
	codec := &mockCodec{}

	// 先获取一个可用端口
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("创建临时监听器失败: %v", err)
	}
	addr := listener.Addr().String()
	listener.Close()

	// 解析地址
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("解析地址失败: %v", err)
	}

	server := NewTCPServer("tcp://127.0.0.1:"+port, "tcp", ":"+port,
		WithHandler(handler),
		WithCodec(codec),
	)

	// 启动服务器
	if err := server.Start(); err != nil {
		t.Fatalf("启动服务器失败: %v", err)
	}
	defer server.Shutdown()

	// 等待服务器启动
	time.Sleep(100 * time.Millisecond)

	// 客户端连接
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("客户端连接失败: %v", err)
	}
	defer conn.Close()

	// 等待连接建立和回调
	time.Sleep(300 * time.Millisecond)

	// 验证连接回调被调用
	if !handler.onConnectCalled {
		t.Error("OnConnect 应该被调用")
	}

	// 发送数据
	testMsg := "hello"
	if _, err := conn.Write([]byte(testMsg)); err != nil {
		t.Fatalf("发送数据失败: %v", err)
	}

	// 等待消息处理
	time.Sleep(300 * time.Millisecond)

	// 验证消息回调被调用
	if !handler.onMessageCalled {
		t.Error("OnMessage 应该被调用")
	}

	// 关闭连接
	conn.Close()
	time.Sleep(300 * time.Millisecond)

	// 验证关闭回调被调用
	if !handler.onCloseCalled {
		t.Error("OnClose 应该被调用")
	}
}
