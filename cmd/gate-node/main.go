package main

import (
	"gas"
	"gas/examples/cluster/gate-node/services/agent"
	"gas/internal/gate"
)

func main() {
	// 初始化节点
	gas.Init("../examples/cluster/conf/gate-node-config.json")

	// 创建网关组件
	gateComponent := &gate.Gate{
		Address: "tcp://127.0.0.1:9002",
		Factory: agent.NewAgent,
	}

	// 启动节点
	if err := gas.Startup(gateComponent); err != nil {
		panic(err)
	}
}

