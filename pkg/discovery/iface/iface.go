package iface

import (
	"context"
	"math/rand"
	"sync/atomic"

	"golang.org/x/exp/slices"
)

type IDiscovery interface {
	Run(ctx context.Context) error
	Register(member *Member) error
	Deregister(memberId uint64) error
	GetById(memberId uint64) *Member
	GetByKind(kind string) map[uint64]*Member
	GetAll() map[uint64]*Member
	Watch(kind string, handler ServiceChangeListener)
	Unwatch(kind string, handler ServiceChangeListener)
	Shutdown(ctx context.Context) error
}

type MemberList struct {
	Dict map[uint64]*Member
}

func NewMemberList(dict map[uint64]*Member) *MemberList {
	return &MemberList{
		Dict: dict,
	}
}

func (m *MemberList) UpdateTopology(old *MemberList) *Topology {
	topology := &Topology{}

	for _, node := range m.Dict {
		if _, ok := old.Dict[node.GetID()]; ok {
			topology.Alive = append(topology.Alive, node)
		} else {
			topology.Joined = append(topology.Joined, node)
		}
		topology.All = append(topology.All, node)
	}
	for id := range old.Dict {
		if _, ok := m.Dict[id]; !ok {
			topology.Left = append(topology.Left, old.Dict[id])
		}
	}
	return topology
}

type Member struct {
	Id      uint64            `json:"id" yaml:"id"`           // 节点ID
	Kind    string            `json:"kind" yaml:"kind"`       // 节点类型
	Address string            `json:"address" yaml:"address"` // 节点地址
	Port    int               `json:"port" yaml:"port"`       // 节点端口
	Tags    []string          `json:"tags" yaml:"tags"`       // 节点标签
	Meta    map[string]string `json:"meta" yaml:"meta"`       // 节点元数据
}

func (b *Member) GetKind() string {
	return b.Kind
}

func (b *Member) GetID() uint64 {
	return b.Id
}

func (b *Member) GetAddress() string {
	return b.Address
}

func (b *Member) GetPort() int {
	return b.Port
}

func (b *Member) GetTags() []string {
	return b.Tags
}

func (b *Member) GetMeta() map[string]string {
	return b.Meta
}

func (b *Member) SetTags(tags []string) {
	b.Tags = tags
}

func (b *Member) RemoveTag(tag string) {
	slices.DeleteFunc(b.Tags, func(s string) bool {
		return s == tag
	})
}
func (b *Member) AddTag(tag string) {
	if slices.Contains(b.Tags, tag) {
		return
	}
	b.Tags = append(b.Tags, tag)
}

type Topology struct {
	All    []*Member
	Alive  []*Member
	Joined []*Member
	Left   []*Member
}

func (t *Topology) IsChange() bool {
	return len(t.Left) > 0 || len(t.Joined) > 0
}

type ServiceChangeListener func(_ *Topology)

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
