package iface

import (
	"github.com/dzm2020/gas/pkg/cluster"
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
		Cluster() cluster.ICluster
		Startup(comps ...component.IComponent[INode]) error
		Serializer() lib.ISerializer
	}
)
