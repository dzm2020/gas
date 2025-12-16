package gas

import (
	"gas/internal/config"
	"gas/internal/iface"
	"gas/internal/node"
)

func init() {
	InitWithConfig(config.Default())
}

func Init(path string) {
	iface.SetNode(node.New(path))
}

func InitWithConfig(config *config.Config) {
	iface.SetNode(node.NewWithConfig(config))
}

func Startup(comps ...iface.IComponent) error {
	return iface.GetNode().Startup(comps...)
}
