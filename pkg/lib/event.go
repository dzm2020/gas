package lib

import (
	"reflect"

	"golang.org/x/exp/slices"
)

func handlerComparable[T any](this, other T) bool {
	return reflect.ValueOf(this).Pointer() == reflect.ValueOf(other).Pointer()
}

type Listener[V any] struct {
	handlers []func(V)
}

func NewListener[V any]() *Listener[V] {
	return &Listener[V]{}
}

func (m *Listener[V]) Register(handler func(V)) {
	f := func(other func(V)) bool {
		return handlerComparable(handler, other)
	}
	if slices.ContainsFunc(m.handlers, f) {

		return
	}
	m.handlers = append(m.handlers, handler)
}

func (m *Listener[V]) UnRegister(handler func(V)) {
	index := slices.IndexFunc(m.handlers, func(other func(V)) bool {
		return handlerComparable(handler, other)
	})
	if index < 0 {
		return
	}
	m.handlers = slices.Delete(m.handlers, index, index+1)
}

func (m *Listener[V]) Notify(param V) {
	handlers := m.handlers
	for _, handler := range handlers {
		handler(param)
	}
}
