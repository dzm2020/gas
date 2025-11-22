package app

import (
	"gas/internal/actor"
	"gas/internal/iface"
)

// Config app 组件配置
type Config struct {
	// Actors actor 配置列表
	Actors []ActorConfig `json:"actors"`
}

// ActorConfig actor 配置
type ActorConfig struct {
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

// RouterConfig 路由配置
type RouterConfig struct {
	// Routes 路由列表，key 为消息ID，value 为处理函数
	Routes map[uint16]interface{} `json:"-"`
}

// NewRouterConfig 创建路由配置
func NewRouterConfig() *RouterConfig {
	return &RouterConfig{
		Routes: make(map[uint16]interface{}),
	}
}

// RegisterRoute 注册路由
func (r *RouterConfig) RegisterRoute(msgId uint16, handler interface{}) {
	if r.Routes == nil {
		r.Routes = make(map[uint16]interface{})
	}
	r.Routes[msgId] = handler
}

// BuildRouter 构建路由实例
func (r *RouterConfig) BuildRouter() (iface.IRouter, error) {
	if r == nil || len(r.Routes) == 0 {
		return nil, nil
	}

	router := actor.NewRouter()
	for msgId, handler := range r.Routes {
		if err := router.Register(msgId, handler); err != nil {
			return nil, err
		}
	}
	return router, nil
}
