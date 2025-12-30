package iface

import (
	"context"
)

type (
	IDiscovery interface {
		Run(ctx context.Context) error
		Register(member *Member) error
		Deregister(memberId uint64) error
		GetById(memberId uint64) *Member
		GetByKind(kind string) map[uint64]*Member
		GetAll() map[uint64]*Member
		Watch(kind string, handler ServiceChangeHandler)
		Unwatch(kind string, handler ServiceChangeHandler)
		Shutdown(ctx context.Context) error
	}

	ServiceChangeHandler func(_ *Topology)
)

func NewMemberList(dict map[uint64]*Member) *MemberList {
	return &MemberList{
		Dict: dict,
	}
}

type MemberList struct {
	Dict map[uint64]*Member
}

func (m *MemberList) UpdateTopology(old *MemberList) *Topology {
	topology := &Topology{}

	if old == nil {
		old = NewMemberList(nil)
	}
	for _, member := range m.Dict {
		if _, ok := old.Dict[member.GetID()]; ok {
			topology.Alive = append(topology.Alive, member)
		} else {
			topology.Joined = append(topology.Joined, member)
		}
		topology.All = append(topology.All, member)
	}
	for id := range old.Dict {
		if _, ok := m.Dict[id]; !ok {
			topology.Left = append(topology.Left, old.Dict[id])
		}
	}
	return topology
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
