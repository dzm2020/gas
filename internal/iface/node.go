package iface

import (
	"gas/internal/config"
	"gas/pkg/component"
	discovery "gas/pkg/discovery/iface"
	"gas/pkg/lib"

	"go.uber.org/zap/zapcore"
)

type INode interface {
	GetId() uint64
	SetSerializer(ser lib.ISerializer)
	GetSerializer() lib.ISerializer
	GetActorSystem() ISystem
	SetActorSystem(system ISystem)
	GetRemote() IRemote
	SetRemote(IRemote)
	GetConfig() *config.Config
	Self() *discovery.Node
	StarUp(profileFilePath string, comps ...component.Component) error
	SetPanicHook(panicHook func(entry zapcore.Entry))
}
