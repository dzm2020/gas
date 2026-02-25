package gate

import (
	"context"
	"testing"
	"time"

	"github.com/dzm2020/gas/internal/gate/codec"
	"github.com/dzm2020/gas/internal/gate/protocol"
	"github.com/dzm2020/gas/internal/gate/session"
	"github.com/dzm2020/gas/internal/iface"
	"github.com/dzm2020/gas/pkg/cluster"
	"github.com/dzm2020/gas/pkg/network"
)

// mockConnection 用于 Gate 测试的假连接
type mockConnection struct {
	id      int64
	context interface{}
}

func (m *mockConnection) ID() int64                                      { return m.id }
func (m *mockConnection) Send([]byte) error                               { return nil }
func (m *mockConnection) Close(error) error                               { return nil }
func (m *mockConnection) LocalAddr() string                               { return "" }
func (m *mockConnection) RemoteAddr() string                             { return "" }
func (m *mockConnection) IsStop() bool                                    { return false }
func (m *mockConnection) Type() network.ConnType                          { return 0 }
func (m *mockConnection) Context() interface{}                            { return m.context }
func (m *mockConnection) SetContext(ctx interface{})                      { m.context = ctx }
func (m *mockConnection) SetReadBuffer(int) error                         { return nil }
func (m *mockConnection) SetWriteBuffer(int) error                        { return nil }
func (m *mockConnection) SetLinger(bool, int) error                       { return nil }
func (m *mockConnection) SetNoDelay(bool) error                           { return nil }
func (m *mockConnection) SetTCPKeepAlive(bool, time.Duration) error        { return nil }
func (m *mockConnection) OnConnect(network.IConnection) error              { return nil }
func (m *mockConnection) OnMessage(network.IConnection, []byte) (int, error) { return 0, nil }
func (m *mockConnection) OnClose(network.IConnection, error)              {}

// mockSystem 记录 Spawn / SubmitTask / ShutdownProcess 调用
type mockSystem struct {
	spawned          []iface.IActor
	submitTaskPid    *iface.Pid
	submitTaskRun   func(ctx iface.IContext) error
	shutdownProcess *iface.Pid
}

func (m *mockSystem) Spawn(actor iface.IActor, _ ...interface{}) *iface.Pid {
	m.spawned = append(m.spawned, actor)
	return &iface.Pid{ServiceId: 1}
}
func (m *mockSystem) Add(*iface.Pid, iface.IProcess)                      {}
func (m *mockSystem) Remove(*iface.Pid) error                             { return nil }
func (m *mockSystem) Named(string, *iface.Pid) error                       { return nil }
func (m *mockSystem) Unname(*iface.Pid) error                             { return nil }
func (m *mockSystem) HasName(string) bool                                 { return false }
func (m *mockSystem) GetProcess(*iface.Pid) iface.IProcess                { return nil }
func (m *mockSystem) GetProcessById(uint64) iface.IProcess                { return nil }
func (m *mockSystem) GetProcessByName(string) iface.IProcess              { return nil }
func (m *mockSystem) GetAllProcesses() []iface.IProcess                  { return nil }
func (m *mockSystem) SubmitTask(pid *iface.Pid, task iface.Task) error {
	m.submitTaskPid = pid
	m.submitTaskRun = task
	return nil
}
func (m *mockSystem) SubmitTaskAndWait(*iface.Pid, iface.Task, time.Duration) error {
	return nil
}
func (m *mockSystem) Send(*iface.ActorMessage) error   { return nil }
func (m *mockSystem) Call(*iface.ActorMessage) ([]byte, error) { return nil, nil }
func (m *mockSystem) ShutdownProcess(pid *iface.Pid) { m.shutdownProcess = pid }
func (m *mockSystem) Shutdown() error                 { return nil }
func (m *mockSystem) Select(string, cluster.RouteStrategy) *iface.Pid { return nil }

func TestGate_OnConnect_MaxConn(t *testing.T) {
	g := &Gate{MaxConn: 1}
	g.count.Store(2)
	sys := &mockSystem{}
	g.system = sys
	g.Factory = func() IAgent { return &Agent{} }

	conn := &mockConnection{id: 1}
	err := g.OnConnect(conn)
	if err == nil {
		t.Fatal("OnConnect with count > MaxConn should return error")
	}
	if g.count.Load() != 2 {
		t.Errorf("count should not increase when rejected, got %d", g.count.Load())
	}
}

