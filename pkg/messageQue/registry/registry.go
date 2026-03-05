// Package registry 提供 messageQue 提供者的注册表，供 messageQue 包与各 provider 引用，避免循环依赖。
package registry

import (
	"github.com/dzm2020/gas/pkg/lib/factory"
	"github.com/dzm2020/gas/pkg/messageQue/iface"
)

var (
	factoryMgr = factory.New[iface.IMessageQue]()
)

// GetFactoryMgr 返回 messageQue 提供者工厂管理器，供 provider 在 init 中注册。
func GetFactoryMgr() *factory.Manager[iface.IMessageQue] {
	return factoryMgr
}
