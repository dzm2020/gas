//go:build integration

// 集成测试：需 ../conf/gate-node-config.yaml 及 Consul/NATS 等；Startup 会阻塞，默认不参与 go test ./...
// 运行：go test -tags=integration ./examples/cluster/gate-node/...

package gate_node

import (
	"testing"

	"github.com/dzm2020/gas"
	. "github.com/dzm2020/gas/examples/cluster/common"
	"github.com/dzm2020/gas/examples/cluster/gate-node/services"
	"github.com/dzm2020/gas/internal/component/gate"
)

// TestGate 为集成测试：会加载 ../conf/gate-node-config.yaml，需 consul、nats 等依赖。
// 若本地未启动发现与消息队列，测试会 Skip。
func TestGate(t *testing.T) {
	Node = gas.Configure("../conf/node_gate.yaml")

	gateComponent := gate.NewComponent(&services.BusinessHandler{})

	_ = Node.Startup(gateComponent)
}
