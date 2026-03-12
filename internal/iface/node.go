package iface

import (
	"github.com/dzm2020/gas/pkg/cluster"
	discovery "github.com/dzm2020/gas/pkg/discovery/iface"
	"github.com/dzm2020/gas/pkg/glog"
	"github.com/dzm2020/gas/pkg/lib/component"
	"github.com/dzm2020/gas/pkg/lib/serializer"
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

	// IProfile 节点配置加载器，由 Profile 组件实现；GetCluster/GetLogger 失败时内部 Fatal。
	IProfile interface {
		Get(key string, cfg interface{}) error
		GetCluster() *cluster.Config
		GetLogger() *glog.Config
		IsSingleNodeMode() bool
	}

	INode interface {
		IMember
		component.IManager[INode]
		Info() *Member
		Serializer() serializer.ISerializer
		SetSerializer(ser serializer.ISerializer)
		System() ISystem
		Cluster() cluster.ICluster
		Profile() IProfile
		Startup(comps ...component.IComponent[INode]) error
	}
)

type ICluster = cluster.ICluster
