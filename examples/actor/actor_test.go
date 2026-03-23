package actor

import (
	"os"
	"testing"
	"time"

	"github.com/dzm2020/gas/internal/actor"
	"github.com/dzm2020/gas/internal/iface"
	"github.com/dzm2020/gas/pkg/glog"
	"github.com/dzm2020/gas/pkg/lib/serializer"
	"github.com/dzm2020/gas/pkg/lib/uid"
)

func TestMain(m *testing.M) {
	uid.Init(1)
	os.Exit(m.Run())
}

type Request struct {
	Data []byte `json:"data"`
}

type Response struct {
	Code int64 `json:"code"`
}

// MyActor 实现 IActor，并提供可被 Router 路由的导出方法（首字母大写）。
type MyActor struct{ iface.Actor }

func (a *MyActor) OnInit(ctx iface.IContext, params []interface{}) error { return nil }

func (a *MyActor) OnMessage(ctx iface.IContext, msg interface{}) error {
	return nil
}

func (a *MyActor) OnStop(ctx iface.IContext) error { return nil }

// Ping 异步消息：用于 Send 测试，Router 按方法名 "Ping" 派发。
func (a *MyActor) Ping(ctx iface.IContext, req *Request) error {
	glog.Infof("MyActor Ping data: %v", string(req.Data))
	return nil
}

// Echo 同步消息：用于 Call 测试，请求经序列化传入，响应经序列化返回。
func (a *MyActor) Echo(ctx iface.IContext, req *Request, resp *Response) error {
	resp.Code = 0
	if len(req.Data) > 0 {
		resp.Code = 123456
	}
	glog.Infof("MyActor Echo data: %v", string(req.Data))
	return nil
}

// LifecycleActor 用于验证 OnInit/OnStop 调用顺序的 Actor；在 OnInit/OnStop 中关闭对应 channel。
type LifecycleActor struct {
	iface.Actor
	InitDone chan struct{} // OnInit 执行后关闭
	StopDone chan struct{} // OnStop 执行后关闭
}

func (a *LifecycleActor) OnInit(ctx iface.IContext, params []interface{}) error {
	if a.InitDone != nil {
		close(a.InitDone)
	}
	return nil
}

func (a *LifecycleActor) OnMessage(ctx iface.IContext, msg interface{}) error { return nil }

func (a *LifecycleActor) OnStop(ctx iface.IContext) error {
	if a.StopDone != nil {
		close(a.StopDone)
	}
	return nil
}

func TestActor_Send(t *testing.T) {
	sys := actor.NewSystem(1, serializer.Json)
	defer sys.Shutdown()

	pid := sys.Spawn(&MyActor{})
	if pid == nil {
		t.Fatal("Spawn returned nil")
	}

	request := &Request{Data: []byte("send test message")}
	if err := sys.Send(nil, pid, "Ping", request); err != nil {
		t.Fatalf("Send: %v", err)
	}
	time.Sleep(100 * time.Millisecond)
}

func TestActor_Call(t *testing.T) {
	sys := actor.NewSystem(1, serializer.Json)
	defer sys.Shutdown()

	caller := sys.Spawn(&MyActor{})
	target := sys.Spawn(&MyActor{})
	if caller == nil || target == nil {
		t.Fatal("Spawn returned nil")
	}

	req := &Request{Data: []byte("call test message")}
	var resp Response
	if err := sys.Call(caller, target, "Echo", req, &resp, 2*time.Second); err != nil {
		t.Fatalf("Call: %v", err)
	}
	if resp.Code != 123456 {
		t.Errorf("Echo response Code = %d, want 123456", resp.Code)
	}
}

func TestActor_SubmitTask(t *testing.T) {
	sys := actor.NewSystem(1, serializer.Json)
	defer sys.Shutdown()

	pid := sys.Spawn(&MyActor{})
	if pid == nil {
		t.Fatal("Spawn returned nil")
	}

	done := make(chan struct{})
	task := func(ctx iface.IContext) error {
		close(done)
		return nil
	}
	if err := sys.SubmitTask(pid, task); err != nil {
		t.Fatalf("SubmitTask: %v", err)
	}
	select {
	case <-done:
		// 任务已在目标进程内执行
	case <-time.After(2 * time.Second):
		t.Fatal("SubmitTask: task did not run within timeout")
	}
}

func TestActor_SubmitTaskAndWait(t *testing.T) {
	sys := actor.NewSystem(1, serializer.Json)
	defer sys.Shutdown()

	pid := sys.Spawn(&MyActor{})
	if pid == nil {
		t.Fatal("Spawn returned nil")
	}

	var ran bool
	task := func(ctx iface.IContext) error {
		ran = true
		return nil
	}
	if err := sys.SubmitTaskAndWait(pid, task, 2*time.Second); err != nil {
		t.Fatalf("SubmitTaskAndWait: %v", err)
	}
	if !ran {
		t.Error("SubmitTaskAndWait: task should have run")
	}
}

func TestActor_OnInit_OnStop(t *testing.T) {
	sys := actor.NewSystem(1, serializer.Json)
	defer sys.Shutdown()

	initDone := make(chan struct{})
	stopDone := make(chan struct{})
	a := &LifecycleActor{InitDone: initDone, StopDone: stopDone}

	pid := sys.Spawn(a)
	if pid == nil {
		t.Fatal("Spawn returned nil")
	}

	select {
	case <-initDone:
		// OnInit 已执行
	case <-time.After(2 * time.Second):
		t.Fatal("OnInit did not run within timeout")
	}

	if err := sys.ShutdownProcess(pid); err != nil {
		t.Fatalf("ShutdownProcess: %v", err)
	}

	select {
	case <-stopDone:
		// OnStop 已执行
	case <-time.After(2 * time.Second):
		t.Fatal("OnStop did not run within timeout")
	}
}

func TestActor_System_Shutdown(t *testing.T) {
	sys := actor.NewSystem(1, serializer.Json)

	stopDone := make(chan struct{})
	lifecycleActor := &LifecycleActor{StopDone: stopDone}

	pid := sys.Spawn(lifecycleActor)
	if pid == nil {
		t.Fatal("Spawn returned nil")
	}

	if err := sys.Shutdown(); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}

	// 再次 Shutdown 应返回错误（或幂等）
	_ = sys.Shutdown()

	// Shutdown 后向已存在 pid 发消息应失败（进程可能在退出中或已注销）
	if err := sys.Send(nil, pid, "Ping", &Request{}); err == nil {
		t.Error("Send after Shutdown expected to fail")
	}

	// 验证 actor 的 OnStop 已被正确调用（Shutdown 会向进程投递退出任务，进程处理完后会执行 OnStop）
	select {
	case <-stopDone:
		// OnStop 已执行
	case <-time.After(2 * time.Second):
		t.Fatal("OnStop did not run within timeout after Shutdown")
	}
}
