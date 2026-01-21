package actor

import (
	"github.com/dzm2020/gas/internal/iface"
	"github.com/dzm2020/gas/pkg/glog"
	"reflect"
	"sync"

	"go.uber.org/zap"
)

var (
	globalRouterManager = &routerManager{
		routers: make(map[reflect.Type]iface.IRouter),
	}
)

// routerManager 全局 router 管理器，按 actor 类型缓存 router
type routerManager struct {
	mu      sync.RWMutex
	routers map[reflect.Type]iface.IRouter // 按 actor 类型缓存 router
}

// GetRouterForActor 获取指定 actor 类型的 router，如果不存在则创建并注册
func GetRouterForActor(actor iface.IActor) iface.IRouter {
	if actor == nil {
		return NewRouter()
	}

	actorType := reflect.TypeOf(actor)
	if actorType.Kind() == reflect.Ptr {
		actorType = actorType.Elem()
	}

	globalRouterManager.mu.RLock()
	router, exists := globalRouterManager.routers[actorType]
	globalRouterManager.mu.RUnlock()

	if exists {
		return router
	}

	return RegisterRouter(actor)
}

func RegisterRouter(actor iface.IActor) iface.IRouter {
	actorType := reflect.TypeOf(actor)
	if actorType.Kind() == reflect.Ptr {
		actorType = actorType.Elem()
	}

	// 创建新的 router 并注册
	globalRouterManager.mu.Lock()
	defer globalRouterManager.mu.Unlock()

	if router, exists := globalRouterManager.routers[actorType]; exists {
		return router
	}

	router := NewRouter()
	router.AutoRegister(actor)
	globalRouterManager.routers[actorType] = router
	glog.Debug("创建并缓存 router", zap.String("actorType", actorType.String()))
	return router
}
