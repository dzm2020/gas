package gate

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dzm2020/gas/internal/component/gate/codec"
	"github.com/dzm2020/gas/internal/component/gate/protocol"
	"github.com/dzm2020/gas/internal/component/gate/session"
	"github.com/dzm2020/gas/internal/iface"
	"github.com/dzm2020/gas/pkg/cluster"
	"github.com/dzm2020/gas/pkg/lib"
	"github.com/dzm2020/gas/pkg/network"
)

// ---------- mocks ----------

type mockConn struct {
	id      int64
	context interface{}
	sent    [][]byte
}

func (m *mockConn) ID() int64 { return m.id }
func (m *mockConn) Send(data []byte) error {
	m.sent = append(m.sent, append([]byte(nil), data...))
	return nil
}
func (m *mockConn) Close(error) error                                  { return nil }
func (m *mockConn) LocalAddr() string                                  { return "" }
func (m *mockConn) RemoteAddr() string                                 { return "" }
func (m *mockConn) IsStop() bool                                       { return false }
func (m *mockConn) Type() network.ConnType                             { return 0 }
func (m *mockConn) Context() interface{}                               { return m.context }
func (m *mockConn) SetContext(ctx interface{})                         { m.context = ctx }
func (m *mockConn) SetReadBuffer(int) error                            { return nil }
func (m *mockConn) SetWriteBuffer(int) error                           { return nil }
func (m *mockConn) SetLinger(bool, int) error                          { return nil }
func (m *mockConn) SetNoDelay(bool) error                              { return nil }
func (m *mockConn) SetTCPKeepAlive(bool, time.Duration) error          { return nil }
func (m *mockConn) OnConnect(network.IConnection) error                { return nil }
func (m *mockConn) OnMessage(network.IConnection, []byte) (int, error) { return 0, nil }
func (m *mockConn) OnClose(network.IConnection, error)                 {}

type mockSystem struct {
	spawned         []iface.IActor
	shutdownProcess *iface.Pid
}

func (m *mockSystem) Spawn(actor iface.IActor, _ ...interface{}) *iface.Pid {
	m.spawned = append(m.spawned, actor)
	return &iface.Pid{ServiceId: 1}
}
func (m *mockSystem) Add(*iface.Pid, iface.IProcess)                                {}
func (m *mockSystem) Remove(*iface.Pid) error                                       { return nil }
func (m *mockSystem) Named(string, *iface.Pid) error                                { return nil }
func (m *mockSystem) Unname(*iface.Pid) error                                       { return nil }
func (m *mockSystem) HasName(string) bool                                           { return false }
func (m *mockSystem) GetProcess(*iface.Pid) iface.IProcess                          { return nil }
func (m *mockSystem) GetProcessById(uint64) iface.IProcess                          { return nil }
func (m *mockSystem) GetProcessByName(string) iface.IProcess                        { return nil }
func (m *mockSystem) GetAllProcesses() []iface.IProcess                             { return nil }
func (m *mockSystem) SubmitTask(*iface.Pid, iface.Task) error                       { return nil }
func (m *mockSystem) SubmitTaskAndWait(*iface.Pid, iface.Task, time.Duration) error { return nil }
func (m *mockSystem) Send(*iface.ActorMessage) error                                { return nil }
func (m *mockSystem) Call(*iface.ActorMessage) ([]byte, error)                      { return nil, nil }
func (m *mockSystem) ShutdownProcess(pid *iface.Pid)                                { m.shutdownProcess = pid }
func (m *mockSystem) Shutdown() error                                               { return nil }
func (m *mockSystem) Select(string, cluster.RouteStrategy) *iface.Pid               { return nil }

type testContext struct{}

func (testContext) InvokerMessage(interface{}) error                        { return nil }
func (testContext) ID() *iface.Pid                                          { return &iface.Pid{} }
func (testContext) Named(string) error                                      { return nil }
func (testContext) Unname() error                                           { return nil }
func (testContext) Actor() iface.IActor                                     { return &Agent{} }
func (testContext) SetCallTimeout(time.Duration)                            {}
func (testContext) Send(*iface.Pid, string, interface{}) error              { return nil }
func (testContext) Call(*iface.Pid, string, interface{}, interface{}) error { return nil }
func (testContext) Forward(*iface.Pid, string) error                        { return nil }
func (testContext) AfterFunc(time.Duration, iface.Task) *lib.Timer          { return nil }
func (testContext) Message() *iface.ActorMessage                            { return nil }
func (testContext) Process() iface.IProcess                                 { return nil }
func (testContext) System() iface.ISystem                                   { return nil }
func (testContext) Shutdown() error                                         { return nil }
func (testContext) Node() iface.INode                                       { return nil }

