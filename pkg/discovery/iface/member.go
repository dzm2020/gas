package iface

type Member struct {
	Id      uint64            `json:"id" yaml:"id"`           // 节点ID
	Kind    string            `json:"kind" yaml:"kind"`       // 节点类型
	Address string            `json:"address" yaml:"address"` // 节点地址
	Port    int               `json:"port" yaml:"port"`       // 节点端口
	Tags    []string          `json:"tags" yaml:"tags"`       // 节点标签
	Meta    map[string]string `json:"meta" yaml:"meta"`       // 节点元数据
}

func (b *Member) GetKind() string {
	return b.Kind
}
func (b *Member) GetID() uint64 {
	return b.Id
}
func (b *Member) GetAddress() string {
	return b.Address
}
func (b *Member) GetPort() int {
	return b.Port
}
func (b *Member) GetTags() []string {
	return b.Tags
}
func (b *Member) GetMeta() map[string]string {
	return b.Meta
}

// Equal 比较两个 Member 是否相等
func (b *Member) Equal(other *Member) bool {
	if b == nil && other == nil {
		return true
	}
	if b == nil || other == nil {
		return false
	}
	if b.Id != other.Id {
		return false
	}
	if b.Kind != other.Kind {
		return false
	}
	if b.Address != other.Address {
		return false
	}
	if b.Port != other.Port {
		return false
	}
	// 比较 Tags 切片
	if len(b.Tags) != len(other.Tags) {
		return false
	}
	for i := range b.Tags {
		if b.Tags[i] != other.Tags[i] {
			return false
		}
	}
	// 比较 Meta map
	if len(b.Meta) != len(other.Meta) {
		return false
	}
	for k, v := range b.Meta {
		if other.Meta[k] != v {
			return false
		}
	}
	return true
}
