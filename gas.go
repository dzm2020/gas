package gas

import (
	"github.com/dzm2020/gas/internal/iface"
	"github.com/dzm2020/gas/internal/node"
)

func Configure(configPath string) iface.INode {
	n := node.New(configPath)
	return n
}
