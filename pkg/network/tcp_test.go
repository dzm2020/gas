package network

import (
	"context"
	"encoding/binary"
	"github.com/dzm2020/gas/pkg/glog"
	"net"
	"testing"
	"time"

	"go.uber.org/zap"
)

var (
	handler = &mockHandler{}
	codec   = &mockCodec{}
)

type tcpClient struct {
	conn net.Conn
	t    *testing.T
}

func (c *tcpClient) Dial(addr string) {
	// 客户端连接
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		c.t.Fatalf("客户端连接失败: %v", err)
	}
	c.conn = conn
}
func (c *tcpClient) Send(msg string) {
	testMsg := "hello"
	encoded, err := codec.Encode(testMsg)
	if err != nil {
		c.t.Fatalf("编码消息失败: %v", err)
	}
	if _, err := c.conn.Write(encoded); err != nil {
		c.t.Fatalf("发送数据失败: %v", err)
	}
}
func (c *tcpClient) Recv() interface{} {
	readBuf := make([]byte, 1024)
	n, err := c.conn.Read(readBuf)
	if err != nil {
		c.t.Fatalf("接收数据失败: %v", err)
	}
	glog.Info("收到消息", zap.String("msg", string(readBuf[:n])))
	return readBuf[:n]
}

// mockCodec 简单的 mock 编解码器（两字节包体长度 + 包体）
type mockCodec struct{}

func (m *mockCodec) Encode(msg interface{}) ([]byte, error) {
	var body []byte
	if str, ok := msg.(string); ok {
		body = []byte(str)
	} else if bytes, ok := msg.([]byte); ok {
		body = bytes
	} else {
		body = []byte("test")
	}

	// 两字节包体长度（大端序）+ 包体
	length := len(body)
	if length > 65535 {
		length = 65535 // 限制最大长度
		body = body[:length]
	}

	buf := make([]byte, 2+length)
	binary.BigEndian.PutUint16(buf[0:2], uint16(length))
	copy(buf[2:], body)
	return buf, nil
}

func (m *mockCodec) Decode(b []byte) (interface{}, int, error) {
	if len(b) < 2 {
		// 数据不完整，需要更多数据
		return nil, 0, nil
	}

	// 读取两字节包体长度（大端序）
	bodyLen := int(binary.BigEndian.Uint16(b[0:2]))
	totalLen := 2 + bodyLen

	if len(b) < totalLen {
		// 数据不完整，需要更多数据
		return nil, 0, nil
	}

	// 返回包体内容和已解析的字节数
	body := make([]byte, bodyLen)
	copy(body, b[2:totalLen])
	return string(body), totalLen, nil
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
	glog.Debug("OnMessage", zap.Int64("connectionId", conn.ID()), zap.Any("msg", msg))

	conn.Send("response message")

	//conn.Close(nil)
	return nil
}

func (m *mockHandler) OnClose(conn IConnection, err error) {
	m.onCloseCalled = true
}

// TestTCPServer_Listen 测试 TCP 服务器监听
func TestTCPServer_Listen(t *testing.T) {
	glog.SetLogLevel(zap.DebugLevel)

	server, err := NewServer(context.Background(), "tcp", "127.0.0.1:9988", WithHandler(handler), WithCodec(codec))
	if err != nil {
		t.Fatalf("创建服务器失败: %v", err)
	}

	// 启动服务器
	if err := server.Start(); err != nil {
		t.Fatalf("启动服务器失败: %v", err)
	}
	defer server.Shutdown(context.Background())

	time.Sleep(10 * time.Second)
}

func (c *tcpClient) Close() {
	c.conn.Close()
}

func TestTCPServer_Close(t *testing.T) {
	client := tcpClient{}
	client.Dial("127.0.0.1:9988")
	client.Send("hello")
	client.Send("hello")
	client.Send("hello")
	client.Recv()
	client.Close()
	time.Sleep(time.Second * 60)
}
