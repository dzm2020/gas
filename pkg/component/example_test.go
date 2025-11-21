package component_test

import (
	"context"
	"fmt"
	"time"

	"gas/pkg/component"
)

// ExampleComponent 示例组件
type ExampleComponent struct {
	name string
}

func (e *ExampleComponent) Name() string {
	return e.name
}

func (e *ExampleComponent) Start(ctx context.Context) error {
	fmt.Printf("Starting component: %s\n", e.name)
	// 模拟启动逻辑
	time.Sleep(100 * time.Millisecond)
	return nil
}

func (e *ExampleComponent) Stop(ctx context.Context) error {
	fmt.Printf("Stopping component: %s\n", e.name)
	// 模拟停止逻辑
	time.Sleep(50 * time.Millisecond)
	return nil
}

func ExampleManager() {
	manager := component.New()

	// 注册组件
	manager.Register(&ExampleComponent{name: "database"})
	manager.Register(&ExampleComponent{name: "cache"})
	manager.Register(&ExampleComponent{name: "server"})

	// 启动所有组件
	ctx := context.Background()
	if err := manager.Start(ctx); err != nil {
		fmt.Printf("Failed to start: %v\n", err)
		return
	}

	// 运行一段时间...
	time.Sleep(1 * time.Second)

	// 停止所有组件
	if err := manager.Stop(ctx); err != nil {
		fmt.Printf("Failed to stop: %v\n", err)
	}

	// Output:
	// Starting component: database
	// Starting component: cache
	// Starting component: server
	// Stopping component: server
	// Stopping component: cache
	// Stopping component: database
}
