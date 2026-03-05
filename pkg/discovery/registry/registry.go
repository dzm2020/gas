// Package registry 提供 discovery 提供者的注册表，供 discovery 包与各 provider 引用，避免循环依赖。
package registry

import (
	"github.com/dzm2020/gas/pkg/discovery/iface"
	"github.com/dzm2020/gas/pkg/lib/factory"
)

var (
	factoryMgr = factory.New[iface.IDiscovery]()
)

// GetFactoryMgr 返回 discovery 提供者工厂管理器，供 provider 在 init 中注册。
func GetFactoryMgr() *factory.Manager[iface.IDiscovery] {
	return factoryMgr
}
