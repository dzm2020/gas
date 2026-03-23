package consul

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dzm2020/gas/pkg/discovery/iface"
	"github.com/dzm2020/gas/pkg/glog"
	"go.uber.org/zap"

	"go.uber.org/zap/zapcore"
)

func onNodeChangeHandler(topology *iface.Topology) {
	glog.Info("节点发生变化", zap.Any("topology", topology))
}

// TestDefaultConfig 测试默认配置
func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Address != "127.0.0.1:8500" || cfg.WatchWaitTime != 1*time.Second {
		t.Error("默认配置值不正确")
	}
}

// TestRegistrar 测试注册器
func TestRegistrar(t *testing.T) {
	glog.SetLogLevel(zapcore.DebugLevel)
	provider := New(DefaultConfig())
	provider.Run(context.Background())
	defer func() {
		provider.Shutdown(context.Background())
		time.Sleep(1 * time.Second)
	}()

	kind := fmt.Sprintf("consul-registrar-%d", time.Now().UnixNano())
	memberID := uint64(time.Now().UnixNano())
	member := &iface.Member{Id: memberID, Kind: kind, Address: "127.0.0.1", Port: 8080}
	provider.Watch(kind, onNodeChangeHandler)

	if err := provider.Register(member); err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Second)
	if provider.GetById(memberID) == nil {
		t.Fatal("GetById after Register")
	}
	member.Port = 8081
	if err := provider.Update(member); err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Second)
	if list := provider.GetByKind(kind); len(list) != 1 {
		t.Fatalf("GetByKind(%q) want 1, got %d", kind, len(list))
	}

	if err := provider.Deregister(member.GetID()); err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Second)
	provider.Unwatch(kind, onNodeChangeHandler)

	_ = provider.Deregister(member.GetID())

	if list := provider.GetByKind(kind); len(list) != 0 {
		t.Fatalf("GetByKind(%q) after Deregister want 0, got %d", kind, len(list))
	}

	if err := provider.Register(member); err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * time.Second)
}

func TestRegistrarConcurrency(t *testing.T) {
	provider := New(DefaultConfig())
	provider.Run(context.Background())

	defer provider.Shutdown(context.Background())

	var genId atomic.Uint64

	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 10; j++ {
				member := &iface.Member{Id: 1, Kind: "test", Address: "127.0.0.1", Port: 8080}
				member.Id = genId.Add(1)
				provider.Register(member)
			}
		}()
	}
	time.Sleep(time.Second * 10)
}

func BenchmarkDiscovery(b *testing.B) {
	provider := New(DefaultConfig())
	provider.Run(context.Background())

	defer provider.Shutdown(context.Background())

	var genId atomic.Uint64
	for i := 0; i < b.N; i++ {
		member := &iface.Member{Id: 1, Kind: "test", Address: "127.0.0.1", Port: 8080}
		member.Id = genId.Add(1)
		provider.Register(member)
	}
}

func BenchmarkGet(b *testing.B) {
	provider := New(DefaultConfig())
	_ = provider.Run(context.Background())
	defer provider.Shutdown(context.Background())

	for i := uint64(0); i < 100; i++ {
		member := &iface.Member{Id: 1, Kind: "test", Address: "127.0.0.1", Port: 8080}
		member.Id += i
		_ = provider.Register(member)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		provider.GetById(uint64(i))
		provider.GetByKind("test")
		provider.GetAll()
	}
}
