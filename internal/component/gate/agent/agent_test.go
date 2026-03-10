package agent

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/dzm2020/gas/internal/component/gate/codec"
	gateiface "github.com/dzm2020/gas/internal/component/gate/gateiface"
	"github.com/dzm2020/gas/internal/component/gate/protocol"
	"github.com/dzm2020/gas/internal/component/gate/session"
	"github.com/dzm2020/gas/internal/iface"
	"github.com/dzm2020/gas/internal/pb"
	"github.com/dzm2020/gas/pkg/lib"
	"github.com/dzm2020/gas/pkg/network"
)

// ---------- mock IConnection ----------

type mockConn struct {
	id     int64
	sent   [][]byte
	closed bool
}

func (m *mockConn) ID() int64 { return m.id }
func (m *mockConn) Send(data []byte) error {
	m.sent = append(m.sent, append([]byte(nil), data...))
	return nil
}
func (m *mockConn) Close(err error) error                              { m.closed = true; return nil }
func (m *mockConn) LocalAddr() string                                  { return "" }
func (m *mockConn) RemoteAddr() string                                 { return "" }
func (m *mockConn) IsStop() bool                                       { return m.closed }
func (m *mockConn) Type() network.ConnType                             { return 0 }
func (m *mockConn) Context() interface{}                               { return nil }
func (m *mockConn) SetContext(interface{})                             {}
func (m *mockConn) SetReadBuffer(int) error                            { return nil }
func (m *mockConn) SetWriteBuffer(int) error                           { return nil }
func (m *mockConn) SetLinger(bool, int) error                          { return nil }
func (m *mockConn) SetNoDelay(bool) error                              { return nil }
func (m *mockConn) SetTCPKeepAlive(bool, time.Duration) error          { return nil }
func (m *mockConn) OnConnect(network.IConnection) error                { return nil }
func (m *mockConn) OnMessage(network.IConnection, []byte) (int, error) { return 0, nil }
func (m *mockConn) OnClose(network.IConnection, error)                 {}

// ---------- mock IContext ----------

type mockContext struct {
	pid    *iface.Pid
	actor  iface.IActor
	system iface.ISystem
}

func (c *mockContext) ID() *iface.Pid                                          { return c.pid }
func (c *mockContext) Actor() iface.IActor                                     { return c.actor }
func (c *mockContext) InvokerMessage(interface{}) error                        { return nil }
func (c *mockContext) Serializer() lib.ISerializer                             { return nil }
func (c *mockContext) Message() *iface.ActorMessage                            { return nil }
func (c *mockContext) Process() iface.IProcess                                 { return nil }
func (c *mockContext) System() iface.ISystem                                   { return c.system }
func (c *mockContext) Named(string) error                                      { return nil }
func (c *mockContext) Unname() error                                           { return nil }
func (c *mockContext) GetName() string                                         { return "" }
func (c *mockContext) SetCallTimeout(time.Duration)                            {}
func (c *mockContext) Send(*iface.Pid, string, interface{}) error              { return nil }
func (c *mockContext) Call(*iface.Pid, string, interface{}, interface{}) error { return nil }
func (c *mockContext) Forward(*iface.Pid, string) error                        { return nil }
func (c *mockContext) AfterFunc(time.Duration, iface.Task) *lib.Timer          { return nil }
func (c *mockContext) Shutdown() error                                         { return nil }

// ---------- record handler ----------

type recordHandler struct {
	onInitCalled bool
	onRouteData  []byte
	onStopCalled bool
	onRouteErr   error
}

func (h *recordHandler) OnInit(a gateiface.IAgent) error {
	h.onInitCalled = true
	return nil
}
func (h *recordHandler) OnRoute(a gateiface.IAgent, data []byte) error {
	h.onRouteData = append(h.onRouteData, data...)
	return h.onRouteErr
}
func (h *recordHandler) OnStop(a gateiface.IAgent) error {
	h.onStopCalled = true
	return nil
}

// ---------- tests ----------

