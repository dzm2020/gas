package node

import (
	"context"
	"gas/internal/iface"
	"gas/pkg/lib/glog"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// LogComponent glog 日志组件
type LogComponent struct {
	node *Node
}

// NewLogComponent 创建 glog 组件
func NewLogComponent() iface.IComponent {
	return &LogComponent{}
}

func (m *LogComponent) Name() string {
	return "log"
}
func (m *LogComponent) Start(ctx context.Context, node iface.INode) error {
	m.node = node.(*Node)
	cfg := m.node.GetConfig()

	// 使用节点配置中的 glog 配置初始化
	if err := glog.InitFromConfig(&cfg.Logger); err != nil {
		return err
	}
	options := []zap.Option{
		zap.Fields(zap.String("nodeKind", m.node.GetKind()), zap.Uint64("nodeId", m.node.GetID())),
		zap.Hooks(func(entry zapcore.Entry) error {
			if entry.Level >= zap.DPanicLevel {
				m.node.CallPanicHook(entry)
			}
			return nil
		}),
	}
	glog.WithOptions(options...)
	return nil
}
func (m *LogComponent) Stop(ctx context.Context) error {
	glog.Stop()
	return nil
}
