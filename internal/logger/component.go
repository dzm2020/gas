package logger

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
	conf := defaultConfig()
	if err := profile.Get(c.Name(), conf); err != nil {
		return err
	}
	// 使用节点配置中的 glog 配置初始化
	if err := logger.InitFromConfig(conf); err != nil {
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
	if err := logger.Stop(); err != nil {
		// Sync() 在标准输出/错误时会返回错误，这是 zap 的已知行为，可以忽略
		// 但对于文件写入器，应该记录错误
		return err
	}
	return nil
}