func TestAgent_New_GetEntity_GetSession(t *testing.T) {
	conn := &mockConn{id: 100}
	h := &recordHandler{}
	a := New(conn, h)
	if a.GetEntity() != conn {
		t.Error("GetEntity want conn")
	}
	if a.GetSession() != nil {
		t.Error("GetSession before OnInit should be nil")
	}
	if a.GetMiddleware() != nil {
		t.Errorf("GetMiddleware want nil, got %v", a.GetMiddleware())
	}
}

func TestAgent_OnInit(t *testing.T) {
	conn := &mockConn{id: 42}
	pid := &iface.Pid{NodeId: 1, ActorId: 2}
	ctx := &mockContext{pid: pid}
	h := &recordHandler{}
	a := New(conn, h)
	err := a.OnInit(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !h.onInitCalled {
		t.Error("OnInit should call handler.OnInit")
	}
	s := a.GetSession()
	if s == nil {
		t.Fatal("GetSession after OnInit should be non-nil")
	}
	if s.GetAgent() != pid {
		t.Errorf("session Agent want %v, got %v", pid, s.GetAgent())
	}
}

func TestAgent_OnData(t *testing.T) {
	conn := &mockConn{id: 1}
	pid := &iface.Pid{NodeId: 1, ActorId: 1}
	ctx := &mockContext{pid: pid}
	h := &recordHandler{}
	a := New(conn, h)
	_ = a.OnInit(ctx, nil)
	msg := protocol.New(1, 2, []byte("hello"))
	err := a.OnData(ctx, msg)
	if err != nil {
		t.Fatalf("OnData: %v", err)
	}
	if !bytes.Equal(h.onRouteData, []byte("hello")) {
		t.Errorf("OnRoute data want 'hello', got %q", h.onRouteData)
	}
}

func TestAgent_OnData_MiddlewareError(t *testing.T) {
	errMw := errors.New("middleware err")
	mw := &errorMiddleware{err: errMw}
	a := New(&mockConn{id: 1}, &recordHandler{})
	pid := &iface.Pid{NodeId: 1, ActorId: 1}
	ctx := &mockContext{pid: pid}
	_ = a.OnInit(ctx, nil)
	a.SetMiddleware([]gateiface.IMiddleware{mw})
	msg := protocol.New(0, 0, []byte("x"))
	err := a.OnData(ctx, msg)
	if err != errMw {
		t.Errorf("OnData want err %v, got %v", errMw, err)
	}
}

type errorMiddleware struct{ err error }

func (e *errorMiddleware) AfterDecode(gateiface.IAgent, *protocol.Message) (*protocol.Message, error) {
	return nil, e.err
}
func (e *errorMiddleware) BeforeEncode(gateiface.IAgent, *protocol.Message) (*protocol.Message, error) {
	return nil, e.err
}

func TestAgent_Push_Success(t *testing.T) {
	conn := &mockConn{id: 1}
	pid := &iface.Pid{NodeId: 1, ActorId: 1}
	ctx := &mockContext{pid: pid}
	a := New(conn, &recordHandler{})
	_ = a.OnInit(ctx, nil)
	msg := protocol.New(1, 2, []byte("body"))
	err := a.Push(msg)
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	if len(conn.sent) != 1 {
		t.Fatalf("Send want 1 time, got %d", len(conn.sent))
	}
	dec, _, _ := codec.Decode(conn.sent[0])
	if dec == nil || string(dec.Data) != "body" {
		t.Errorf("sent data want 'body', got %v", dec)
	}
}

func TestAgent_Push_MessageTooLarge(t *testing.T) {
	a := New(&mockConn{id: 1}, &recordHandler{})
	pid := &iface.Pid{NodeId: 1, ActorId: 1}
	ctx := &mockContext{pid: pid}
	_ = a.OnInit(ctx, nil)
	oversized := protocol.New(1, 2, make([]byte, codec.MaxMsgSize))
	err := a.Push(oversized)
	if err == nil {
		t.Fatal("Push with message too large should error")
	}
	if len((a.GetEntity().(*mockConn)).sent) != 0 {
		t.Error("Send should not be called")
	}
}

func TestAgent_SetValue(t *testing.T) {
	conn := &mockConn{id: 1}
	pid := &iface.Pid{NodeId: 1, ActorId: 1}
	ctx := &mockContext{pid: pid}
	a := New(conn, &recordHandler{})
	_ = a.OnInit(ctx, nil)
	a.GetSession().SetString("a", "1")
	err := a.SetValues(map[string]string{"b": "2", "c": "3"})
	if err != nil {
		t.Fatal(err)
	}
	s := a.GetSession()
	if s.GetString("a") != "1" || s.GetString("b") != "2" || s.GetString("c") != "3" {
		t.Errorf("Values merge: a=1 b=2 c=3, got a=%q b=%q c=%q", s.GetString("a"), s.GetString("b"), s.GetString("c"))
	}
}

func TestAgent_Shutdown(t *testing.T) {
	conn := &mockConn{id: 1}
	ctx := &mockContext{pid: &iface.Pid{}}
	a := New(conn, &recordHandler{})
	_ = a.OnInit(ctx, nil)
	err := a.Shutdown()
	if err != nil {
		t.Fatal(err)
	}
	if !conn.closed {
		t.Error("Close should be called on entity")
	}
}

func TestAgent_SetMiddleware_AppendMiddleware(t *testing.T) {
	a := New(&mockConn{id: 1}, &recordHandler{})
	mw := &errorMiddleware{}
	a.SetMiddleware([]gateiface.IMiddleware{mw})
	if len(a.GetMiddleware()) != 1 {
		t.Errorf("SetMiddleware len want 1, got %d", len(a.GetMiddleware()))
	}
	a.AppendMiddleware(&errorMiddleware{})
	if len(a.GetMiddleware()) != 2 {
		t.Errorf("AppendMiddleware len want 2, got %d", len(a.GetMiddleware()))
	}
}

func TestAgent_Push_WithBeforeEncodeMiddleware(t *testing.T) {
	conn := &mockConn{id: 1}
	pid := &iface.Pid{NodeId: 1, ActorId: 1}
	ctx := &mockContext{pid: pid}
	a := New(conn, &recordHandler{})
	_ = a.OnInit(ctx, nil)
	a.SetMiddleware([]gateiface.IMiddleware{&suffixMiddleware{suffix: []byte("-ok")}})
	msg := protocol.New(0, 0, []byte("x"))
	err := a.Push(msg)
	if err != nil {
		t.Fatal(err)
	}
	dec, _, _ := codec.Decode(conn.sent[0])
	if dec == nil || string(dec.Data) != "x-ok" {
		t.Errorf("middleware BeforeEncode: data want 'x-ok', got %q", dec.Data)
	}
}

type suffixMiddleware struct{ suffix []byte }

func (s *suffixMiddleware) AfterDecode(_ gateiface.IAgent, msg *protocol.Message) (*protocol.Message, error) {
	if msg == nil {
		return nil, nil
	}
	out := *msg
	out.Data = append(append([]byte(nil), msg.Data...), s.suffix...)
	return &out, nil
}
func (s *suffixMiddleware) BeforeEncode(_ gateiface.IAgent, msg *protocol.Message) (*protocol.Message, error) {
	if msg == nil {
		return nil, nil
	}
	out := *msg
	out.Data = append(append([]byte(nil), msg.Data...), s.suffix...)
	return &out, nil
}

func TestSession_New_FromFactory(t *testing.T) {
	raw := &pb.Session{Id: 10, Agent: &iface.Pid{NodeId: 1, ActorId: 1}, Values: map[string]string{}}
	ctx := &mockContext{pid: raw.GetAgent()}
	s := session.New(raw, ctx)
	if s == nil {
		t.Fatal("New should return non-nil Session")
	}
	if s.GetAgent() != raw.GetAgent() {
		t.Error("GetAgent mismatch")
	}
}
