package iface

import (
	"math/rand"
	"sync/atomic"
)

// RouteStrategy 路由策略函数，从节点列表中选择一个节点
type RouteStrategy func(members []*Member) *Member

// RouteRandom 随机路由策略
func RouteRandom(members []*Member) *Member {
	if len(members) == 0 {
		return nil
	}
	return members[rand.Intn(len(members))]
}

// RouteRoundRobin 轮询路由策略（需要外部维护状态）
func RouteRoundRobin(counter *uint64) RouteStrategy {
	return func(members []*Member) *Member {
		if len(members) == 0 {
			return nil
		}
		idx := int(atomic.AddUint64(counter, 1) % uint64(len(members)))
		return members[idx]
	}
}

// RouteFirst 选择第一个节点
func RouteFirst(members []*Member) *Member {
	if len(members) == 0 {
		return nil
	}
	return members[0]
}
