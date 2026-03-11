package gate

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dzm2020/gas/internal/component/gate/agent"
	"github.com/dzm2020/gas/internal/component/gate/codec"
	"github.com/dzm2020/gas/internal/component/gate/protocol"
	"github.com/dzm2020/gas/internal/iface"
	"github.com/dzm2020/gas/pkg/lib/serializer"
	"github.com/dzm2020/gas/pkg/network"
)

// ---------- mock IConnection ----------

type mockConn struct {
	id      int64
	context interface{}
}

func (m *mockConn) ID() int64                                    { return m.id }
func (m *mockConn) Send(data []byte) error                       { return nil }
func (m *mockConn) Close(error) error                            { return nil }
func (m *mockConn) LocalAddr() string                            { return "" }
func (m *mockConn) RemoteAddr() string                           { return "" }
func (m *mockConn) IsStop() bool                                 { return false }
func (m *mockConn) Type() network.ConnType                       { return 0 }
func (m *mockConn) Context() interface{}                         { return m.context }
func (m *mockConn) SetContext(ctx interface{})                    { m.context = ctx }
func (m *mockConn) SetReadBuffer(int) error                       { return nil }
func (m *mockConn) SetWriteBuffer(int) error                      { return nil }
func (m *mockConn) SetLinger(bool, int) error                     { return nil }
func (m *mockConn) SetNoDelay(bool) error                         { return nil }
func (m *mockConn) SetTCPKeepAlive(bool, time.Duration) error    { return nil }
func (m *mockConn) OnConnect(network.IConnection) error           { return nil }
func (m *mockConn) OnMessage(network.IConnection, []byte) (int, error) { return 0, nil }
func (m *mockConn) OnClose(network.IConnection, error)            {}

// ---------- mock ISystem ----------

type mockSystem struct {
	spawned          []iface.IActor
	shutdownPid      *iface.Pid
	submitTaskPid    *iface.Pid
	submitTaskCalled bool
}

func (m *mockSystem) Spawn(actor iface.IActor, _ ...interface{}) *iface.Pid {
	m.spawned = append(m.spawned, actor)
	return &iface.Pid{NodeId: 1, ActorId: 1}
}
func (m *mockSystem) SetSessionFactory(_ iface.ISessionFactory) {}
func (m *mockSystem) SessionFactory() iface.ISessionFactory     { return nil }
func (m *mockSystem) NodeId() uint64                             { return 1 }
func (m *mockSystem) NextID() uint64                            { return 1 }
func (m *mockSystem) Serializer() serializer.ISerializer { return nil }
func (m *mockSystem) Add(*iface.Pid, iface.IProcess)             {}
func (m *mockSystem) Remove(*iface.Pid) error                    { return nil }
func (m *mockSystem) SubmitTask(pid *iface.Pid, task iface.Task) error {
	m.submitTaskPid = pid
	m.submitTaskCalled = true
	return nil
}
func (m *mockSystem) SubmitTaskAndWait(*iface.Pid, iface.Task, time.Duration) error { return nil }
func (m *mockSystem) Send(*iface.ActorMessage) error            { return nil }
func (m *mockSystem) Call(*iface.ActorMessage) ([]byte, error)   { return nil, nil }
func (m *mockSystem) GetProcess(ref interface{}) iface.IProcess {
	if pid, ok := ref.(*iface.Pid); ok && pid != nil {
		return &mockProcess{sys: m, pid: pid}
	}
	return nil
}
func (m *mockSystem) GetAllProcesses() []iface.IProcess   { return nil }
func (m *mockSystem) ShutdownProcess(pid *iface.Pid) error { m.shutdownPid = pid; return nil }
func (m *mockSystem) Shutdown() error                            { return nil }

type mockProcess struct {
	sys *mockSystem
	pid *iface.Pid
}

func (p *mockProcess) PostMessage(iface.IMessage) error { return nil }
func (p *mockProcess) Shutdown() error                 { p.sys.shutdownPid = p.pid; return nil }
func (m *mockSystem) Register(iface.IContext) error              { return nil }
func (m *mockSystem) Unregister(iface.IContext) error            { return nil }
func (m *mockSystem) Named(iface.IContext) error                 { return nil }
func (m *mockSystem) Unname(iface.IContext) error                { return nil }

// ---------- tests ----------

