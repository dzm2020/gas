package logger

import (
	"context"
	"gas/internal/iface"
	logger "gas/pkg/glog"
	"gas/pkg/lib/component"

	"github.com/duke-git/lancet/v2/convertor"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func defaultConfig() *logger.Config {
	return &logger.Config{
		Path:         "./logs/app.log",
		Level:        "info",
		PrintConsole: true,
		File: logger.FileConfig{
			MaxSize:    500,
			MaxBackups: 100,
			MaxAge:     30,
			Compress:   false,
			LocalTime:  true,
		},
	}
}

// Component glog 日志组件
type Component struct {
	component.BaseComponent[iface.INode]
}

// NewComponent 创建 glog 组件
func NewComponent() *Component {
	return &Component{}
}

func (c *Component) Name() string {
	return "logger"
}

func (c *Component) Start(ctx context.Context, node iface.INode) error {
	config := convertor.DeepClone(defaultConfig())
	if err := node.GetConfig(c.Name(), config); err != nil {
		return err
	}
	// 使用节点配置中的 glog 配置初始化
	if err := logger.InitFromConfig(config); err != nil {
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

func (c *Component) Stop(ctx context.Context) error {
	logger.Stop()
	return nil
}
