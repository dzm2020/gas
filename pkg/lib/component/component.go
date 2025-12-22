package component

import (
	"context"
)

type IComponent[T any] interface {
	Init(t T) error
	Start(ctx context.Context, t T) error
	Stop(ctx context.Context) error
	Name() string
}

type BaseComponent[T any] struct {
}

func (*BaseComponent[T]) Init(t T) error { return nil }
func (*BaseComponent[T]) Start(ctx context.Context, t T) error {
	return nil
}
func (*BaseComponent[T]) Stop(ctx context.Context) error {
	return nil
}
