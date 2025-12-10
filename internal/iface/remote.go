package iface

import (
	discovery "gas/pkg/discovery/iface"
	"time"
)

type RouteStrategy func(nodes []*discovery.Node) *discovery.Node

type IRemote interface {
	Send(message *Message) error
	Request(message *Message, timeout time.Duration) *Response
	Select(service string, strategy RouteStrategy) (*Pid, error)
	RegistryName(name string) error
	UnregisterName(name string) error
}
