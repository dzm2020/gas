package cluster

import (
	"context"
	"fmt"
	"sync/atomic"
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

const clusterTestIDBase = uint64(8_000_000)

var clusterTestSeq atomic.Uint64

// allocClusterMemberID 生成与其它集成测试不易冲突的节点 ID（多包并行 go test 时减轻 Consul 串扰）。
func allocClusterMemberID() uint64 {
	return clusterTestSeq.Add(1) + clusterTestIDBase
}

func waitUntilMemberGone(t *testing.T, c ICluster, id uint64, d time.Duration) {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if c.GetById(id) == nil {
			return
		}
		time.Sleep(300 * time.Millisecond)
	}
	if c.GetById(id) != nil {
		t.Fatalf("GetById(%d) 在 Deregister 后仍非 nil", id)
	}
}

var clusterMQNameSeq atomic.Uint64

func testConfig() *Config {
	mqName := fmt.Sprintf("cluster-test-mq-%d", clusterMQNameSeq.Add(1))
	return &Config{
		Name: mqName,
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
				"name":    mqName,
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
	if err := c.Run(ctx); err != nil {
		t.Skipf("Run: %v (consul/nats 可能未启动)", err)
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
	if err := c.Run(ctx); err != nil {
		t.Skipf("Run: %v (consul/nats 可能未启动)", err)
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
	mid := allocClusterMemberID()
	mem := &discovery.Member{Id: mid, Kind: fmt.Sprintf("cluster-test-svc-%d", mid), Address: "127.0.0.1", Port: 8080, Status: "passing"}
	if err := c.Register(mem); err != nil {
		t.Fatalf("Register: %v", err)
	}
	waitSync(t)
	if err := c.Send(mid, "hello"); err != nil {
		t.Fatalf("Send: %v", err)
	}
	_ = c.Deregister(mid)
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
	mid := allocClusterMemberID()
	subject := fmt.Sprint(mid)
	_, err := c.Subscribe(subject, sub)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	mem := &discovery.Member{Id: mid, Kind: fmt.Sprintf("cluster-test-call-%d", mid), Address: "127.0.0.1", Port: 8081, Status: "passing"}
	if err := c.Register(mem); err != nil {
		t.Fatalf("Register: %v", err)
	}
	waitSync(t)
	data, err := c.Call(mid, []byte("ping"), 2*time.Second)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if string(data) != "pong" {
		t.Errorf("Call 期望响应 pong, 得到 %q", data)
	}
	_ = c.Deregister(mid)
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
	subID := allocClusterMemberID()
	got, err := c.Subscribe(fmt.Sprint(subID), sub)
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
	id1, id2 := allocClusterMemberID(), allocClusterMemberID()
	tag := fmt.Sprintf("tag1-%d-%d", id1, id2)
	m1 := &discovery.Member{Id: id1, Kind: fmt.Sprintf("cluster-test-select-%d", id1), Address: "127.0.0.1", Port: 9010, Tags: []string{tag}, Status: "passing"}
	m2 := &discovery.Member{Id: id2, Kind: fmt.Sprintf("cluster-test-select-%d", id2), Address: "127.0.0.1", Port: 9020, Tags: []string{tag}, Status: "passing"}
	_ = c.Register(m1)
	_ = c.Register(m2)
	waitSync(t)
	id, err := c.Select(tag, nil)
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	if id != id1 && id != id2 {
		t.Errorf("Select 期望 %d 或 %d, 得到 %d", id1, id2, id)
	}
	id, err = c.Select(tag, RouteFirst)
	if err != nil || (id != id1 && id != id2) {
		t.Errorf("Select RouteFirst: id=%d err=%v", id, err)
	}
	_ = c.Deregister(id1)
	_ = c.Deregister(id2)
}

func TestCluster_Register_Update_Deregister(t *testing.T) {
	c, cleanup := createCluster(t)
	defer cleanup()
	mid := allocClusterMemberID()
	kind := fmt.Sprintf("cluster-test-reg-%d", mid)
	mem := &discovery.Member{Id: mid, Kind: kind, Address: "127.0.0.1", Port: 9100, Tags: []string{"t1"}, Status: "passing"}
	if err := c.Register(mem); err != nil {
		t.Fatalf("Register: %v", err)
	}
	waitSync(t)
	if c.GetById(mid) == nil {
		t.Errorf("GetById(%d) 应有已注册的 member", mid)
	}
	mem2 := &discovery.Member{Id: mid, Kind: kind, Address: "127.0.0.1", Port: 9101, Tags: []string{"t1"}, Status: "warning"}
	if err := c.Update(mem2); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if err := c.Deregister(mid); err != nil {
		t.Fatalf("Deregister: %v", err)
	}
	waitUntilMemberGone(t, c, mid, 15*time.Second)
}

func TestCluster_GetByKind_GetByTag_GetAll(t *testing.T) {
	c, cleanup := createCluster(t)
	defer cleanup()
	id1, id2, id3 := allocClusterMemberID(), allocClusterMemberID(), allocClusterMemberID()
	kindGet := fmt.Sprintf("cluster-test-get-%d", id1)
	tagA := fmt.Sprintf("tagA-%d", id1)
	tagB := fmt.Sprintf("tagB-%d", id1)
	m1 := &discovery.Member{Id: id1, Kind: kindGet, Address: "127.0.0.1", Port: 9201, Tags: []string{tagA}, Status: "passing"}
	m2 := &discovery.Member{Id: id2, Kind: kindGet, Address: "127.0.0.1", Port: 9202, Tags: []string{tagA}, Status: "passing"}
	m3 := &discovery.Member{Id: id3, Kind: kindGet + "-x", Address: "127.0.0.1", Port: 9203, Tags: []string{tagB}, Status: "passing"}
	_ = c.Register(m1)
	_ = c.Register(m2)
	_ = c.Register(m3)
	waitSync(t)
	byKind := c.GetByKind(kindGet)
	if len(byKind) != 2 {
		t.Errorf("GetByKind(%q) 期望 2, 得到 %d", kindGet, len(byKind))
	}
	byTag := c.GetByTag(tagA)
	if len(byTag) != 2 {
		t.Errorf("GetByTag(%q) 期望 2, 得到 %d", tagA, len(byTag))
	}
	all := c.GetAll()
	for _, id := range []uint64{id1, id2, id3} {
		if all[id] == nil {
			t.Errorf("GetAll 应包含本用例注册的 id %d", id)
		}
	}
	_ = c.Deregister(id1)
	_ = c.Deregister(id2)
	_ = c.Deregister(id3)
}

func TestCluster_Watch_Unwatch(t *testing.T) {
	c, cleanup := createCluster(t)
	defer cleanup()
	var called bool
	handler := func(_ *discovery.Topology) { called = true }
	c.Watch("cluster-test-watch", handler)
	// 注册一个成员触发 topology 变更
	mid := allocClusterMemberID()
	kind := fmt.Sprintf("cluster-test-watch-%d", mid)
	mem := &discovery.Member{Id: mid, Kind: kind, Address: "127.0.0.1", Port: 9301, Status: "passing"}
	_ = c.Register(mem)
	waitSync(t)
	c.Unwatch(kind, handler)
	_ = c.Deregister(mid)
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
