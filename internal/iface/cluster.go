package iface

import (
	"context"
	discovery "gas/pkg/discovery/iface"
	"math/rand"
	"sync/atomic"
)

type RouteStrategy func(nodes []*discovery.Member) *discovery.Member

type ICluster interface {
	Send(message *ActorMessage) (err error)
	Call(message *ActorMessage) (data []byte, err error)
	Start(ctx context.Context) error
	Select(service string, strategy RouteStrategy) *Pid
	UpdateNode() error
	Shutdown(ctx context.Context) error
}

// RouteStrategy 路由策略函数，从节点列表中选择一个节点

// RouteRandom 随机路由策略
func RouteRandom(nodes []*discovery.Member) *discovery.Member {
	if len(nodes) == 0 {
		return nil
	}
	return nodes[rand.Intn(len(nodes))]
}

// RouteRoundRobin 轮询路由策略（需要外部维护状态）
func RouteRoundRobin(counter *uint64) RouteStrategy {
	return func(nodes []*discovery.Member) *discovery.Member {
		if len(nodes) == 0 {
			return nil
		}
		idx := int(atomic.AddUint64(counter, 1) % uint64(len(nodes)))
		return nodes[idx]
	}
}

// RouteFirst 选择第一个节点
func RouteFirst(nodes []*discovery.Member) *discovery.Member {
	if len(nodes) == 0 {
		return nil
	}
	return nodes[0]
}
