package iface

import (
	discovery "github.com/dzm2020/gas/pkg/discovery/iface"
	"github.com/dzm2020/gas/pkg/lib"
	"github.com/dzm2020/gas/pkg/lib/component"
)

type (
	Member = discovery.Member

	IMember interface {
		GetKind() string
		GetID() uint64
		GetAddress() string
		GetPort() int
		GetTags() []string
		GetMeta() map[string]string
	}

	INode interface {
		IMember
		lib.ISerializer
		component.IManager[INode]
		Info() *Member
		SetSerializer(ser lib.ISerializer)
		System() ISystem
		SetSystem(system ISystem)
		Cluster() ICluster
		SetCluster(ICluster)
		Startup(comps ...component.IComponent[INode]) error
	}
)
