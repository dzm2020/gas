package factory

import (
	"errors"
	"sync"
)

func New[T any]() *Manager[T] {
	return &Manager[T]{
		factories: make(map[string]func(args ...any) (T, error)),
	}
}

type Manager[T any] struct {
	mu        sync.RWMutex
	factories map[string]func(args ...any) (T, error) // key:工厂名  value：构造函数
}

// Register 注册一个工厂函数
func (f *Manager[T]) Register(name string, factory func(args ...any) (T, error)) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.factories[name]; ok {
		return errors.New("factory already exists")
	}
	f.factories[name] = factory
	return nil
}

// Unregister 注销一个工厂函数
func (f *Manager[T]) Unregister(name string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.factories, name)
}

// Get 获取一个工厂函数
func (f *Manager[T]) Get(name string) (func(args ...any) (T, error), bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	factory, ok := f.factories[name]
	return factory, ok
}

// List 列出所有已注册的工厂名称
func (f *Manager[T]) List() []string {
	f.mu.RLock()
	defer f.mu.RUnlock()
	names := make([]string, 0, len(f.factories))
	for name := range f.factories {
		names = append(names, name)
	}
	return names
}
