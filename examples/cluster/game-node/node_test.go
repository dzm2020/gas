package node1

import (
	"context"
	"testing"

	"github.com/dzm2020/gas"
	. "github.com/dzm2020/gas/examples/cluster/common"
	"github.com/dzm2020/gas/examples/cluster/game-node/services"
	"github.com/dzm2020/gas/internal/iface"
	"github.com/dzm2020/gas/pkg/lib/component"
)

func TestGameNode(t *testing.T) {
	Node = gas.Configure("../conf/node_game.yaml")

	_ = Node.Startup(&GameNode{})
}

type GameNode struct {
	component.BaseComponent[iface.INode]
}

func (g *GameNode) Name() string {
	return "GameNode"
}

func (g *GameNode) Start(ctx context.Context, node iface.INode) error {
	system := node.System()
	system.Spawn(services.NewGameActor())
	return nil
}
