package network

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/dzm2020/gas/pkg/glog"

	"go.uber.org/zap"
)

var (
	udpHandler = &mockUDPHandler{}
	udpCodec   = &mockCodec{} // 复用相同的编解码器
)

type udpClient struct {
	conn   *net.UDPConn
	server *net.UDPAddr
	t      *testing.T
}

func (c *udpClient) Dial(addr string) {
	// 解析服务器地址
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		c.t.Fatalf("解析UDP地址失败: %v", err)
	}

	// 创建UDP客户端连接
	conn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		c.t.Fatalf("UDP客户端连接失败: %v", err)
	}
	c.conn = conn
	c.server = udpAddr
}

func (c *udpClient) Send(msg string) {
	testMsg := msg
	if testMsg == "" {
		testMsg = "hello"
	}
	encoded, err := udpCodec.Encode(testMsg)
	if err != nil {
		c.t.Fatalf("编码消息失败: %v", err)
	}
	if _, err := c.conn.Write(encoded); err != nil {
		c.t.Fatalf("发送数据失败: %v", err)
	}
}

func (c *udpClient) Recv() interface{} {
	readBuf := make([]byte, 1024)
	c.conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	n, addr, err := c.conn.ReadFromUDP(readBuf)
	if err != nil {
		c.t.Fatalf("接收数据失败: %v", err)
	}
	if addr == nil {
		c.t.Fatalf("接收到的地址为空")
	}
	glog.Info("收到消息", zap.String("msg", string(readBuf[:n])), zap.String("from", addr.String()))
	return readBuf[:n]
}

func (c *udpClient) Close() {
	if c.conn != nil {
		c.conn.Close()
	}
}

// mockUDPHandler 简单的 mock UDP 处理器
type mockUDPHandler struct {
	onConnectCalled bool
	onMessageCalled bool
	onCloseCalled   bool
}

func (m *mockUDPHandler) OnConnect(conn IConnection) error {
	m.onConnectCalled = true
	glog.Debug("UDP OnConnect", zap.Int64("connectionId", conn.ID()))
	return nil
}

func (m *mockUDPHandler) OnMessage(conn IConnection, msg interface{}) error {
	m.onMessageCalled = true
	glog.Debug("UDP OnMessage", zap.Int64("connectionId", conn.ID()), zap.Any("msg", msg))

	// 发送响应消息
	conn.Send("udp response message")

	// conn.Close(nil)
	return nil
}

func (m *mockUDPHandler) OnClose(conn IConnection, err error) {
	m.onCloseCalled = true
	glog.Debug("UDP OnClose", zap.Int64("connectionId", conn.ID()), zap.Error(err))
}

// TestUDPServer_Listen 测试 UDP 服务器监听
func TestUDPServer_Listen(t *testing.T) {
	glog.SetLogLevel(zap.DebugLevel)

	server, err := NewServer(udpHandler, "udp://127.0.0.1:9989",
		WithCodec(udpCodec),
		WithUdpRcvChanSize(1024))
	if err != nil {
		t.Fatalf("创建UDP服务器失败: %v", err)
	}

	// 启动服务器
	if err := server.Start(); err != nil {
		t.Fatalf("启动UDP服务器失败: %v", err)
	}
	defer server.Shutdown(context.Background())

	time.Sleep(10 * time.Second)
}

// TestUDPServer_Close 测试 UDP 服务器关闭和消息收发
func TestUDPServer_Close(t *testing.T) {
	glog.SetLogLevel(zap.DebugLevel)

	server, err := NewServer(udpHandler, "udp://127.0.0.1:9990",
		WithCodec(udpCodec),
		WithUdpRcvChanSize(1024))
	if err != nil {
		t.Fatalf("创建UDP服务器失败: %v", err)
	}

	// 启动服务器
	if err := server.Start(); err != nil {
		t.Fatalf("启动UDP服务器失败: %v", err)
	}

	// 等待服务器启动
	time.Sleep(100 * time.Millisecond)

	// 创建客户端并测试
	client := udpClient{t: t}
	client.Dial("127.0.0.1:9990")
	defer client.Close()

	// 发送多条消息
	client.Send("hello")
	time.Sleep(50 * time.Millisecond)
	client.Send("world")
	time.Sleep(50 * time.Millisecond)
	client.Send("udp test")
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

// TestUDPServer_MultipleClients 测试多个UDP客户端
func TestUDPServer_MultipleClients(t *testing.T) {
	glog.SetLogLevel(zap.DebugLevel)

	server, err := NewServer(udpHandler, "udp://127.0.0.1:9991",
		WithCodec(udpCodec),
		WithUdpRcvChanSize(1024))
	if err != nil {
		t.Fatalf("创建UDP服务器失败: %v", err)
	}

	// 启动服务器
	if err := server.Start(); err != nil {
		t.Fatalf("启动UDP服务器失败: %v", err)
	}
	defer server.Shutdown(context.Background())

	// 等待服务器启动
	time.Sleep(100 * time.Millisecond)

	// 创建多个客户端
	client1 := udpClient{t: t}
	client1.Dial("127.0.0.1:9991")
	defer client1.Close()

	client2 := udpClient{t: t}
	client2.Dial("127.0.0.1:9991")
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
