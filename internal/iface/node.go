package iface

import (
	"gas/internal/config"
	"gas/pkg/component"
	discovery "gas/pkg/discovery/iface"
	"gas/pkg/utils/serializer"
)

type INode interface {
	GetId() uint64
	SetSerializer(ser serializer.ISerializer)
	GetSerializer() serializer.ISerializer
	GetActorSystem() ISystem
	SetActorSystem(system ISystem)
	GetRemote() IRemote
	SetRemote(IRemote)
	GetConfig() *config.Config
	Self() *discovery.Node
	StarUp(profileFilePath string, comps ...component.Component) error
	Shutdown() error
}
