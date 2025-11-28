package app

import (
	"context"
	"fmt"
	"gas/internal/iface"
	"gas/pkg/component"
	"gas/pkg/glog"
)

// Ensure Component implements component.Component interface
var _ component.Component = (*Component)(nil)

// Component app 组件，用于管理一组 actor
type Component struct {
	name   string
	config *Config
	node   iface.INode
}

// New 创建 app 组件
func New(name string, config *Config, node iface.INode) *Component {
	if config == nil {
		config = &Config{
			Services: []ServiceConfig{},
		}
	}
	return &Component{
		name:   name,
		config: config,
		node:   node,
	}
}

func (c *Component) Name() string {
	return c.name
}

func (c *Component) Start(ctx context.Context) error {
	if c.node == nil {
		return fmt.Errorf("node is nil")
	}

	actorSystem := c.node.GetActorSystem()
	if actorSystem == nil {
		return fmt.Errorf("actor system not initialized")
	}

	// 启动所有配置的 actor
	for _, actorConfig := range c.config.Services {
		if actorConfig.Actor == nil {
			glog.Warnf("app: actor '%s' has nil actor instance, skipping", actorConfig.Name)
			continue
		}
		// 构建启动选项
		var options []iface.Option
		if len(actorConfig.Name) > 0 {
			options = append(options, iface.WithName(actorConfig.Name))
		}
		if len(actorConfig.Params) > 0 {
			options = append(options, iface.WithParams(actorConfig.Params))
		}
		if actorConfig.Router != nil {
			options = append(options, iface.WithRouter(actorConfig.Router))
		}
		if len(actorConfig.Middlewares) > 0 {
			options = append(options, iface.WithMiddlewares(actorConfig.Middlewares))
		}
		// 启动 actor
		pid, process := actorSystem.Spawn(actorConfig.Actor, options...)
		if process == nil {
			return fmt.Errorf("spawn actor '%s' failed", actorConfig.Name)
		}
		glog.Infof("app: actor '%s' started with pid=%d", actorConfig.Name, pid.GetServiceId())
	}
	return nil
}

func (c *Component) Stop(ctx context.Context) error {
	return nil
}
