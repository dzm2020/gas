// 本文件将 Gate 封装为可挂载到 INode 的生命周期组件，并从 profile 读取配置。
package gate

import (
	"context"

	"github.com/dzm2020/gas/internal/iface"
	"github.com/dzm2020/gas/internal/profile"
	"github.com/dzm2020/gas/pkg/lib/component"
)

const (
	ComponentName = "gate" // 组件在 profile 中的配置键名
)

// Component 网关组件：从 profile 读取配置，启动/停止 Gate。
type Component struct {
	component.BaseComponent[iface.INode]
	*Gate
}

// NewComponent 创建新的网关组件（使用默认配置，实际地址等由 profile 覆盖）。
func NewComponent() *Component {
	c := &Component{
		Gate: &Gate{},
	}
	return c
}

// Name 返回组件名 "gate"，用于 profile 配置键。
func (r *Component) Name() string {
	return ComponentName
}

// Start 从 profile 读取 gate 配置，填充 Gate 的 Address/Options/MaxConn，并调用 Gate.Start；同时会由 Gate 注册 Session 工厂。
func (r *Component) Start(ctx context.Context, node iface.INode) error {
	conf := DefaultConfig()
	if err := profile.Get(r.Name(), conf); err != nil {
		return err
	}

	r.Gate.Options = ToOptions(conf)
	r.Gate.Address = conf.Address
	r.Gate.MaxConn = int64(conf.MaxConn)
	return r.Gate.Start(ctx, node.System())
}

// Stop 委托 Gate.Stop 关闭网络服务。
func (r *Component) Stop(ctx context.Context) error {
	return r.Gate.Stop(ctx)
}
