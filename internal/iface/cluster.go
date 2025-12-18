package iface

import (
	"context"
	discovery "gas/pkg/discovery/iface"
)

type ICluster interface {
	Send(message *ActorMessage) (err error)
	Call(message *ActorMessage) (data []byte, err error)
	Start(ctx context.Context) error
	Select(service string, strategy discovery.RouteStrategy) uint64
	UpdateMember() error
	Shutdown(ctx context.Context) error
}
