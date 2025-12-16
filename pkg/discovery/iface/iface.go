package iface

import "context"

type IDiscovery interface {
	Run(ctx context.Context) error
	GetById(memberId uint64) *Member
	GetByKind(kind string) []*Member
	GetAll() []*Member
	Add(member *Member) error
	Remove(memberId uint64) error
	Watch(kind string, handler ServiceChangeListener) error
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
			topology.Left = append(topology.Left, m.Dict[id])
		}
	}
	return topology
}

type IMember interface {
	GetKind() string
	GetID() uint64
	GetAddress() string
	GetPort() int
	GetTags() []string
	GetMeta() map[string]string
	SetTags(tags []string)
}

type Member struct {
	Id            uint64
	Kind, Address string
	Port          int
	Tags          []string
	Meta          map[string]string
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

type Topology struct {
	All    []*Member
	Alive  []*Member
	Joined []*Member
	Left   []*Member
}

type ServiceChangeListener func(_ *Topology)
