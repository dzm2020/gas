package iface

import (
	"context"
	discovery "gas/pkg/discovery/iface"
	"time"
)

type RouteStrategy func(nodes []*discovery.Member) *discovery.Member

type IRemote interface {
	Start(ctx context.Context) error
	Send(message *ActorMessage) error
	Call(message *ActorMessage, timeout time.Duration) *Response
	Select(service string, strategy RouteStrategy) *Pid
	UpdateNode() error
	Shutdown(ctx context.Context) error
}
