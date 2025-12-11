package node

import (
	"context"
	"fmt"
	"gas/internal/iface"
	"gas/pkg/lib/glog"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// GlogComponent glog 日志组件
type GlogComponent struct {
	node iface.INode
}

// NewGlogComponent 创建 glog 组件
func NewGlogComponent() iface.Component {
	return &GlogComponent{}
}

func (g *GlogComponent) Name() string {
	return "log"
}
func (g *GlogComponent) Start(ctx context.Context, node iface.INode) error {
	g.node = node

	cfg := g.node.GetConfig()
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}

	// 使用节点配置中的 glog 配置初始化
	if err := glog.InitFromConfig(&cfg.Glog); err != nil {
		return fmt.Errorf("init glog from config failed: %w", err)
	}
	self := g.node.Self()
	options := []zap.Option{
		zap.Fields(
			zap.String("nodeName", self.GetName()),
			zap.Uint64("nodeId", self.GetID()),
		),
		zap.Hooks(func(entry zapcore.Entry) error {
			if entry.Level >= zap.DPanicLevel {
				//  todo
				//if g.node.panicHook != nil {
				//	g.node.panicHook(entry)
				//}
			}
			return nil
		}),
	}
	glog.WithOptions(options...)
	return nil
}
func (g *GlogComponent) Stop(ctx context.Context) error {
	glog.Stop()
	return nil
}
