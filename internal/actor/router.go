package actor

import (
	"errors"
	"fmt"
	"gas/internal/iface"
	"reflect"
	"sync"
)

type IRouter interface {
	Register(msgId uint16, handler interface{}) error
	Handle(ctx iface.IContext, msgId uint16, data []byte) ([]byte, error) // 返回 response 数据和错误，异步调用时 response 为 nil
	HasRoute(msgId uint16) bool                                           // 判断指定消息ID的路由是否存在
}

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

func NewRouter() IRouter {
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
		return fmt.Errorf("gate: handler is nil")
	}

	handlerValue := reflect.ValueOf(handler)
	handlerType := handlerValue.Type()

	if handlerType.Kind() != reflect.Func {
		return fmt.Errorf("gate: handler must be a function, got %s", handlerType.Kind())
	}

	numIn := handlerType.NumIn()
	if numIn != 2 && numIn != 3 {
		return fmt.Errorf("gate: handler must accept 2 parameters (ctx, request) for async or 3 parameters (ctx, request, response) for sync, got %d", numIn)
	}

	// 第一个参数必须是 IContext
	if handlerType.In(0) != typeOfActorContext {
		return fmt.Errorf("gate: handler first parameter must be actor.IContext")
	}

	// 第二个参数是 request message
	requestType := handlerType.In(1)
	if requestType.Kind() != reflect.Pointer {
		return fmt.Errorf("gate: handler second parameter (request) must be pointer, got %s", requestType)
	}

	isSync := numIn == 3
	var responseType reflect.Type

	if isSync {
		// 同步调用：第三个参数是 response message
		responseType = handlerType.In(2)
		if responseType.Kind() != reflect.Pointer {
			return fmt.Errorf("gate: handler third parameter (response) must be pointer, got %s", responseType)
		}
	}

	numOut := handlerType.NumOut()
	if numOut > 1 {
		return fmt.Errorf("gate: handler can return at most one value")
	}

	returnsError := false
	if numOut == 1 {
		if !handlerType.Out(0).Implements(typeOfError) {
			return fmt.Errorf("gate: handler return type must be error")
		}
		returnsError = true
	}

	key := msgId

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.routes[key]; exists {
		return fmt.Errorf("actor: handler already registered for msgId=%d", msgId)
	}

	r.routes[key] = routerEntry{
		msgType:      requestType,
		handler:      handlerValue,
		returnsError: returnsError,
		isSync:       isSync,
		responseType: responseType,
	}
	return nil
}

func (r *Router) Handle(ctx iface.IContext, msgId uint16, data []byte) ([]byte, error) {
	key := msgId

	r.mu.RLock()
	entry, ok := r.routes[key]
	r.mu.RUnlock()

	if !ok {
		return nil, ErrMessageHandlerNotFound
	}

	// 解析 request message
	requestValue := reflect.New(entry.msgType.Elem())
	requestMsg := requestValue.Interface()

	ser := ctx.GetSerializer()
	if len(data) > 0 {
		if err := ser.Unmarshal(data, requestMsg); err != nil {
			return nil, fmt.Errorf("actor: unmarshal message msgId:%d failed: %w", key, err)
		}
	}

	// 构建调用参数
	args := []reflect.Value{
		reflect.ValueOf(ctx),
		requestValue,
	}

	// 如果是同步调用，需要传递 response 参数并返回序列化后的数据
	if entry.isSync {
		responseValue := reflect.New(entry.responseType.Elem())
		args = append(args, responseValue)

		// 调用 handler
		results := entry.handler.Call(args)

		// 处理返回值
		var handlerErr error
		if entry.returnsError && len(results) > 0 && !results[0].IsNil() {
			if err, ok := results[0].Interface().(error); ok {
				handlerErr = err
			} else {
				return nil, fmt.Errorf("actor: handler returned non-error value")
			}
		}

		// 如果有错误，返回错误
		if handlerErr != nil {
			return nil, handlerErr
		}

		// 序列化 response message
		responseMsg := responseValue.Interface()
		ser := ctx.GetSerializer()
		responseData, err := ser.Marshal(responseMsg)
		if err != nil {
			return nil, fmt.Errorf("actor: marshal response message msgId:%d failed: %w", key, err)
		}

		return responseData, nil
	}

	// 异步调用：只需要 request 参数，返回 nil, error
	results := entry.handler.Call(args)
	if entry.returnsError && len(results) == 1 && !results[0].IsNil() {
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
