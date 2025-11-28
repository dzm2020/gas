package node

import (
	"context"
	"fmt"
	"gas/pkg/component"
	glog2 "gas/pkg/glog"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// GlogComponent glog 日志组件
type GlogComponent struct {
	name string
	node *Node
}

// NewGlogComponent 创建 glog 组件
func NewGlogComponent(name string, node *Node) component.Component {
	return &GlogComponent{
		name: name,
		node: node,
	}
}
func (g *GlogComponent) Name() string {
	return g.name
}
func (g *GlogComponent) Start(ctx context.Context) error {
	if g.node == nil {
		return fmt.Errorf("node is nil")
	}

	cfg := g.node.GetConfig()
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}

	// 使用节点配置中的 glog 配置初始化
	if err := glog2.InitFromConfig(&cfg.Glog); err != nil {
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
				if g.node.panicHook != nil {
					g.node.panicHook(entry)
				}
			}
			return nil
		}),
	}
	glog2.WithOptions(options...)
	return nil
}
func (g *GlogComponent) Stop(ctx context.Context) error {
	glog2.Stop()
	return nil
}
