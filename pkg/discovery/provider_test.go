package discovery

import (
	"gas/pkg/discovery/iface"
	"gas/pkg/discovery/provider/consul"
	"os"
	"sync"
	"testing"
	"time"
)

// 集成测试需要真实的 Consul 服务
// 运行集成测试: go test -tags=integration -v ./pkg/discovery/provider/consul
// 或者设置环境变量: CONSUL_ADDR=127.0.0.1:8500

func TestProvider_AddNode(t *testing.T) {
	provider, err := consul.New(consul.WithAddress(getConsulAddr()))
	if err != nil {
		t.Skipf("跳过测试: 无法连接到 Consul: %v", err)
	}
	defer provider.Close()

	node := &iface.Node{
		Id:      1001,
		Name:    "test-service",
		Address: "127.0.0.1",
		Port:    8080,
		Tags:    []string{"test", "v1"},
		Meta:    map[string]string{"version": "1.0.0"},
	}

	if err := provider.Add(node); err != nil {
		t.Fatalf("注册节点失败: %v", err)
	}

	// 验证节点已注册
	provider.Watch(node.Name, nil)

	// 等待注册完成
	time.Sleep(1 * time.Second)

	registeredNode := provider.GetById(node.Id)
	if registeredNode == nil {
		t.Fatal("节点未找到")
	}

	if registeredNode.GetName() != node.GetName() {
		t.Errorf("节点名称不匹配: 期望 %s, 实际 %s", node.GetName(), registeredNode.GetName())
	}
	// 等待移除完成
	time.Sleep(5 * time.Second)
	// 清理
	_ = provider.Remove(node.Id)

}

func TestProvider_WatchNode(t *testing.T) {
	provider, err := consul.New(consul.WithAddress(getConsulAddr()))
	if err != nil {
		t.Skipf("跳过测试: 无法连接到 Consul: %v", err)
	}
	defer provider.Close()

	serviceName := "test-watch-service"
	var mu sync.Mutex
	var receivedTopology *iface.Topology
	var updateCount int
	_ = receivedTopology
	handler := func(topology *iface.Topology) {
		mu.Lock()
		defer mu.Unlock()
		receivedTopology = topology
		updateCount++
	}

	// 开始监听
	if err := provider.Watch(serviceName, handler); err != nil {
		t.Fatalf("开始监听失败: %v", err)
	}

	// 等待初始同步
	time.Sleep(1 * time.Second)

	// 注册一个新节点
	node := &iface.Node{
		Id:      1003,
		Name:    serviceName,
		Address: "127.0.0.1",
		Port:    8082,
		Tags:    []string{"test", "watched"},
	}

	if err := provider.Add(node); err != nil {
		t.Fatalf("注册节点失败: %v", err)
	}

	// 等待发现
	time.Sleep(2 * time.Second)

	mu.Lock()
	hasUpdate := updateCount > 0
	mu.Unlock()

	if !hasUpdate {
		t.Error("未收到节点更新事件")
	}

	// 验证节点被发现
	watchedNode := provider.GetById(node.Id)
	if watchedNode == nil {
		t.Error("监听的节点未找到")
	}

	// 清理
	_ = provider.Remove(node.Id)
}

func TestProvider_GetByKind(t *testing.T) {
	provider, err := consul.New(consul.WithAddress(getConsulAddr()))
	if err != nil {
		t.Skipf("跳过测试: 无法连接到 Consul: %v", err)
	}
	defer provider.Close()

	serviceName := "test-kind-service"
	kind := "gateway"

	// 注册多个不同 kind 的节点
	nodes := []*iface.Node{
		{
			Id:      2001,
			Name:    serviceName,
			Address: "127.0.0.1",
			Port:    9001,
			Tags:    []string{kind, "v1"},
		},
		{
			Id:      2002,
			Name:    serviceName,
			Address: "127.0.0.1",
			Port:    9002,
			Tags:    []string{kind, "v2"},
		},
		{
			Id:      2003,
			Name:    serviceName,
			Address: "127.0.0.1",
			Port:    9003,
			Tags:    []string{"worker", "v1"},
		},
	}

	for _, node := range nodes {
		if err := provider.Add(node); err != nil {
			t.Fatalf("注册节点失败: %v", err)
		}
	}

	// 等待注册和发现
	time.Sleep(2 * time.Second)

	// 开始监听以更新本地缓存
	if err := provider.Watch(serviceName, func(_ *iface.Topology) {}); err != nil {
		t.Fatalf("开始监听失败: %v", err)
	}

	time.Sleep(1 * time.Second)

	// 按 kind 查询
	gatewayNodes := provider.GetByKind(kind)
	if len(gatewayNodes) < 2 {
		t.Errorf("期望至少 2 个 gateway 节点, 实际 %d", len(gatewayNodes))
	}

	// 验证节点 ID
	foundIds := make(map[uint64]bool)
	for _, node := range gatewayNodes {
		foundIds[node.GetID()] = true
	}

	if !foundIds[2001] || !foundIds[2002] {
		t.Error("未找到预期的 gateway 节点")
	}

	// 清理
	for _, node := range nodes {
		_ = provider.Remove(node.Id)
	}
}

