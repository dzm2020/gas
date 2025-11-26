package remote

import (
	discovery "gas/pkg/discovery/iface"
	"math/rand"
	"sync/atomic"
)

// RouteStrategy 路由策略函数，从节点列表中选择一个节点
type RouteStrategy func(nodes []*discovery.Node) *discovery.Node

// RouteRandom 随机路由策略
func RouteRandom(nodes []*discovery.Node) *discovery.Node {
	if len(nodes) == 0 {
		return nil
	}
	return nodes[rand.Intn(len(nodes))]
}

// RouteRoundRobin 轮询路由策略（需要外部维护状态）
func RouteRoundRobin(counter *uint64) RouteStrategy {
	return func(nodes []*discovery.Node) *discovery.Node {
		if len(nodes) == 0 {
			return nil
		}
		idx := int(atomic.AddUint64(counter, 1) % uint64(len(nodes)))
		return nodes[idx]
	}
}

// RouteFirst 选择第一个节点
func RouteFirst(nodes []*discovery.Node) *discovery.Node {
	if len(nodes) == 0 {
		return nil
	}
	return nodes[0]
}
