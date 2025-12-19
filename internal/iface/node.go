package iface

import (
	"context"
	"gas/internal/config"
	discovery "gas/pkg/discovery/iface"
	"gas/pkg/lib"

	"go.uber.org/zap/zapcore"
)

type (
	IComponent interface {
		Start(ctx context.Context, node INode) error
		Stop(ctx context.Context) error
		Name() string
	}

	IComponentManager interface {
		ComponentCount() int
		GetComponent(name string) IComponent
		GetComponentNames() []string
		Register(component IComponent) error
	}

	INode interface {
		IComponentManager
		discovery.IMember
		lib.ISerializer
		Info() *discovery.Member
		SetSerializer(ser lib.ISerializer)
		SetPanicHook(panicHook func(entry zapcore.Entry))
		CallPanicHook(entry zapcore.Entry)
		System() ISystem
		SetSystem(system ISystem)
		Cluster() ICluster
		SetCluster(ICluster)
		GetConfig() *config.Config
		Startup(comps ...IComponent) error
	}

	BaseComponent struct {
	}
)

func (*BaseComponent) Start(ctx context.Context) error {
	return nil
}
func (*BaseComponent) Stop(ctx context.Context) error {
	return nil
}
