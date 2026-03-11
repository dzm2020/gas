package cluster

import (
	"context"
	"testing"
	"time"

	dis "github.com/dzm2020/gas/pkg/discovery"
	discovery "github.com/dzm2020/gas/pkg/discovery/iface"
	"github.com/dzm2020/gas/pkg/glog"
	"github.com/dzm2020/gas/pkg/lib/serializer"
	mq "github.com/dzm2020/gas/pkg/messageQue"
	messageQue "github.com/dzm2020/gas/pkg/messageQue/iface"
	"go.uber.org/zap/zapcore"

	_ "github.com/dzm2020/gas/pkg/discovery/provider/consul"
	_ "github.com/dzm2020/gas/pkg/messageQue/provider/nats"
)

func testConfig() *Config {
	return &Config{
		Name: "cluster-test",
		Discovery: &dis.Config{
			Type: "consul",
			Config: map[string]interface{}{
				"address":            "127.0.0.1:8500",
				"watchWaitTime":      "1s",
				"healthTTL":          "1s",
				"deregisterInterval": "3s",
			},
		},
		MessageQueue: &mq.Config{
			Type: "nats",
			Config: map[string]interface{}{
				"servers": []string{"nats://127.0.0.1:4222"},
				"name":    "cluster-test-mq",
			},
		},
	}
}

func createCluster(t *testing.T) (ICluster, func()) {
	t.Helper()
	c, err := New(testConfig(), serializer.Json)
	if err != nil {
		t.Skipf("New: %v (consul/nats 可能未启动)", err)
	}
	ctx := context.Background()
	if err := c.Start(ctx); err != nil {
		t.Skipf("Start: %v (consul/nats 可能未启动)", err)
	}
	return c, func() { _ = c.Shutdown(ctx) }
}

func waitSync(t *testing.T) {
	t.Helper()
	time.Sleep(2 * time.Second)
}

func TestErrNotFoundMember(t *testing.T) {
	if ErrNotFoundMember == nil {
		t.Fatal("ErrNotFoundMember should not be nil")
	}
}

func TestNew_WithRealConfig(t *testing.T) {
	glog.SetLogLevel(zapcore.DebugLevel)
	c, err := New(testConfig(), serializer.Json)
	if err != nil {
		t.Skipf("New: %v (consul/nats 可能未启动)", err)
	}
	if c == nil {
		t.Fatal("New returned nil cluster")
	}
	ctx := context.Background()
	if err := c.Start(ctx); err != nil {
		t.Skipf("Start: %v (consul/nats 可能未启动)", err)
	}
	if err := c.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
}

func TestCluster_Send(t *testing.T) {
	glog.SetLogLevel(zapcore.DebugLevel)
	c, cleanup := createCluster(t)
	defer cleanup()
	// 节点不存在应报错
	if err := c.Send(999999, "msg"); err == nil {
		t.Fatal("Send 不存在的节点应返回错误")
	}
	mem := &discovery.Member{Id: 1, Kind: "cluster-test-svc", Address: "127.0.0.1", Port: 8080, Status: "passing"}
	if err := c.Register(mem); err != nil {
		t.Fatalf("Register: %v", err)
	}
	waitSync(t)
	if err := c.Send(1, "hello"); err != nil {
		t.Fatalf("Send: %v", err)
	}
	_ = c.Deregister(1)
}

func TestCluster_Call(t *testing.T) {
	c, cleanup := createCluster(t)
	defer cleanup()
	// 先订阅节点 1 以响应 Request
	ch := make(chan []byte, 1)
	sub := &testSubscriber{onMessage: func(req []byte, resp func([]byte) error) {
		ch <- req
		_ = resp([]byte("pong"))
	}}
	_, err := c.Subscribe(1, sub)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	mem := &discovery.Member{Id: 1, Kind: "cluster-test-call", Address: "127.0.0.1", Port: 8081, Status: "passing"}
	if err := c.Register(mem); err != nil {
		t.Fatalf("Register: %v", err)
	}
	waitSync(t)
	data, err := c.Call(1, []byte("ping"), 2*time.Second)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if string(data) != "pong" {
		t.Errorf("Call 期望响应 pong, 得到 %q", data)
	}
	_ = c.Deregister(1)
}

type testSubscriber struct {
	onMessage func([]byte, func([]byte) error)
}

func (s *testSubscriber) OnMessage(request []byte, response func(data []byte) error) {
	if s.onMessage != nil {
		s.onMessage(request, response)
	}
}

func TestCluster_Subscribe(t *testing.T) {
	c, cleanup := createCluster(t)
	defer cleanup()
	sub := &testSubscriber{}
	got, err := c.Subscribe(2, sub)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	if got == nil {
		t.Fatal("Subscribe 应返回非 nil 的 subscription")
	}
	_ = got.Unsubscribe()
}

