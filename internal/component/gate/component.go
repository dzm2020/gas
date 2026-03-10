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

// NewComponent
//
//	@Description: 创建网关组件，配置由 profile 覆盖。
//	@return *Component
func NewComponent() *Component {
	c := &Component{
		Gate: &Gate{},
	}
	return c
}

// Name
//
//	@Description: 返回组件名 "gate"。
//	@receiver r
//	@return string
func (r *Component) Name() string {
	return ComponentName
}

// Start
//
//	@Description: 从 profile 读配置并启动 Gate。
//	@receiver r
//	@param ctx
//	@param node
//	@return error
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

// Stop
//
//	@Description: 关闭 Gate 网络服务。
//	@receiver r
//	@param ctx
//	@return error
func (r *Component) Stop(ctx context.Context) error {
	return r.Gate.Stop(ctx)
}
