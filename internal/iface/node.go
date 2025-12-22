package iface

import (
	discovery "gas/pkg/discovery/iface"
	"gas/pkg/lib"
	"gas/pkg/lib/component"

	"go.uber.org/zap/zapcore"
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
		SetTags(tags []string)
		RemoteTag(tag string)
		AddTag(tag string)
	}

	INode interface {
		IMember
		lib.ISerializer
		component.IManager[INode]
		Info() *Member
		SetSerializer(ser lib.ISerializer)
		SetPanicHook(panicHook func(entry zapcore.Entry))
		CallPanicHook(entry zapcore.Entry)
		System() ISystem
		SetSystem(system ISystem)
		Cluster() ICluster
		SetCluster(ICluster)
		Startup(comps ...component.IComponent[INode]) error
		GetConfig(key string, cfg interface{}) error
		SetDefaultConfig(key string, cfg interface{})
	}
)
