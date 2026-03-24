// Package event 提供泛型 Listener[V]：在调用方 goroutine 内同步通知多个 handler。
// 与 Actor 的 topic 事件总线（internal/actor、IContext.Subscribe、PublishLocal/PublishCluster）语义不同，见 docs/event.md。
//
// Register/UnRegister 使用函数指针比较；闭包每次为不同指针，重复注册时不会与已有项合并。
package event

import (
	"reflect"
	"sync"

	"golang.org/x/exp/slices"
)

func handlerComparable[T any](this, other T) bool {
	return reflect.ValueOf(this).Pointer() == reflect.ValueOf(other).Pointer()
}

type Listener[V any] struct {
	mu       sync.RWMutex
	handlers []func(V)
}

func NewListener[V any]() *Listener[V] {
	return &Listener[V]{}
}

func (m *Listener[V]) Register(handler func(V)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	f := func(other func(V)) bool {
		return handlerComparable(handler, other)
	}
	if slices.ContainsFunc(m.handlers, f) {

		return
	}
	m.handlers = append(m.handlers, handler)
}

func (m *Listener[V]) UnRegister(handler func(V)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	index := slices.IndexFunc(m.handlers, func(other func(V)) bool {
		return handlerComparable(handler, other)
	})
	if index < 0 {
		return
	}
	m.handlers = slices.Delete(m.handlers, index, index+1)
}

func (m *Listener[V]) Notify(param V) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	handlers := m.handlers
	for _, handler := range handlers {
		handler(param)
	}
}
