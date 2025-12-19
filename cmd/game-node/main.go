package main

import (
	"context"
	"gas"
	"gas/examples/cluster/common"
	"gas/examples/cluster/game-node/services/usermgr"
	"gas/internal/iface"
)

func main() {
	// 初始化节点
	gas.Init("../examples/cluster/conf/game-node-config.json")

	// 创建游戏节点组件
	gameNode := &GameNode{}

	// 启动节点
	if err := gas.Startup(gameNode); err != nil {
		panic(err)
	}
}

// GameNode 游戏节点组件
type GameNode struct {
	iface.BaseComponent
}

func (g *GameNode) Name() string {
	return "GameNode"
}

func (g *GameNode) Start(ctx context.Context) error {
	system := iface.GetNode().System()
	// 启动用户管理服务
	system.Spawn(usermgr.New())
	return nil
}




