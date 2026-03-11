// actor_test.go 单文件集中测试 internal/actor 包：System 进程管理、名字管理、任务与关闭。
package actor

import (
	"errors"
	"testing"
	"time"

	"github.com/dzm2020/gas/internal/iface"
	"github.com/dzm2020/gas/pkg/lib/serializer"
)

// 用于 Send/Call 测试的请求与响应类型（可 JSON 序列化）
type appendReq struct{ S string }
type emptyReq struct{}
type getLastResp struct{ S string }
type echoReq struct{ Msg string }
type echoResp struct{ Msg string }

const testNodeID = uint64(1)

// -------------------- 测试用 Actor --------------------

type testActor struct{ iface.Actor }

func (a *testActor) OnInit(ctx iface.IContext, _ []interface{}) error  { return nil }
func (a *testActor) OnMessage(ctx iface.IContext, _ interface{}) error { return nil }
func (a *testActor) OnStop(ctx iface.IContext) error                   { return nil }

// sendCallTestActor 用于测试 Send：异步 Append 写入，同步 GetLast 读出。
type sendCallTestActor struct {
	iface.Actor
	last string
}

func (a *sendCallTestActor) OnInit(ctx iface.IContext, _ []interface{}) error  { return nil }
func (a *sendCallTestActor) OnMessage(ctx iface.IContext, _ interface{}) error { return nil }
func (a *sendCallTestActor) OnStop(ctx iface.IContext) error                   { return nil }
func (a *sendCallTestActor) Append(ctx iface.IContext, req *appendReq) error {
	a.last = req.S
	return nil
}
func (a *sendCallTestActor) GetLast(ctx iface.IContext, _ *emptyReq, resp *getLastResp) error {
	resp.S = a.last
	return nil
}

// echoActor 用于测试 Call：同步 Echo 回显请求内容。
type echoActor struct{ iface.Actor }

func (a *echoActor) OnInit(ctx iface.IContext, _ []interface{}) error  { return nil }
func (a *echoActor) OnMessage(ctx iface.IContext, _ interface{}) error { return nil }
func (a *echoActor) OnStop(ctx iface.IContext) error                   { return nil }
func (a *echoActor) Echo(ctx iface.IContext, req *echoReq, resp *echoResp) error {
	resp.Msg = req.Msg
	return nil
}

// -------------------- System 构造与进程查找 --------------------

func TestNewSystem(t *testing.T) {
	sys := NewSystem(testNodeID, serializer.Json)
	if sys == nil {
		t.Fatal("NewSystem returned nil")
	}
	if sys.NodeId() != testNodeID {
		t.Error("system.NodeId() != testNodeID")
	}
}

func TestSystem_Register_GetProcess_Unregister(t *testing.T) {
	sys := NewSystem(testNodeID, serializer.Json)

	pid := &iface.Pid{NodeId: testNodeID, ActorId: 100}
	ctx := &actorContext{
		pid:        pid,
		actor:      &testActor{},
		router:     GetRouterForActor(&testActor{}),
		system:     sys,
		timeout:    DefaultCallTimeout,
		serializer: serializer.Json,
	}
	mailbox := NewMailbox()
	proc := NewProcess(mailbox)
	ctx.process = proc
	mailbox.RegisterHandlers(ctx, NewSynchronizedDispatcher(DefaultDispatcherThroughput))

	if err := sys.Register(ctx); err != nil {
		t.Fatalf("Register: %v", err)
	}

	// GetProcess by id
	if p := sys.GetProcess(uint64(100)); p == nil {
		t.Error("GetProcess(100) want process, got nil")
	}
	// GetProcess by pid
	if p := sys.GetProcess(pid); p == nil {
		t.Error("GetProcess(pid) want process, got nil")
	}
	// GetProcess by string name (未命名应返回 nil)
	if p := sys.GetProcess("notexist"); p != nil {
		t.Error("GetProcess(\"notexist\") want nil, got process")
	}

	// GetAllProcesses
	all := sys.GetAllProcesses()
	if len(all) != 1 {
		t.Errorf("GetAllProcesses len = %d, want 1", len(all))
	}

	if err := sys.Unregister(ctx); err != nil {
		t.Fatalf("Unregister: %v", err)
	}
	if p := sys.GetProcess(uint64(100)); p != nil {
		t.Error("after Unregister GetProcess(100) want nil, got process")
	}
}