func TestGate_OnConnect_MaxConnReject(t *testing.T) {
	g := &Gate{MaxConn: 1}
	g.count.Store(2) // count > MaxConn
	g.system = &mockSystem{}
	g.Factory = func() agent.IHandler { return &agent.Handler{} }
	err := g.OnConnect(&mockConn{id: 1})
	if err == nil {
		t.Fatal("want error when count > MaxConn")
	}
	if err.Error() != "too many connections" {
		t.Errorf("want 'too many connections' error, got %v", err)
	}
}

func TestGate_OnConnect_Success(t *testing.T) {
	sys := &mockSystem{}
	g := &Gate{MaxConn: 10, system: sys}
	g.Factory = func() agent.IHandler { return &agent.Handler{} }
	conn := &mockConn{id: 99}
	err := g.OnConnect(conn)
	if err != nil {
		t.Fatal(err)
	}
	pid, ok := conn.Context().(*iface.Pid)
	if !ok || pid == nil {
		t.Fatalf("context should be *iface.Pid, got %T %v", conn.Context(), conn.Context())
	}
	if g.count.Load() != 1 {
		t.Errorf("count want 1, got %d", g.count.Load())
	}
	if len(sys.spawned) != 1 {
		t.Errorf("Spawn want 1 time, got %d", len(sys.spawned))
	}
}

func TestGate_OnMessage_ShortBuffer(t *testing.T) {
	g := &Gate{system: &mockSystem{}}
	conn := &mockConn{id: 1}
	n, err := g.OnMessage(conn, []byte{0, 0, 0})
	if err != nil {
		t.Fatalf("short buffer should not error: %v", err)
	}
	if n != 0 {
		t.Errorf("n want 0, got %d", n)
	}
}

func TestGate_OnMessage_ErrNoAgent(t *testing.T) {
	g := &Gate{system: &mockSystem{}}
	conn := &mockConn{id: 1} // no context/pid set
	enc, _ := codec.Encode(protocol.New(1, 2, []byte("x")))
	n, err := g.OnMessage(conn, enc)
	if err == nil {
		t.Fatal("want ErrNoAgent when no pid bound")
	}
	if !errors.Is(err, ErrNoAgent) {
		t.Errorf("want ErrNoAgent, got %v", err)
	}
	if n != len(enc) {
		t.Errorf("n want %d (bytes consumed before process), got %d", len(enc), n)
	}
}

func TestGate_OnMessage_WithPid(t *testing.T) {
	sys := &mockSystem{}
	pid := &iface.Pid{NodeId: 1, ActorId: 2}
	conn := &mockConn{id: 1, context: pid}
	g := &Gate{system: sys}
	g.Factory = func() agent.IHandler { return &agent.Handler{} }
	enc, _ := codec.Encode(protocol.New(1, 2, []byte("hello")))
	n, err := g.OnMessage(conn, enc)
	if err != nil {
		t.Fatalf("OnMessage: %v", err)
	}
	if n != len(enc) {
		t.Errorf("n want %d, got %d", len(enc), n)
	}
	if !sys.submitTaskCalled || sys.submitTaskPid != pid {
		t.Errorf("SubmitTask should be called with pid %v, called=%v pid=%v", pid, sys.submitTaskCalled, sys.submitTaskPid)
	}
}

func TestGate_OnClose(t *testing.T) {
	g := &Gate{}
	g.count.Store(1)
	sys := &mockSystem{}
	g.system = sys

	g.OnClose(&mockConn{id: 1}, nil)
	if g.count.Load() != 0 {
		t.Errorf("count want 0, got %d", g.count.Load())
	}
	if sys.shutdownPid != nil {
		t.Errorf("no pid on conn, shutdownPid should be nil, got %v", sys.shutdownPid)
	}

	pid := &iface.Pid{NodeId: 1, ActorId: 42}
	g.count.Store(1)
	g.OnClose(&mockConn{id: 1, context: pid}, nil)
	if g.count.Load() != 0 {
		t.Errorf("count want 0, got %d", g.count.Load())
	}
	if sys.shutdownPid != pid {
		t.Errorf("shutdownPid want %v, got %v", pid, sys.shutdownPid)
	}
}

func TestGate_Stop(t *testing.T) {
	err := (&Gate{}).Stop(context.Background())
	if err != nil {
		t.Fatal(err)
	}
}
