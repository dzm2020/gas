package event

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/dzm2020/gas/internal/actor"
	"github.com/dzm2020/gas/internal/iface"
	pkgcluster "github.com/dzm2020/gas/pkg/cluster"
	dis "github.com/dzm2020/gas/pkg/discovery"
	"github.com/dzm2020/gas/pkg/lib/serializer"
	"github.com/dzm2020/gas/pkg/lib/uid"
	mq "github.com/dzm2020/gas/pkg/messageQue"

	_ "github.com/dzm2020/gas/pkg/discovery/provider/consul"
	_ "github.com/dzm2020/gas/pkg/messageQue/provider/nats"
)

func TestMain(m *testing.M) {
	uid.Init(1)
	os.Exit(m.Run())
}

const topicDemo = "example.event.demo"

// EventListenerActor 在 OnInit 中 SubscribeEvent，将收到的 payload 写入 Payloads（带缓冲以免阻塞邮箱）。
type EventListenerActor struct {
	iface.Actor
	Topic    string
	Payloads chan string
	Ready    chan struct{}
	// SubOut 非 nil 时，订阅成功后把 IEventSubscription 发回测试协程（用于 Unsubscribe 等场景）
	SubOut chan iface.IEventSubscription
}

func (a *EventListenerActor) OnInit(ctx iface.IContext, params []interface{}) error {
	topic := a.Topic
	if topic == "" {
		topic = topicDemo
	}
	sub, err := ctx.Subscribe(topic, func(_ string, payload []byte) {
		select {
		case a.Payloads <- string(payload):
		default:
		}
	})
	if err != nil {
		return err
	}
	if a.SubOut != nil {
		a.SubOut <- sub
	}
	if a.Ready != nil {
		close(a.Ready)
	}
	return nil
}

func (a *EventListenerActor) OnMessage(ctx iface.IContext, msg interface{}) error { return nil }
func (a *EventListenerActor) OnStop(ctx iface.IContext) error                     { return nil }

func waitReady(t *testing.T, ready <-chan struct{}) {
	t.Helper()
	select {
	case <-ready:
	case <-time.After(2 * time.Second):
		t.Fatal("OnInit / ctx.Subscribe 超时")
	}
}

