package iface

import discovery "gas/pkg/discovery/iface"

type INode interface {
	GetId() uint64
	GetSystem() ISystem
	GetRemote() IRemote
	GetDiscovery() discovery.IDiscovery
}
