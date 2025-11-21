package iface

type IDiscovery interface {
	GetById(nodeId uint64) *Node
	GetByKind(kind string) []*Node
	GetAll() []*Node
	Add(node *Node) error
	Remove(nodeId uint64) error
	Watch(name string, handler UpdateHandler) error
	Unwatch(name string, handler UpdateHandler) bool
	Close() error
}