func TestSystem_Named_Unname(t *testing.T) {
	sys := NewSystem(testNodeID, serializer.Json)

	pid := &iface.Pid{NodeId: testNodeID, ActorId: 101}
	ctx := &actorContext{
		pid:        pid,
		actor:      &testActor{},
		router:     GetRouterForActor(&testActor{}),
		system:     sys,
		timeout:    DefaultCallTimeout,
		serializer: serializer.Json,
	}
	mailbox := NewMailbox()
	proc := NewProcess(mailbox)
	ctx.process = proc
	mailbox.RegisterHandlers(ctx, NewSynchronizedDispatcher(DefaultDispatcherThroughput))
	if err := sys.Register(ctx); err != nil {
		t.Fatalf("Register: %v", err)
	}

	ctx.pid.ActorName = "my-actor"
	if err := sys.Named(ctx); err != nil {
		t.Fatalf("Named: %v", err)
	}
	if p := sys.GetProcess("my-actor"); p == nil {
		t.Error("GetProcess(\"my-actor\") want process, got nil")
	}

	if err := sys.Unname(ctx); err != nil {
		t.Fatalf("Unname: %v", err)
	}
	if p := sys.GetProcess("my-actor"); p != nil {
		t.Error("after Unname GetProcess(\"my-actor\") want nil, got process")
	}
}

func TestSystem_Named_ErrNameAlreadyRegistered(t *testing.T) {
	sys := NewSystem(testNodeID, serializer.Json)

	pid1 := &iface.Pid{NodeId: testNodeID, ActorId: 201}
	ctx1 := &actorContext{
		pid:        pid1,
		actor:      &testActor{},
		router:     GetRouterForActor(&testActor{}),
		system:     sys,
		timeout:    DefaultCallTimeout,
		serializer: serializer.Json,
	}
	mb1 := NewMailbox()
	ctx1.process = NewProcess(mb1)
	mb1.RegisterHandlers(ctx1, NewSynchronizedDispatcher(DefaultDispatcherThroughput))
	_ = sys.Register(ctx1)
	ctx1.pid.ActorName = "dup"
	_ = sys.Named(ctx1)

	pid2 := &iface.Pid{NodeId: testNodeID, ActorId: 202, ActorName: "dup"}
	ctx2 := &actorContext{
		pid:        pid2,
		actor:      &testActor{},
		router:     GetRouterForActor(&testActor{}),
		system:     sys,
		timeout:    DefaultCallTimeout,
		serializer: serializer.Json,
	}
	mb2 := NewMailbox()
	ctx2.process = NewProcess(mb2)
	mb2.RegisterHandlers(ctx2, NewSynchronizedDispatcher(DefaultDispatcherThroughput))
	_ = sys.Register(ctx2)
	if err := sys.Named(ctx2); err != ErrNameAlreadyRegistered {
		t.Errorf("Named(dup) err = %v, want ErrNameAlreadyRegistered", err)
	}
}

func TestSystem_GetProcess_nilRef(t *testing.T) {
	sys := NewSystem(testNodeID, serializer.Json)

	if p := sys.GetProcess(nil); p != nil {
		t.Error("GetProcess(nil) want nil, got process")
	}
	var pid *iface.Pid
	if p := sys.GetProcess(pid); p != nil {
		t.Error("GetProcess(nil Pid) want nil, got process")
	}
}

func TestSystem_Spawn_returnsPid(t *testing.T) {
	sys := NewSystem(testNodeID, serializer.Json)

	pid := sys.Spawn(&testActor{})
	if pid == nil {
		t.Fatal("Spawn returned nil pid")
	}
	if pid.GetNodeId() != testNodeID {
		t.Errorf("pid.NodeId = %d, want %d", pid.GetNodeId(), testNodeID)
	}
	if pid.GetActorId() == 0 {
		t.Error("pid.ActorId should be non-zero")
	}
}

func TestSystem_SubmitTaskAndWait(t *testing.T) {
	sys := NewSystem(testNodeID, serializer.Json)

	pid := &iface.Pid{NodeId: testNodeID, ActorId: 301}
	ctx := &actorContext{
		pid:        pid,
		actor:      &testActor{},
		router:     GetRouterForActor(&testActor{}),
		system:     sys,
		timeout:    DefaultCallTimeout,
		serializer: serializer.Json,
	}
	mailbox := NewMailbox()
	proc := NewProcess(mailbox)
	ctx.process = proc
	mailbox.RegisterHandlers(ctx, NewSynchronizedDispatcher(DefaultDispatcherThroughput))
	if err := sys.Register(ctx); err != nil {
		t.Fatalf("Register: %v", err)
	}

	done := false
	err := sys.SubmitTaskAndWait(pid, func(c iface.IContext) error {
		done = true
		return nil
	}, 2*time.Second)
	if err != nil {
		t.Fatalf("SubmitTaskAndWait: %v", err)
	}
	if !done {
		t.Error("task did not run")
	}
}

