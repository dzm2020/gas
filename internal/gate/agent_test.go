package gate

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/dzm2020/gas/internal/gate/codec"
	"github.com/dzm2020/gas/internal/gate/protocol"
	"github.com/dzm2020/gas/internal/gate/session"
	"github.com/dzm2020/gas/internal/iface"
	"github.com/dzm2020/gas/pkg/lib"
	"github.com/dzm2020/gas/pkg/network"
)

// sendRecorder 记录 Send 调用，用于 Agent.Push 测试
type sendRecorder struct {
	id      int64
	context interface{}
	sent    [][]byte
}

func (s *sendRecorder) ID() int64                                      { return s.id }
func (s *sendRecorder) Send(data []byte) error                        { s.sent = append(s.sent, append([]byte(nil), data...)); return nil }
func (s *sendRecorder) Close(error) error                             { return nil }
func (s *sendRecorder) LocalAddr() string                              { return "" }
func (s *sendRecorder) RemoteAddr() string                              { return "" }
func (s *sendRecorder) IsStop() bool                                   { return false }
func (s *sendRecorder) Type() network.ConnType                         { return 0 }
func (s *sendRecorder) Context() interface{}                          { return s.context }
func (s *sendRecorder) SetContext(ctx interface{})                    { s.context = ctx }
func (s *sendRecorder) SetReadBuffer(int) error                        { return nil }
func (s *sendRecorder) SetWriteBuffer(int) error                       { return nil }
func (s *sendRecorder) SetLinger(bool, int) error                      { return nil }
func (s *sendRecorder) SetNoDelay(bool) error                         { return nil }
func (s *sendRecorder) SetTCPKeepAlive(bool, time.Duration) error       { return nil }
func (s *sendRecorder) OnConnect(network.IConnection) error            { return nil }
func (s *sendRecorder) OnMessage(network.IConnection, []byte) (int, error) { return 0, nil }
func (s *sendRecorder) OnClose(network.IConnection, error)             {}

// minimalContext 用于 Push 等不依赖 ctx 具体实现的测试
type minimalContext struct{}

func (minimalContext) InvokerMessage(interface{}) error                { return nil }
func (minimalContext) ID() *iface.Pid                                 { return &iface.Pid{} }
func (minimalContext) Named(string) error                             { return nil }
func (minimalContext) Unname() error                                  { return nil }
func (minimalContext) Actor() iface.IActor                            { return &Agent{} }
func (minimalContext) SetCallTimeout(time.Duration)                   {}
func (minimalContext) Send(*iface.Pid, string, interface{}) error     { return nil }
func (minimalContext) Call(*iface.Pid, string, interface{}, interface{}) error { return nil }
func (minimalContext) Forward(*iface.Pid, string) error                { return nil }
func (minimalContext) AfterFunc(time.Duration, iface.Task) *lib.Timer { return nil }
func (minimalContext) Message() *iface.ActorMessage                   { return nil }
func (minimalContext) Process() iface.IProcess                        { return nil }
func (minimalContext) System() iface.ISystem                           { return nil }
func (minimalContext) Shutdown() error                                 { return nil }
func (minimalContext) Node() iface.INode                              { return nil }

func TestAgent_Push_SendsEncodedMessage(t *testing.T) {
	network.ClearConnections()
	defer network.ClearConnections()

	rec := &sendRecorder{id: 100}
	network.AddConnection(rec)

	agent := &Agent{}
	s := session.New(100, &iface.Pid{})
	s.Cmd = 1
	s.Act = 2
	s.Index = 3
	data := []byte("hello")
	ctx := minimalContext{}

	err := agent.Push(ctx, s, data)
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	if len(rec.sent) != 1 {
		t.Fatalf("Send called %d times, want 1", len(rec.sent))
	}
	expected := protocol.New(1, 2, data)
	expected.Index = 3
	encoded, _ := codec.Encode(expected)
	if !bytes.Equal(rec.sent[0], encoded) {
		t.Errorf("sent %q, want %q", rec.sent[0], encoded)
	}
}

func TestAgent_Push_ConnectionNotFound(t *testing.T) {
	network.ClearConnections()
	defer network.ClearConnections()

	agent := &Agent{}
	s := session.New(999, &iface.Pid{})
	err := agent.Push(minimalContext{}, s, []byte("x"))
	if err == nil {
		t.Fatal("Push with no connection should return error")
	}
	if !errors.Is(err, ErrNotFoundEntity) {
		t.Errorf("err=%v, want ErrNotFoundEntity", err)
	}
}

func TestAgent_Push_WithBeforeEncodeMiddleware(t *testing.T) {
	network.ClearConnections()
	defer network.ClearConnections()

	rec := &sendRecorder{id: 1}
	network.AddConnection(rec)

	agent := &Agent{}
	agent.SetMiddleware([]Middleware{modifyMiddleware{suffix: []byte("-suffix")}})
	s := session.New(1, &iface.Pid{})
	s.Cmd = 0
	s.Act = 0
	ctx := minimalContext{}

	err := agent.Push(ctx, s, []byte("body"))
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	if len(rec.sent) != 1 {
		t.Fatalf("Send called %d times, want 1", len(rec.sent))
	}
	msg, _, err := codec.Decode(rec.sent[0])
	if err != nil {
		t.Fatalf("Decode sent: %v", err)
	}
	if string(msg.Data) != "body-suffix" {
		t.Errorf("Data=%q, want \"body-suffix\"", msg.Data)
	}
}

func TestAgent_Shutdown_NilSession(t *testing.T) {
	agent := &Agent{}
	err := agent.Shutdown(minimalContext{}, nil)
	if err != nil {
		t.Errorf("Shutdown(nil session) should return nil: %v", err)
	}
}

func TestAgent_Shutdown_ConnectionNotFound(t *testing.T) {
	network.ClearConnections()
	defer network.ClearConnections()

	agent := &Agent{}
	s := session.New(888, &iface.Pid{})
	err := agent.Shutdown(minimalContext{}, s)
	if err == nil {
		t.Fatal("Shutdown with no connection should return error")
	}
	if !errors.Is(err, ErrNotFoundEntity) {
		t.Errorf("err=%v, want ErrNotFoundEntity", err)
	}
}
