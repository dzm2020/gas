package actor

import (
	"errors"
	"fmt"
	"gas/internal/iface"
	"reflect"
	"sync"
)

var ErrMessageHandlerNotFound = errors.New("actor message handler not found")

type Router struct {
	mu     sync.RWMutex
	routes map[uint16]routerEntry
}

type routerEntry struct {
	msgType      reflect.Type
	handler      reflect.Value
	returnsError bool
	isSync       bool         // true: 同步调用 (ctx, request, response) error, false: 异步调用 (ctx, request) error
	responseType reflect.Type // 同步调用时的 response 类型
}

func NewRouter() iface.IRouter {
	return &Router{
		routes: make(map[uint16]routerEntry),
	}
}

var (
	typeOfActorContext = reflect.TypeOf((*iface.IContext)(nil)).Elem()
	typeOfError        = reflect.TypeOf((*error)(nil)).Elem()
)

func (r *Router) Register(msgId uint16, handler interface{}) error {
	if handler == nil {
		return fmt.Errorf("actor: handler is nil")
	}

	handlerValue := reflect.ValueOf(handler)
	handlerType := handlerValue.Type()

	if handlerType.Kind() != reflect.Func {
		return fmt.Errorf("actor: handler must be a function, got %s", handlerType.Kind())
	}

	numIn := handlerType.NumIn()
	if numIn != 2 && numIn != 3 {
		return fmt.Errorf("actor: handler must accept 2 parameters (ctx, request) for async or 3 parameters (ctx, request, response) for sync, got %d", numIn)
	}

	if handlerType.In(0) != typeOfActorContext {
		return fmt.Errorf("actor: handler first parameter must be actor.IContext")
	}

	requestType := handlerType.In(1)
	if requestType.Kind() != reflect.Pointer {
		return fmt.Errorf("actor: handler second parameter (request) must be pointer, got %s", requestType)
	}

	isSync := numIn == 3
	var responseType reflect.Type
	if isSync {
		responseType = handlerType.In(2)
		if responseType.Kind() != reflect.Pointer {
			return fmt.Errorf("actor: handler third parameter (response) must be pointer, got %s", responseType)
		}
	}

	numOut := handlerType.NumOut()
	if numOut > 1 {
		return fmt.Errorf("actor: handler can return at most one value")
	}

	returnsError := numOut == 1 && handlerType.Out(0).Implements(typeOfError)
	if numOut == 1 && !returnsError {
		return fmt.Errorf("actor: handler return type must be error")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.routes[msgId]; exists {
		return fmt.Errorf("actor: handler already registered for msgId=%d", msgId)
	}

	r.routes[msgId] = routerEntry{
		msgType:      requestType,
		handler:      handlerValue,
		returnsError: returnsError,
		isSync:       isSync,
		responseType: responseType,
	}
	return nil
}

func (r *Router) Handle(ctx iface.IContext, msgId uint16, data []byte) ([]byte, error) {
	r.mu.RLock()
	entry, ok := r.routes[msgId]
	r.mu.RUnlock()

	if !ok {
		return nil, ErrMessageHandlerNotFound
	}

	requestValue := reflect.New(entry.msgType.Elem())
	if len(data) > 0 {
		if err := ctx.GetSerializer().Unmarshal(data, requestValue.Interface()); err != nil {
			return nil, fmt.Errorf("actor: unmarshal message msgId:%d failed: %w", msgId, err)
		}
	}

	args := []reflect.Value{reflect.ValueOf(ctx), requestValue}

	if entry.isSync {
		responseValue := reflect.New(entry.responseType.Elem())
		args = append(args, responseValue)

		results := entry.handler.Call(args)
		if entry.returnsError && len(results) > 0 && !results[0].IsNil() {
			if err, ok := results[0].Interface().(error); ok {
				return nil, err
			}
			return nil, fmt.Errorf("actor: handler returned non-error value")
		}

		responseData, err := ctx.GetSerializer().Marshal(responseValue.Interface())
		if err != nil {
			return nil, fmt.Errorf("actor: marshal response message msgId:%d failed: %w", msgId, err)
		}
		return responseData, nil
	}

	results := entry.handler.Call(args)
	if entry.returnsError && len(results) > 0 && !results[0].IsNil() {
		if err, ok := results[0].Interface().(error); ok {
			return nil, err
		}
		return nil, fmt.Errorf("actor: handler returned non-error value")
	}
	return nil, nil
}

// HasRoute 判断指定消息ID的路由是否存在
func (r *Router) HasRoute(msgId uint16) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, exists := r.routes[msgId]
	return exists
}
