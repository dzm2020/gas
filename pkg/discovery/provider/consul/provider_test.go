package consul

import (
	"gas/pkg/discovery/iface"
	"os"
	"sync"
	"testing"
	"time"
)

func getConsulAddr() string {
	addr := os.Getenv("CONSUL_ADDR")
	if addr == "" {
		return "127.0.0.1:8500"
	}
	return addr
}

func TestProvider_Add_GetById(t *testing.T) {
	provider, err := New(WithAddress(getConsulAddr()))
	if err != nil {
		t.Skipf("跳过测试: 无法连接到 Consul: %v", err)
	}
	defer provider.Close()

	node := &iface.Node{
		Id:      1001,
		Name:    "test-service",
		Address: "127.0.0.1",
		Port:    8080,
		Tags:    []string{"test"},
	}

	if err := provider.Add(node); err != nil {
		t.Fatalf("注册节点失败: %v", err)
	}

	// 等待节点被发现
	time.Sleep(2 * time.Second)

	found := provider.GetById(node.Id)
	if found == nil {
		t.Fatal("节点未找到")
	}

	if found.GetName() != node.GetName() {
		t.Errorf("节点名称不匹配: 期望 %s, 实际 %s", node.GetName(), found.GetName())
	}

	_ = provider.Remove(node.Id)
}

func TestProvider_GetService(t *testing.T) {
	provider, err := New(WithAddress(getConsulAddr()))
	if err != nil {
		t.Skipf("跳过测试: 无法连接到 Consul: %v", err)
	}
	defer provider.Close()

	serviceName := "test-service-multi"
	nodes := []*iface.Node{
		{Id: 2001, Name: serviceName, Address: "127.0.0.1", Port: 9001},
		{Id: 2002, Name: serviceName, Address: "127.0.0.1", Port: 9002},
	}

	for _, node := range nodes {
		if err := provider.Add(node); err != nil {
			t.Fatalf("注册节点失败: %v", err)
		}
	}

	time.Sleep(2 * time.Second)

	serviceNodes := provider.GetService(serviceName)
	if len(serviceNodes) < 2 {
		t.Errorf("期望至少 2 个节点, 实际 %d", len(serviceNodes))
	}

	for _, node := range nodes {
		_ = provider.Remove(node.Id)
	}
}

func TestProvider_Subscribe_Unsubscribe(t *testing.T) {
	provider, err := New(WithAddress(getConsulAddr()))
	if err != nil {
		t.Skipf("跳过测试: 无法连接到 Consul: %v", err)
	}
	defer func() {
		provider.Close()
		time.Sleep(500 * time.Millisecond)
	}()

	serviceName := "test-subscribe-service"
	var mu sync.Mutex
	var receivedCount int

	listener := func(topology *iface.Topology) {
		mu.Lock()
		receivedCount++
		t.Log("TestProvider_Subscribe_Unsubscribe ", receivedCount)
		mu.Unlock()
	}

	if err := provider.Subscribe(serviceName, listener); err != nil {
		t.Fatalf("订阅失败: %v", err)
	}

	nodes := []*iface.Node{
		{Id: 2001, Name: serviceName, Address: "127.0.0.1", Port: 9001},
		{Id: 2002, Name: serviceName, Address: "127.0.0.1", Port: 9002},
	}
	for _, node := range nodes {
		if err := provider.Add(node); err != nil {
			t.Fatalf("注册节点失败: %v", err)
		}
		time.Sleep(time.Second)
	}

	//provider.Unsubscribe(serviceName, listener)
}
