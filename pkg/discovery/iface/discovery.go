package iface

type IDiscovery interface {
	GetById(nodeId uint64) *Node
	GetService(service string) []*Node
	GetAll() []*Node
	Add(node *Node) error
	Remove(nodeId uint64) error
	Subscribe(name string, handler ServiceChangeListener) error
	Unsubscribe(name string, handler ServiceChangeListener)
	Close() error
}
