package redis

import (
	"context"

	"github.com/dzm2020/gas/internal/profile"

	"github.com/dzm2020/gas/internal/iface"
	"github.com/dzm2020/gas/pkg/lib/component"
)

const ComponentName = "redis"

// Component Redis 组件
type Component struct {
	component.BaseComponent[iface.INode]
}

// NewComponent 创建新的 Redis 组件实例
func NewComponent() *Component {
	return &Component{}
}

// Name 返回组件名称
func (c *Component) Name() string {
	return ComponentName
}

// Start 启动组件，初始化所有 Redis 连接并加载脚本
func (c *Component) Start(ctx context.Context, node iface.INode) error {
	var configs []*Config
	if err := profile.Get(c.Name(), &configs); err != nil {
		return err
	}

	// 初始化所有 Redis 数据库连接
	for i, config := range configs {
		if err := Add(i, config); err != nil {
			return err
		}
	}

	// 为所有数据库加载脚本
	var loadErr error
	Range(func(client *Client) {
		if loadErr != nil {
			return
		}
		if err := loadAllScripts(client); err != nil {
			loadErr = err
		}
	})

	return loadErr
}

// Stop 停止组件，关闭所有数据库连接
func (c *Component) Stop(ctx context.Context) error {
	Close()
	return nil
}
