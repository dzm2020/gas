package iface

type NodeList struct {
	Dict map[uint64]*Node
}

func NewNodeList(dict map[uint64]*Node) *NodeList {
	return &NodeList{
		Dict: dict,
	}
}

func (m *NodeList) UpdateTopology(old *NodeList) *Topology {
	topology := &Topology{}

	for _, node := range m.Dict {
		if _, ok := old.Dict[node.GetID()]; ok {
			topology.Alive = append(topology.Alive, node)
		} else {
			topology.Joined = append(topology.Joined, node)
		}
		topology.All = append(topology.All, node)
	}
	for id := range old.Dict {
		if _, ok := m.Dict[id]; !ok {
			topology.Left = append(topology.Left, m.Dict[id])
		}
	}
	return topology
}
