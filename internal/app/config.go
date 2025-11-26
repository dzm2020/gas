package app

import (
	"gas/internal/iface"
)

// Config app 组件配置
type Config struct {
	// Actors actor 配置列表
	Service []ServiceConfig `json:"services"`
	Name    string          `json:"name"`
}

// ServiceConfig actor 配置
type ServiceConfig struct {
	// Name actor 名称
	Name string `json:"name"`
	// Actor actor 实例（需要实现 iface.IActor 接口）
	Actor iface.IActor `json:"-"`
	// Params 启动参数
	Params []interface{} `json:"params"`
	// Router 路由配置
	Router iface.IRouter `json:"router"`
	// Middlewares 中间件列表
	Middlewares []iface.TaskMiddleware `json:"-"`
}