type noopMw struct{}

func (noopMw) AfterDecode(msg *protocol.Message) (*protocol.Message, error)  { return msg, nil }
func (noopMw) BeforeEncode(msg *protocol.Message) (*protocol.Message, error) { return msg, nil }

type errMw struct{ err error }

func (e errMw) AfterDecode(*protocol.Message) (*protocol.Message, error)  { return nil, e.err }
func (e errMw) BeforeEncode(*protocol.Message) (*protocol.Message, error) { return nil, e.err }

type suffixMw struct{ suffix []byte }

func (m suffixMw) AfterDecode(msg *protocol.Message) (*protocol.Message, error) {
	if msg == nil {
		return nil, nil
	}
	out := *msg
	out.Data = append(append([]byte(nil), msg.Data...), m.suffix...)
	return &out, nil
}
func (m suffixMw) BeforeEncode(msg *protocol.Message) (*protocol.Message, error) {
	if msg == nil {
		return nil, nil
	}
	out := *msg
	out.Data = append(append([]byte(nil), msg.Data...), m.suffix...)
	return &out, nil
}

// ---------- middleware ----------

func TestMiddleware(t *testing.T) {
	t.Run("RunAfterDecode", func(t *testing.T) {
		msg := protocol.New(1, 2, []byte("a"))
		out, err := RunAfterDecode(nil, msg)
		if err != nil || out != msg {
			t.Fatalf("nil chain: err=%v out=%p", err, out)
		}
		out, err = RunAfterDecode([]Middleware{noopMw{}}, msg)
		if err != nil || out != msg {
			t.Fatalf("noop: err=%v", err)
		}
		e := errors.New("err")
		out, err = RunAfterDecode([]Middleware{errMw{e}}, msg)
		if err != e || out != nil {
			t.Fatalf("error: err=%v", err)
		}
		out, err = RunAfterDecode([]Middleware{suffixMw{[]byte("b")}, suffixMw{[]byte("c")}}, msg)
		if err != nil || string(out.Data) != "abc" {
			t.Fatalf("chain: data=%q", out.Data)
		}
	})
	t.Run("RunBeforeEncode", func(t *testing.T) {
		msg := protocol.New(0, 0, []byte("1"))
		out, err := RunBeforeEncode(nil, msg)
		if err != nil || out != msg {
			t.Fatalf("nil: err=%v", err)
		}
		e := errors.New("e")
		out, err = RunBeforeEncode([]Middleware{errMw{e}}, msg)
		if err != e || out != nil {
			t.Fatalf("error: err=%v", err)
		}
		out, err = RunBeforeEncode([]Middleware{suffixMw{[]byte("2")}, suffixMw{[]byte("3")}}, msg)
		if err != nil || string(out.Data) != "123" {
			t.Fatalf("chain: data=%q", out.Data)
		}
	})
}

// ---------- gate ----------

