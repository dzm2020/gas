package app_test

import (
	"context"
	"fmt"
	"gas/internal/app"
	"gas/internal/iface"
	"gas/internal/node"
	"gas/pkg/component"
	"testing"
)

// ExampleActor 示例 actor
type ExampleActor struct {
	iface.Actor
}

func (a *ExampleActor) OnInit(ctx iface.IContext, params []interface{}) error {
	fmt.Printf("ExampleActor initialized with params: %v\n", params)
	return nil
}

func (a *ExampleActor) OnMessage(ctx iface.IContext, msg interface{}) error {
	fmt.Printf("ExampleActor received message: %v\n", msg)
	return nil
}

// ExampleRequest 示例请求消息
type ExampleRequest struct {
	Message string
}

// ExampleResponse 示例响应消息
type ExampleResponse struct {
	Result string
}

func TestAppStart(t *testing.T) {
	// 创建节点
	n := node.New()

	// 创建 app 配置
	config := &app.Config{
		Service: []app.ServiceConfig{
			{
				Name:   "example-actor-1",
				Actor:  &ExampleActor{},
				Params: []interface{}{"param1", "param2"},
			},
			{
				Name:   "example-actor-2",
				Actor:  &ExampleActor{},
				Params: []interface{}{"param3"},
			},
		},
	}

	// 创建 app 组件
	appComponent := app.New("my-app", config, n)

	// 创建组件管理器
	manager := component.New()
	manager.Register(appComponent)

	// 启动组件
	ctx := context.Background()
	if err := manager.Start(ctx); err != nil {
		fmt.Printf("Failed to start: %v\n", err)
		return
	}

	// 运行一段时间...
	// ...

	// 停止组件
	if err := manager.Stop(ctx); err != nil {
		fmt.Printf("Failed to stop: %v\n", err)
	}
}
