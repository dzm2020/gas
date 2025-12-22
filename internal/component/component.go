package component

import (
	"context"
	"gas/internal/iface"
	logger "gas/pkg/glog"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// LogComponent glog 日志组件
type LogComponent struct {
}

// NewLogComponent 创建 glog 组件
func NewLogComponent() iface.IComponent {
	return &LogComponent{}
}

func (c *LogComponent) Name() string {
	return "log"
}
func (c *LogComponent) Start(ctx context.Context, node iface.INode) error {

	cfg := node.GetConfig()

	// 使用节点配置中的 glog 配置初始化
	if err := logger.InitFromConfig(cfg.Logger); err != nil {
		return err
	}
	options := []zap.Option{
		zap.Fields(zap.String("nodeKind", node.GetKind()), zap.Uint64("nodeId", node.GetID())),
		zap.Hooks(func(entry zapcore.Entry) error {
			if entry.Level >= zap.DPanicLevel {
				node.CallPanicHook(entry)
			}
			return nil
		}),
	}
	logger.WithOptions(options...)
	return nil
}
func (c *LogComponent) Stop(ctx context.Context) error {
	logger.Stop()
	return nil
}
