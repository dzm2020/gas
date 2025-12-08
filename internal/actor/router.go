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
	routes map[int64]routerEntry
}

type handlerType int

const (
	handlerTypeClient handlerType = iota // 客户端消息: (ctx, session, request)
	handlerTypeSync                      // 内部同步消息: (ctx, request, response)
	handlerTypeAsync                     // 内部异步消息: (ctx, request) error
)

type routerEntry struct {
	handlerType  handlerType
	handler      reflect.Value
	requestType  reflect.Type // 请求类型（客户端消息的第三个参数，内部消息的第二个参数）
	responseType reflect.Type // 响应类型（仅同步消息使用）
	returnsError bool         // 是否返回 error（仅异步消息使用）
}

func NewRouter() iface.IRouter {
	return &Router{
		routes: make(map[int64]routerEntry),
	}
}

var (
	typeOfActorContext = reflect.TypeOf((*iface.IContext)(nil)).Elem()
	typeOfError        = reflect.TypeOf((*error)(nil)).Elem()
	typeOfSession      = reflect.TypeOf((*iface.Session)(nil))
	typeOfByteArray    = reflect.TypeOf(([]byte)(nil))
)

func (r *Router) Register(msgId int64, handler interface{}) error {
	if handler == nil {
		return fmt.Errorf("actor: handler is nil")
	}

	handlerValue := reflect.ValueOf(handler)
	handlerFuncType := handlerValue.Type()

	if handlerFuncType.Kind() != reflect.Func {
		return fmt.Errorf("actor: handler must be a function, got %s", handlerFuncType.Kind())
	}

	numIn := handlerFuncType.NumIn()
	if numIn < 2 || numIn > 3 {
		return fmt.Errorf("actor: handler must accept 2 or 3 parameters, got %d", numIn)
	}

	// 第一个参数必须是 IContext
	if handlerFuncType.In(0) != typeOfActorContext {
		return fmt.Errorf("actor: handler first parameter must be actor.IContext")
	}

	var entry routerEntry
	entry.handler = handlerValue

	// 判断处理函数类型
	if numIn == 3 {
		secondParamType := handlerFuncType.In(1)
		// 如果第二个参数是 Session，则是客户端消息: (ctx, session, request)
		if secondParamType == typeOfSession {
			entry.handlerType = handlerTypeClient
			// 第三个参数是 request
			requestType := handlerFuncType.In(2)
			if requestType.Kind() != reflect.Pointer && (requestType != typeOfByteArray) {
				return fmt.Errorf("actor: client handler third parameter (request) must be pointer, got %s", requestType)
			}
			entry.requestType = requestType
		} else {
			// 内部同步消息: (ctx, request, response)
			entry.handlerType = handlerTypeSync
			// 第二个参数是 request
			requestType := handlerFuncType.In(1)
			if requestType.Kind() != reflect.Pointer {
				return fmt.Errorf("actor: sync handler second parameter (request) must be pointer, got %s", requestType)
			}
			entry.requestType = requestType
			// 第三个参数是 response
			responseType := handlerFuncType.In(2)
			if responseType.Kind() != reflect.Pointer {
				return fmt.Errorf("actor: sync handler third parameter (response) must be pointer, got %s", responseType)
			}
			entry.responseType = responseType
		}
	} else if numIn == 2 {
		// 内部异步消息: (ctx, request) error
		entry.handlerType = handlerTypeAsync
		// 第二个参数是 request
		requestType := handlerFuncType.In(1)
		if requestType.Kind() != reflect.Pointer {
			return fmt.Errorf("actor: async handler second parameter (request) must be pointer, got %s", requestType)
		}
		entry.requestType = requestType

		// 异步消息必须返回 error
		numOut := handlerFuncType.NumOut()
		if numOut != 1 {
			return fmt.Errorf("actor: async handler must return exactly one value (error), got %d", numOut)
		}
		if !handlerFuncType.Out(0).Implements(typeOfError) {
			return fmt.Errorf("actor: async handler return type must be error")
		}
		entry.returnsError = true
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.routes[msgId]; exists {
		return fmt.Errorf("actor: handler already registered for msgId=%d", msgId)
	}

	r.routes[msgId] = entry
	return nil
}

func (r *Router) Handle(ctx iface.IContext, msgId int64, session *iface.Session, data []byte) ([]byte, error) {
	r.mu.RLock()
	entry, ok := r.routes[msgId]
	r.mu.RUnlock()

	if !ok {
		return nil, ErrMessageHandlerNotFound
	}

	var args []reflect.Value
	args = append(args, reflect.ValueOf(ctx))

	switch entry.handlerType {
	case handlerTypeClient:
		// 客户端消息: (ctx, session, request)
		if session == nil {
			return nil, fmt.Errorf("actor: session is nil for msgId:%d", msgId)
		}
		args = append(args, reflect.ValueOf(session))

		// 第三个参数是 request，需要反序列化
		requestValue, err := r.createRequestValue(entry.requestType, data, ctx)
		if err != nil {
			return nil, fmt.Errorf("actor: unmarshal request message msgId:%d failed: %w", msgId, err)
		}
		args = append(args, requestValue)

		// 客户端消息不返回响应数据
		results := entry.handler.Call(args)
		if len(results) > 0 && !results[0].IsNil() {
			if err, ok := results[0].Interface().(error); ok {
				return nil, err
			}
		}
		return nil, nil

	case handlerTypeSync:
		// 内部同步消息: (ctx, request, response)
		requestValue, err := r.createRequestValue(entry.requestType, data, ctx)
		if err != nil {
			return nil, fmt.Errorf("actor: unmarshal request message msgId:%d failed: %w", msgId, err)
		}
		args = append(args, requestValue)

		responseValue := reflect.New(entry.responseType.Elem())
		args = append(args, responseValue)

		results := entry.handler.Call(args)
		if len(results) > 0 && !results[0].IsNil() {
			if err, ok := results[0].Interface().(error); ok {
				return nil, err
			}
		}

		// 序列化响应数据
		responseData, err := ctx.GetSerializer().Marshal(responseValue.Interface())
		if err != nil {
			return nil, fmt.Errorf("actor: marshal response message msgId:%d failed: %w", msgId, err)
		}
		return responseData, nil

	case handlerTypeAsync:
		// 内部异步消息: (ctx, request) error
		requestValue, err := r.createRequestValue(entry.requestType, data, ctx)
		if err != nil {
			return nil, fmt.Errorf("actor: unmarshal request message msgId:%d failed: %w", msgId, err)
		}
		args = append(args, requestValue)

		results := entry.handler.Call(args)
		if entry.returnsError && len(results) > 0 && !results[0].IsNil() {
			if err, ok := results[0].Interface().(error); ok {
				return nil, err
			}
			return nil, fmt.Errorf("actor: handler returned non-error value")
		}
		return nil, nil

	default:
		return nil, fmt.Errorf("actor: unknown handler type for msgId:%d", msgId)
	}
}

// createRequestValue 创建请求值，支持 []byte 直接赋值或其他类型反序列化
func (r *Router) createRequestValue(requestType reflect.Type, data []byte, ctx iface.IContext) (reflect.Value, error) {
	// 如果消息类型是 []byte，则不需要反序列化，直接使用原始数据
	if requestType == typeOfByteArray {
		// 对于 []byte 类型，直接返回指向 data 的指针
		return reflect.ValueOf(&data).Elem(), nil
	}

	// 其他类型需要反序列化
	requestValue := reflect.New(requestType.Elem())
	if len(data) > 0 {
		if err := ctx.GetSerializer().Unmarshal(data, requestValue.Interface()); err != nil {
			return reflect.Value{}, err
		}
	}
	return requestValue, nil
}

// HasRoute 判断指定消息ID的路由是否存在
func (r *Router) HasRoute(msgId int64) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, exists := r.routes[msgId]
	return exists
}
