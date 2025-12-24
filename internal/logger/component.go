package logger

import (
	"context"
	"gas/internal/iface"
	logger "gas/pkg/glog"
	"gas/pkg/lib/component"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	ComponentName = "logger"
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
	panicHook func(entry zapcore.Entry)
}

// NewComponent 创建 glog 组件
func NewComponent(panicHook func(entry zapcore.Entry)) *Component {
	return &Component{
		panicHook: panicHook,
	}
}

func (c *Component) Name() string {
	return ComponentName
}

func (c *Component) Start(ctx context.Context, node iface.INode) error {
	config := defaultConfig()
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
				if c.panicHook != nil {
					c.panicHook(entry)
				}
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