func TestCluster_Select(t *testing.T) {
	c, cleanup := createCluster(t)
	defer cleanup()
	// 无成员
	_, err := c.Select("no-such-tag", nil)
	if err != ErrNotFoundMember {
		t.Errorf("Select 无成员时应返回 ErrNotFoundMember, 得到 %v", err)
	}
	m1 := &discovery.Member{Id: 10, Kind: "cluster-test-select", Address: "127.0.0.1", Port: 9010, Tags: []string{"tag1"}, Status: "passing"}
	m2 := &discovery.Member{Id: 20, Kind: "cluster-test-select", Address: "127.0.0.1", Port: 9020, Tags: []string{"tag1"}, Status: "passing"}
	_ = c.Register(m1)
	_ = c.Register(m2)
	waitSync(t)
	id, err := c.Select("tag1", nil)
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	if id != 10 && id != 20 {
		t.Errorf("Select 期望 10 或 20, 得到 %d", id)
	}
	id, err = c.Select("tag1", RouteFirst)
	if err != nil || (id != 10 && id != 20) {
		t.Errorf("Select RouteFirst: id=%d err=%v", id, err)
	}
	_ = c.Deregister(10)
	_ = c.Deregister(20)
}

func TestCluster_Register_Update_Deregister(t *testing.T) {
	c, cleanup := createCluster(t)
	defer cleanup()
	mem := &discovery.Member{Id: 100, Kind: "cluster-test-reg", Address: "127.0.0.1", Port: 9100, Tags: []string{"t1"}, Status: "passing"}
	if err := c.Register(mem); err != nil {
		t.Fatalf("Register: %v", err)
	}
	waitSync(t)
	if c.GetById(100) == nil {
		t.Error("GetById(100) 应有已注册的 member")
	}
	mem2 := &discovery.Member{Id: 100, Kind: "cluster-test-reg", Address: "127.0.0.1", Port: 9101, Tags: []string{"t1"}, Status: "warning"}
	if err := c.Update(mem2); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if err := c.Deregister(100); err != nil {
		t.Fatalf("Deregister: %v", err)
	}
	waitSync(t)
	if c.GetById(100) != nil {
		t.Error("Deregister 后 GetById(100) 应为 nil")
	}
}

func TestCluster_GetByKind_GetByTag_GetAll(t *testing.T) {
	c, cleanup := createCluster(t)
	defer cleanup()
	m1 := &discovery.Member{Id: 201, Kind: "cluster-test-get", Address: "127.0.0.1", Port: 9201, Tags: []string{"tagA"}, Status: "passing"}
	m2 := &discovery.Member{Id: 202, Kind: "cluster-test-get", Address: "127.0.0.1", Port: 9202, Tags: []string{"tagA"}, Status: "passing"}
	m3 := &discovery.Member{Id: 203, Kind: "cluster-test-get2", Address: "127.0.0.1", Port: 9203, Tags: []string{"tagB"}, Status: "passing"}
	_ = c.Register(m1)
	_ = c.Register(m2)
	_ = c.Register(m3)
	waitSync(t)
	byKind := c.GetByKind("cluster-test-get")
	if len(byKind) != 2 {
		t.Errorf("GetByKind(cluster-test-get) 期望 2, 得到 %d", len(byKind))
	}
	byTag := c.GetByTag("tagA")
	if len(byTag) != 2 {
		t.Errorf("GetByTag(tagA) 期望 2, 得到 %d", len(byTag))
	}
	all := c.GetAll()
	if len(all) < 3 {
		t.Errorf("GetAll 期望至少 3, 得到 %d", len(all))
	}
	_ = c.Deregister(201)
	_ = c.Deregister(202)
	_ = c.Deregister(203)
}

func TestCluster_Watch_Unwatch(t *testing.T) {
	c, cleanup := createCluster(t)
	defer cleanup()
	var called bool
	handler := func(_ *discovery.Topology) { called = true }
	c.Watch("cluster-test-watch", handler)
	// 注册一个成员触发 topology 变更
	mem := &discovery.Member{Id: 301, Kind: "cluster-test-watch", Address: "127.0.0.1", Port: 9301, Status: "passing"}
	_ = c.Register(mem)
	waitSync(t)
	c.Unwatch("cluster-test-watch", handler)
	_ = c.Deregister(301)
	_ = called
}

func TestCluster_Shutdown(t *testing.T) {
	c, cleanup := createCluster(t)
	defer cleanup()
	ctx := context.Background()
	cleanup() // 执行 Shutdown
	if err := c.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	// 二次 Shutdown 应直接返回
	if err := c.Shutdown(ctx); err != nil {
		t.Errorf("二次 Shutdown 应返回 nil: %v", err)
	}
}

// 确保 ICluster 接口完整
var _ ICluster = (*Cluster)(nil)
var _ messageQue.ISubscriber = (*testSubscriber)(nil)
