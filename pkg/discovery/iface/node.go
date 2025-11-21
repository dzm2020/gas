package iface

type Node struct {
	Id            uint64
	Name, Address string
	Port          int
	Tags          []string
	Meta          map[string]string
}

func (b *Node) GetName() string {
	return b.Name
}

func (b *Node) GetID() uint64 {
	return b.Id
}

func (b *Node) GetAddress() string {
	return b.Address
}

func (b *Node) GetPort() int {
	return b.Port
}

func (b *Node) GetTags() []string {
	return b.Tags
}

func (b *Node) GetMeta() map[string]string {
	return b.Meta
}

type Topology struct {
	All    []*Node
	Alive  []*Node
	Joined []*Node
	Left   []*Node
}

type UpdateHandler func(_ *Topology)