func TestGate(t *testing.T) {
	t.Run("OnConnect_MaxConn", func(t *testing.T) {
		g := &Gate{MaxConn: 1}
		g.count.Store(2)
		g.system = &mockSystem{}
		g.Factory = func() IAgent { return &Agent{} }
		err := g.OnConnect(&mockConn{id: 1})
		if err == nil {
			t.Fatal("want error when count > MaxConn")
		}
		if g.count.Load() != 2 {
			t.Errorf("count=%d", g.count.Load())
		}
	})
	t.Run("OnConnect_BindsSession", func(t *testing.T) {
		g := &Gate{MaxConn: 10}
		g.system = &mockSystem{}
		g.Factory = func() IAgent { return &Agent{} }
		conn := &mockConn{id: 99}
		if err := g.OnConnect(conn); err != nil {
			t.Fatal(err)
		}
		s, ok := conn.Context().(*session.Session)
		if !ok || s == nil || s.GetEntityId() != 99 || s.GetAgent() == nil {
			t.Fatalf("session not bound: %T %v", conn.Context(), conn.Context())
		}
	})
	t.Run("OnMessage", func(t *testing.T) {
		g := &Gate{system: &mockSystem{}}
		conn := &mockConn{id: 1}
		n, err := g.OnMessage(conn, []byte{0, 0, 0})
		if err != nil || n != 0 {
			t.Fatalf("short: n=%d err=%v", n, err)
		}
		enc, _ := codec.Encode(protocol.New(1, 2, []byte("x")))
		n, err = g.OnMessage(conn, enc)
		if err != nil || n != len(enc) {
			t.Fatalf("no session: n=%d err=%v", n, err)
		}
		s := session.New(1, nil)
		conn.context = s
		n, err = g.OnMessage(conn, enc)
		if err != nil || n != len(enc) {
			t.Fatalf("session no agent: n=%d err=%v", n, err)
		}
	})
	t.Run("OnClose", func(t *testing.T) {
		g := &Gate{}
		g.count.Store(1)
		sys := &mockSystem{}
		g.system = sys
		g.OnClose(&mockConn{id: 1}, nil)
		if g.count.Load() != 0 || sys.shutdownProcess != nil {
			t.Fatalf("no session: count=%d shutdown=%v", g.count.Load(), sys.shutdownProcess)
		}
		pid := &iface.Pid{ServiceId: 42}
		g.count.Store(1)
		g.OnClose(&mockConn{id: 1, context: session.New(1, pid)}, nil)
		if g.count.Load() != 0 || sys.shutdownProcess != pid {
			t.Fatalf("with session: shutdown=%v", sys.shutdownProcess)
		}
	})
	t.Run("Stop", func(t *testing.T) {
		if err := (&Gate{}).Stop(context.Background()); err != nil {
			t.Fatal(err)
		}
	})
}

// ---------- agent ----------

func TestAgent(t *testing.T) {
	t.Run("Push", func(t *testing.T) {
		network.ClearConnections()
		defer network.ClearConnections()
		conn := &mockConn{id: 100}
		network.AddConnection(conn)
		s := session.New(100, &iface.Pid{})
		s.Cmd, s.Act, s.Index = 1, 2, 3
		if err := (&Agent{}).Push(testContext{}, s, []byte("hello")); err != nil {
			t.Fatal(err)
		}
		if len(conn.sent) != 1 {
			t.Fatalf("Send %d times", len(conn.sent))
		}
		exp := protocol.New(1, 2, []byte("hello"))
		exp.Index = 3
		enc, _ := codec.Encode(exp)
		if !bytes.Equal(conn.sent[0], enc) {
			t.Errorf("sent mismatch")
		}
	})
	t.Run("Push_NoConn", func(t *testing.T) {
		network.ClearConnections()
		defer network.ClearConnections()
		err := (&Agent{}).Push(testContext{}, session.New(999, &iface.Pid{}), []byte("x"))
		if err == nil || !errors.Is(err, ErrNotFoundEntity) {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("Push_Middleware", func(t *testing.T) {
		network.ClearConnections()
		defer network.ClearConnections()
		conn := &mockConn{id: 1}
		network.AddConnection(conn)
		agent := &Agent{}
		agent.SetMiddleware([]Middleware{suffixMw{[]byte("-ok")}})
		if err := agent.Push(testContext{}, session.New(1, &iface.Pid{}), []byte("body")); err != nil {
			t.Fatal(err)
		}
		msg, _, _ := codec.Decode(conn.sent[0])
		if string(msg.Data) != "body-ok" {
			t.Errorf("data=%q", msg.Data)
		}
	})
	t.Run("Shutdown", func(t *testing.T) {
		if err := (&Agent{}).Shutdown(testContext{}, nil); err != nil {
			t.Fatal(err)
		}
		network.ClearConnections()
		defer network.ClearConnections()
		err := (&Agent{}).Shutdown(testContext{}, session.New(888, &iface.Pid{}))
		if err == nil || !errors.Is(err, ErrNotFoundEntity) {
			t.Fatalf("err=%v", err)
		}
	})
}
