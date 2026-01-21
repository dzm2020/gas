package iface

import (
	"context"
	discovery "github.com/dzm2020/gas/pkg/discovery/iface"
)

type ICluster interface {
	Send(message *ActorMessage) (err error)
	Call(message *ActorMessage) (data []byte, err error)
	Start(ctx context.Context) error
	Select(name string, strategy discovery.RouteStrategy) uint64
	UpdateMember() error
	Shutdown(ctx context.Context) error
}