func TestGate_OnConnect_BindsSession(t *testing.T) {
	g := &Gate{MaxConn: 10}
	g.count.Store(0)
	sys := &mockSystem{}
	g.system = sys
	g.Factory = func() IAgent { return &Agent{} }

	conn := &mockConnection{id: 99}
	err := g.OnConnect(conn)
	if err != nil {
		t.Fatalf("OnConnect: %v", err)
	}
	if g.count.Load() != 1 {
		t.Errorf("count want 1, got %d", g.count.Load())
	}
	s, ok := conn.Context().(*session.Session)
	if !ok || s == nil {
		t.Fatalf("Context() want *session.Session, got %T %v", conn.Context(), conn.Context())
	}
	if s.GetEntityId() != 99 {
		t.Errorf("session entity id want 99, got %d", s.GetEntityId())
	}
	if s.GetAgent() == nil {
		t.Error("session agent (pid) should be set")
	}
	if len(sys.spawned) != 1 {
		t.Errorf("Spawn called %d times, want 1", len(sys.spawned))
	}
}

func TestGate_OnMessage_ShortData(t *testing.T) {
	g := &Gate{system: &mockSystem{}}
	conn := &mockConnection{id: 1, context: nil}
	data := []byte{0, 0, 0} // 不足 HeadLen，Decode 返回 processN=0
	n, err := g.OnMessage(conn, data)
	if err != nil {
		t.Fatalf("OnMessage: %v", err)
	}
	if n != 0 {
		t.Errorf("n want 0, got %d", n)
	}
}

func TestGate_OnMessage_NoSession(t *testing.T) {
	msg := protocol.New(1, 2, []byte("hello"))
	encoded, err := codec.Encode(msg)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	g := &Gate{system: &mockSystem{}}
	conn := &mockConnection{id: 1, context: nil}
	n, err := g.OnMessage(conn, encoded)
	if err != nil {
		t.Fatalf("OnMessage: %v", err)
	}
	if n != len(encoded) {
		t.Errorf("n want %d, got %d", len(encoded), n)
	}
}

func TestGate_OnMessage_SessionNoAgent(t *testing.T) {
	msg := protocol.New(1, 2, []byte("x"))
	encoded, _ := codec.Encode(msg)
	s := session.New(1, &iface.Pid{})
	s.Agent = nil
	g := &Gate{system: &mockSystem{}}
	conn := &mockConnection{id: 1, context: s}
	n, err := g.OnMessage(conn, encoded)
	if err != nil {
		t.Fatalf("OnMessage: %v", err)
	}
	if n != len(encoded) {
		t.Errorf("n want %d, got %d", len(encoded), n)
	}
}

func TestGate_OnClose_NoSession(t *testing.T) {
	g := &Gate{}
	g.count.Store(1)
	sys := &mockSystem{}
	g.system = sys
	conn := &mockConnection{id: 1, context: nil}
	g.OnClose(conn, nil)
	if g.count.Load() != 0 {
		t.Errorf("count want 0, got %d", g.count.Load())
	}
	if sys.shutdownProcess != nil {
		t.Error("ShutdownProcess should not be called when no session")
	}
}

func TestGate_OnClose_WithSession(t *testing.T) {
	g := &Gate{}
	g.count.Store(1)
	sys := &mockSystem{}
	g.system = sys
	pid := &iface.Pid{ServiceId: 42}
	s := session.New(1, pid)
	conn := &mockConnection{id: 1, context: s}
	g.OnClose(conn, nil)
	if g.count.Load() != 0 {
		t.Errorf("count want 0, got %d", g.count.Load())
	}
	if sys.shutdownProcess != pid {
		t.Errorf("ShutdownProcess want pid %v, got %v", pid, sys.shutdownProcess)
	}
}

func TestGate_Stop_NilServer(t *testing.T) {
	g := &Gate{server: nil}
	err := g.Stop(context.Background())
	if err != nil {
		t.Errorf("Stop with nil server: %v", err)
	}
}
