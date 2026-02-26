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
	LoggerName = "logger"
)

// Logger glog 日志组件
type Logger struct {
	component.BaseComponent[iface.INode]
	panicHook func(entry zapcore.Entry)
}

// NewLogger 创建 glog 组件
func NewLogger(panicHook func(entry zapcore.Entry)) *Logger {
	return &Logger{
		panicHook: panicHook,
	}
}

func (c *Logger) Name() string {
	return LoggerName
}

func (c *Logger) Start(ctx context.Context, node iface.INode) error {
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

func (c *Logger) Stop(ctx context.Context) error {
	if err := logger.Stop(); err != nil {
		return err
	}
	return nil
}
