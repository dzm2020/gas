package component

import (
	"context"

	"github.com/dzm2020/gas/internal/iface"
	"github.com/dzm2020/gas/internal/profile"
	logger "github.com/dzm2020/gas/pkg/glog"
	"github.com/dzm2020/gas/pkg/lib/component"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	Logger = "logger"
)

// LoggerComponent glog 日志组件
type LoggerComponent struct {
	component.BaseComponent[iface.INode]
	panicHook func(entry zapcore.Entry)
}

// NewLoggerComponent 创建 glog 组件
func NewLoggerComponent(panicHook func(entry zapcore.Entry)) *LoggerComponent {
	return &LoggerComponent{
		panicHook: panicHook,
	}
}

func (c *LoggerComponent) Name() string {
	return Logger
}

func (c *LoggerComponent) Start(ctx context.Context, node iface.INode) error {
	conf := logger.DefaultConfig()
	if err := profile.Get(c.Name(), conf); err != nil {
		return err
	}

	logger.Init(conf)

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

func (c *LoggerComponent) Stop(ctx context.Context) error {
	if err := logger.Stop(); err != nil {
		return err
	}
	return nil
}