func TestProvider_GetAll(t *testing.T) {
	provider, err := consul.New(consul.WithAddress(getConsulAddr()))
	if err != nil {
		t.Skipf("跳过测试: 无法连接到 Consul: %v", err)
	}
	defer provider.Close()

	serviceName := "test-all-service"

	// 注册多个节点
	nodes := []*iface.Node{
		{
			Id:      3001,
			Name:    serviceName,
			Address: "127.0.0.1",
			Port:    10001,
			Tags:    []string{"test"},
		},
		{
			Id:      3002,
			Name:    serviceName,
			Address: "127.0.0.1",
			Port:    10002,
			Tags:    []string{"test"},
		},
	}

	for _, node := range nodes {
		if err := provider.Add(node); err != nil {
			t.Fatalf("注册节点失败: %v", err)
		}
	}

	// 等待注册和发现
	time.Sleep(2 * time.Second)

	// 开始监听
	if err := provider.Watch(serviceName, func(_ *iface.Topology) {}); err != nil {
		t.Fatalf("开始监听失败: %v", err)
	}

	time.Sleep(1 * time.Second)

	// 获取所有节点
	allNodes := provider.GetAll()
	if len(allNodes) < 2 {
		t.Errorf("期望至少 2 个节点, 实际 %d", len(allNodes))
	}

	// 验证节点 ID
	foundIds := make(map[uint64]bool)
	for _, node := range allNodes {
		if node.GetName() == serviceName {
			foundIds[node.GetID()] = true
		}
	}

	if !foundIds[3001] || !foundIds[3002] {
		t.Error("未找到预期的节点")
	}

	// 清理
	for _, node := range nodes {
		_ = provider.Remove(node.Id)
	}
}

func TestProvider_UpdateStatus(t *testing.T) {
	provider, err := consul.New(consul.WithAddress(getConsulAddr()))
	if err != nil {
		t.Skipf("跳过测试: 无法连接到 Consul: %v", err)
	}
	defer provider.Close()

	node := &iface.Node{
		Id:      4001,
		Name:    "test-status-service",
		Address: "127.0.0.1",
		Port:    11001,
		Tags:    []string{"test"},
	}

	// 注册节点
	if err := provider.Add(node); err != nil {
		t.Fatalf("注册节点失败: %v", err)
	}

	time.Sleep(3 * time.Second)

	// 更新状态
	provider.UpdateStatus(node.Id, "warning")

	// 等待状态更新
	time.Sleep(3 * time.Second)

	// 再次更新状态
	provider.UpdateStatus(node.Id, "critical")

	time.Sleep(3 * time.Second)

	// 清理
	_ = provider.Remove(node.Id)
}

func TestProvider_Close(t *testing.T) {
	provider, err := consul.New(consul.WithAddress(getConsulAddr()))
	if err != nil {
		t.Skipf("跳过测试: 无法连接到 Consul: %v", err)
	}

	// 注册一个节点
	node := &iface.Node{
		Id:      5001,
		Name:    "test-close-service",
		Address: "127.0.0.1",
		Port:    12001,
		Tags:    []string{"test"},
	}

	if err := provider.Add(node); err != nil {
		t.Fatalf("注册节点失败: %v", err)
	}

	// 开始监听
	if err := provider.Watch("test-close-service", func(_ *iface.Topology) {}); err != nil {
		t.Fatalf("开始监听失败: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	// 关闭 provider
	if err := provider.Close(); err != nil {
		t.Fatalf("关闭 provider 失败: %v", err)
	}

	// 验证关闭后不能再操作
	// 注意: 这里只是验证 Close 不会 panic，实际行为取决于实现
}

func TestProvider_ConcurrentOperations(t *testing.T) {
	provider, err := consul.New(consul.WithAddress(getConsulAddr()))
	if err != nil {
		t.Skipf("跳过测试: 无法连接到 Consul: %v", err)
	}
	defer provider.Close()

	serviceName := "test-concurrent-service"
	var wg sync.WaitGroup

	// 并发注册节点
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			node := &iface.Node{
				Id:      uint64(6000 + id),
				Name:    serviceName,
				Address: "127.0.0.1",
				Port:    13000 + id,
				Tags:    []string{"test", "concurrent"},
			}
			if err := provider.Add(node); err != nil {
				t.Errorf("并发注册节点失败: %v", err)
			}
		}(i)
	}

	wg.Wait()
	time.Sleep(5 * time.Second)

	// 开始监听
	if err := provider.Watch(serviceName, func(_ *iface.Topology) {}); err != nil {
		t.Fatalf("开始监听失败: %v", err)
	}

	time.Sleep(5 * time.Second)

	// 验证所有节点都被发现
	allNodes := provider.GetAll()
	foundCount := 0
	for _, node := range allNodes {
		if node.GetName() == serviceName {
			foundCount++
		}
	}

	if foundCount < 10 {
		t.Errorf("期望发现 10 个节点, 实际 %d", foundCount)
	}

	// 清理
	for i := 0; i < 10; i++ {
		_ = provider.Remove(uint64(6000 + i))
	}
}

// getConsulAddr 获取 Consul 地址，优先使用环境变量
func getConsulAddr() string {
	addr := "127.0.0.1:8500"
	if envAddr := getEnv("CONSUL_ADDR"); envAddr != "" {
		addr = envAddr
	}
	return addr
}

// getEnv 获取环境变量
func getEnv(key string) string {
	return os.Getenv(key)
}