func TestSystem_ShutdownProcess(t *testing.T) {
	sys := NewSystem(testNodeID, serializer.Json)

	pid := &iface.Pid{NodeId: testNodeID, ActorId: 401}
	ctx := &actorContext{
		pid:        pid,
		actor:      &testActor{},
		router:     GetRouterForActor(&testActor{}),
		system:     sys,
		timeout:    DefaultCallTimeout,
		serializer: serializer.Json,
	}
	mailbox := NewMailbox()
	proc := NewProcess(mailbox)
	ctx.process = proc
	mailbox.RegisterHandlers(ctx, NewSynchronizedDispatcher(DefaultDispatcherThroughput))
	_ = sys.Register(ctx)

	if err := sys.ShutdownProcess(pid); err != nil {
		t.Fatalf("ShutdownProcess: %v", err)
	}
	// 进程关闭后 GetProcess 仍可能返回 process 引用，但 PostMessage 会因进程退出而失败
	if p := sys.GetProcess(pid); p != nil {
		_ = p.PostMessage(iface.NewTaskMessage(func(iface.IContext) error { return nil }))
	}
}

func TestSystem_Shutdown(t *testing.T) {
	sys := NewSystem(testNodeID, serializer.Json)

	if err := sys.Shutdown(); err != nil {
		t.Fatalf("Shutdown (empty): %v", err)
	}
	// 再次 Shutdown 应返回错误（已关闭）
	if err := sys.Shutdown(); err != ErrSystemShuttingDown {
		t.Errorf("second Shutdown err = %v, want ErrSystemShuttingDown", err)
	}
}

func TestSystem_Send_afterShutdown_returnsErr(t *testing.T) {
	sys := NewSystem(testNodeID, serializer.Json)
	_ = sys.Shutdown()

	msg := iface.NewActorMessage(nil, &iface.Pid{NodeId: testNodeID, ActorId: 1}, "M", nil)
	err := sys.Send(msg)
	if err != ErrSystemShuttingDown {
		t.Errorf("Send after Shutdown err = %v, want ErrSystemShuttingDown", err)
	}
}

func TestSystem_sendToProcess_processNotFound(t *testing.T) {
	sys := NewSystem(testNodeID, serializer.Json)

	err := sys.SubmitTask(&iface.Pid{NodeId: testNodeID, ActorId: 99999}, func(iface.IContext) error { return nil })
	if err == nil {
		t.Error("SubmitTask to non-existent process want error, got nil")
	}
	if !errors.Is(err, ErrProcessNotFound) {
		t.Errorf("SubmitTask err = %v, want ErrProcessNotFound", err)
	}
}

func TestSystem_Send(t *testing.T) {
	sys := NewSystem(testNodeID, serializer.Json)

	pid := sys.Spawn(&sendCallTestActor{})
	if pid == nil {
		t.Fatal("Spawn returned nil")
	}

	data, err := serializer.Json.Marshal(&appendReq{S: "hello"})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	msg := iface.NewActorMessage(nil, pid, "Append", data)
	if err := sys.Send(msg); err != nil {
		t.Fatalf("Send: %v", err)
	}
	// 等待 mailbox 处理完 Append
	_ = sys.SubmitTaskAndWait(pid, func(iface.IContext) error { return nil }, 2*time.Second)

	// 通过 Call GetLast 校验异步 Append 已生效
	emptyData, _ := serializer.Json.Marshal(&emptyReq{})
	callMsg := iface.NewActorMessage(nil, pid, "GetLast", emptyData)
	callMsg.Deadline = time.Now().Add(3 * time.Second).Unix()
	reply, err := sys.Call(callMsg)
	if err != nil {
		t.Fatalf("Call GetLast: %v", err)
	}
	var resp getLastResp
	if err := serializer.Json.Unmarshal(reply, &resp); err != nil {
		t.Fatalf("Unmarshal GetLast response: %v", err)
	}
	if resp.S != "hello" {
		t.Errorf("GetLast after Append want S=hello, got S=%q", resp.S)
	}
}

func TestSystem_Call(t *testing.T) {
	sys := NewSystem(testNodeID, serializer.Json)

	pid := sys.Spawn(&echoActor{})
	if pid == nil {
		t.Fatal("Spawn returned nil")
	}

	req := &echoReq{Msg: "ping"}
	data, err := serializer.Json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	msg := iface.NewActorMessage(nil, pid, "Echo", data)
	msg.Deadline = time.Now().Add(3 * time.Second).Unix()
	reply, err := sys.Call(msg)
	if err != nil {
		t.Fatalf("Call Echo: %v", err)
	}
	var resp echoResp
	if err := serializer.Json.Unmarshal(reply, &resp); err != nil {
		t.Fatalf("Unmarshal Echo response: %v", err)
	}
	if resp.Msg != "ping" {
		t.Errorf("Call Echo want Msg=ping, got Msg=%q", resp.Msg)
	}
}