// TestEvent_PublishLocal 本节点 PublishLocal，回调在订阅 Actor 邮箱中执行并收到 payload。
func TestEvent_PublishLocal(t *testing.T) {
	sys := actor.NewSystem(1, serializer.Json)
	defer sys.Shutdown()

	payloads := make(chan string, 4)
	ready := make(chan struct{})
	listener := &EventListenerActor{
		Topic:    "evt.local",
		Payloads: payloads,
		Ready:    ready,
	}
	if sys.Spawn(listener) == nil {
		t.Fatal("Spawn returned nil")
	}
	waitReady(t, ready)

	sys.PublishLocal("evt.local", []byte("hello-local"))
	select {
	case s := <-payloads:
		if s != "hello-local" {
			t.Fatalf("payload = %q, want hello-local", s)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("未收到事件 payload")
	}
}

// explicitSubscribeActor 使用 ctx.Subscribe（订阅者为当前 Actor）。
type explicitSubscribeActor struct {
	iface.Actor
	payloads chan string
	ready    chan struct{}
}

func (a *explicitSubscribeActor) OnInit(ctx iface.IContext, _ []interface{}) error {
	_, err := ctx.Subscribe("evt.named", func(_ string, payload []byte) {
		select {
		case a.payloads <- string(payload):
		default:
		}
	})
	if err != nil {
		return err
	}
	close(a.ready)
	return nil
}

func (a *explicitSubscribeActor) OnMessage(ctx iface.IContext, msg interface{}) error { return nil }
func (a *explicitSubscribeActor) OnStop(ctx iface.IContext) error                     { return nil }

// TestEvent_ContextSubscribe 使用 IContext.Subscribe。
func TestEvent_ContextSubscribe(t *testing.T) {
	sys := actor.NewSystem(1, serializer.Json)
	defer sys.Shutdown()

	payloads := make(chan string, 4)
	ready := make(chan struct{})
	h := &explicitSubscribeActor{payloads: payloads, ready: ready}
	if sys.Spawn(h) == nil {
		t.Fatal("Spawn returned nil")
	}
	waitReady(t, ready)

	sys.PublishLocal("evt.named", []byte("via-pid"))
	select {
	case s := <-payloads:
		if s != "via-pid" {
			t.Fatalf("payload = %q", s)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("未收到事件")
	}
}

// TestEvent_PublishCluster_LocalSystem 单节点 System 上 PublishCluster 应返回 ErrEventNoCluster。
func TestEvent_PublishCluster_LocalSystem(t *testing.T) {
	sys := actor.NewSystem(1, serializer.Json)
	defer sys.Shutdown()

	err := sys.PublishCluster("any.topic", []byte("{}"))
	if err != actor.ErrEventNoCluster {
		t.Fatalf("PublishCluster = %v, want ErrEventNoCluster", err)
	}
}

// TestEvent_Unsubscribe 取消订阅后不再收到新事件。
func TestEvent_Unsubscribe(t *testing.T) {
	sys := actor.NewSystem(1, serializer.Json)
	defer sys.Shutdown()

	payloads := make(chan string, 4)
	ready := make(chan struct{})
	subOut := make(chan iface.IEventSubscription, 1)
	listener := &EventListenerActor{
		Topic:    "evt.unsub",
		Payloads: payloads,
		Ready:    ready,
		SubOut:   subOut,
	}
	if sys.Spawn(listener) == nil {
		t.Fatal("Spawn returned nil")
	}
	waitReady(t, ready)

	sys.PublishLocal("evt.unsub", []byte("first"))
	select {
	case s := <-payloads:
		if s != "first" {
			t.Fatalf("first payload = %q", s)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("未收到第一条")
	}

	sub := <-subOut
	if err := sub.Unsubscribe(); err != nil {
		t.Fatalf("Unsubscribe: %v", err)
	}
	sys.PublishLocal("evt.unsub", []byte("second"))
	select {
	case s := <-payloads:
		t.Fatalf("取消订阅后不应再收到: %q", s)
	case <-time.After(300 * time.Millisecond):
		// 预期：无投递
	}
}

// TestEvent_MultipleSubscribers 同一 topic 多个 Actor 均应收到（各自邮箱）。
func TestEvent_MultipleSubscribers(t *testing.T) {
	sys := actor.NewSystem(1, serializer.Json)
	defer sys.Shutdown()

	const topic = "evt.broadcast"
	ready1, ready2 := make(chan struct{}), make(chan struct{})
	ch1, ch2 := make(chan string, 1), make(chan string, 1)

	a1 := &EventListenerActor{Topic: topic, Payloads: ch1, Ready: ready1}
	a2 := &EventListenerActor{Topic: topic, Payloads: ch2, Ready: ready2}
	if sys.Spawn(a1) == nil || sys.Spawn(a2) == nil {
		t.Fatal("Spawn failed")
	}
	waitReady(t, ready1)
	waitReady(t, ready2)

	sys.PublishLocal(topic, []byte("multi"))

	for i, ch := range []<-chan string{ch1, ch2} {
		select {
		case s := <-ch:
			if s != "multi" {
				t.Fatalf("subscriber %d payload = %q", i+1, s)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("subscriber %d 超时未收到", i+1)
		}
	}
}

// eventClusterConfig 与 pkg/cluster 集成测试一致：需本机 Consul + NATS；不可用则跳过用例。
func eventClusterConfig() *pkgcluster.Config {
	return &pkgcluster.Config{
		Name: "examples-event-cluster",
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
				"name":    "examples-event-cluster-mq",
			},
		},
	}
}

// TestEvent_PublishCluster_crossNode 两个独立 Cluster 连接：节点 A PublishCluster，节点 B 上 Actor 经 MQ 收到并走本地邮箱。
func TestEvent_PublishCluster_crossNode(t *testing.T) {
	ctx := context.Background()
	ser := serializer.Json

	cPub, err := pkgcluster.New(eventClusterConfig(), ser)
	if err != nil {
		t.Skipf("集群不可用（Consul/NATS 等）: New: %v", err)
	}
	if err = cPub.Run(ctx); err != nil {
		t.Skipf("集群 Run: %v", err)
	}
	cSub, err := pkgcluster.New(eventClusterConfig(), ser)
	if err != nil {
		_ = cPub.Shutdown(ctx)
		t.Skipf("第二路集群 New: %v", err)
	}
	if err = cSub.Run(ctx); err != nil {
		_ = cPub.Shutdown(ctx)
		t.Skipf("第二路集群 Run: %v", err)
	}

	defer func() {
		_ = cPub.Shutdown(ctx)
		_ = cSub.Shutdown(ctx)
	}()

	const nodePub uint64 = 91001
	const nodeSub uint64 = 92002
	sysPub, err := actor.NewClusterSystem(nodePub, ser, cPub)
	if err != nil {
		t.Fatalf("NewClusterSystem pub: %v", err)
	}
	sysSub, err := actor.NewClusterSystem(nodeSub, ser, cSub)
	if err != nil {
		_ = sysPub.Shutdown()
		t.Fatalf("NewClusterSystem sub: %v", err)
	}
	defer func() {
		_ = sysSub.Shutdown()
		_ = sysPub.Shutdown()
	}()

	const topic = "evt.cluster.cross"
	payloads := make(chan string, 2)
	ready := make(chan struct{})
	listener := &EventListenerActor{
		Topic:    topic,
		Payloads: payloads,
		Ready:    ready,
	}
	if sysSub.Spawn(listener) == nil {
		t.Fatal("Spawn listener nil")
	}
	waitReady(t, ready)

	// 给 NATS 侧订阅一点建立时间（与 pkg/cluster 集成测试类似）
	time.Sleep(500 * time.Millisecond)

	want := "hello-from-publisher-node"
	if err := sysPub.PublishCluster(topic, []byte(want)); err != nil {
		t.Fatalf("PublishCluster: %v", err)
	}

	select {
	case got := <-payloads:
		if got != want {
			t.Fatalf("payload = %q, want %q", got, want)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("订阅节点未在超时内收到集群事件")
	}
}

// TestEvent_PublishCluster_sameProcessTwoSystems 同一进程内两个 ClusterSystem（两路 MQ）均可收到对方 PublishCluster（验证 gas.event.> 投递与本地 eventBus）。
func TestEvent_PublishCluster_twoReceivers(t *testing.T) {
	ctx := context.Background()
	ser := serializer.Json

	c1, err := pkgcluster.New(eventClusterConfig(), ser)
	if err != nil {
		t.Skipf("集群不可用: New: %v", err)
	}
	if err = c1.Run(ctx); err != nil {
		t.Skipf("集群 Run: %v", err)
	}
	c2, err := pkgcluster.New(eventClusterConfig(), ser)
	if err != nil {
		_ = c1.Shutdown(ctx)
		t.Skipf("第二路 New: %v", err)
	}
	if err = c2.Run(ctx); err != nil {
		_ = c1.Shutdown(ctx)
		t.Skipf("第二路 Run: %v", err)
	}
	defer func() {
		_ = c1.Shutdown(ctx)
		_ = c2.Shutdown(ctx)
	}()

	sys1, err := actor.NewClusterSystem(83001, ser, c1)
	if err != nil {
		t.Fatal(err)
	}
	sys2, err := actor.NewClusterSystem(83002, ser, c2)
	if err != nil {
		_ = sys1.Shutdown()
		t.Fatal(err)
	}
	defer func() {
		_ = sys2.Shutdown()
		_ = sys1.Shutdown()
	}()

	const topic = "evt.cluster.dual"
	ch1 := make(chan string, 1)
	ch2 := make(chan string, 1)
	r1, r2 := make(chan struct{}), make(chan struct{})

	if sys1.Spawn(&EventListenerActor{Topic: topic, Payloads: ch1, Ready: r1}) == nil {
		t.Fatal("spawn1")
	}
	if sys2.Spawn(&EventListenerActor{Topic: topic, Payloads: ch2, Ready: r2}) == nil {
		t.Fatal("spawn2")
	}
	waitReady(t, r1)
	waitReady(t, r2)
	time.Sleep(500 * time.Millisecond)

	payload := []byte("dual-broadcast")
	if err := sys1.PublishCluster(topic, payload); err != nil {
		t.Fatalf("PublishCluster: %v", err)
	}

	// 两节点都应经各自 MQ 收到
	for i, ch := range []<-chan string{ch1, ch2} {
		select {
		case got := <-ch:
			if got != string(payload) {
				t.Fatalf("receiver %d: %q", i+1, got)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("receiver %d 超时", i+1)
		}
	}
}
