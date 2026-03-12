package logger

import (
	"context"

	"github.com/dzm2020/gas/internal/iface"
	"github.com/dzm2020/gas/pkg/glog"
	"github.com/dzm2020/gas/pkg/lib/component"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const Name = "logger"

// Logger glog 日志组件
type Logger struct {
	component.BaseComponent[iface.INode]
	panicHook func(entry zapcore.Entry)
}

// New 创建 glog 组件
func New(panicHook func(entry zapcore.Entry)) *Logger {
	return &Logger{
		panicHook: panicHook,
	}
}

func (c *Logger) Name() string {
	return Name
}

func (c *Logger) Start(ctx context.Context, node iface.INode) error {
	conf := node.Profile().GetLogger()
	glog.Init(conf)
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
	glog.WithOptions(options...)
	return nil
}

func (c *Logger) Stop(ctx context.Context) error {
	if err := glog.Stop(); err != nil {
		return err
	}
	return nil
}
