package actor

import (
	"gas/internal/iface"
	"reflect"
	"sync"
)

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
	typeOfSession      = reflect.TypeOf((iface.ISession)(nil))
	typeOfByteArray    = reflect.TypeOf(([]byte)(nil))
)

func (r *Router) Register(msgId int64, handler interface{}) error {
	if handler == nil {
		return ErrHandlerIsNil()
	}

	handlerValue := reflect.ValueOf(handler)
	handlerFuncType := handlerValue.Type()

	if handlerFuncType.Kind() != reflect.Func {
		return ErrHandlerMustBeFunction(handlerFuncType.Kind().String())
	}

	numIn := handlerFuncType.NumIn()
	if numIn < 2 || numIn > 3 {
		return ErrHandlerParameterCount(numIn)
	}

	// 第一个参数必须是 IContext
	if handlerFuncType.In(0) != typeOfActorContext {
		return ErrHandlerFirstParameterMustBeContext()
	}

	entry, err := r.parseHandler(handlerValue, handlerFuncType, numIn)
	if err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.routes[msgId]; exists {
		return ErrHandlerAlreadyRegistered(msgId)
	}

	r.routes[msgId] = entry
	return nil
}

// parseHandler 解析处理器函数签名
func (r *Router) parseHandler(handlerValue reflect.Value, handlerFuncType reflect.Type, numIn int) (routerEntry, error) {
	var entry routerEntry
	entry.handler = handlerValue

	switch numIn {
	case 3:
		return r.parseThreeParamHandler(handlerFuncType, entry)
	case 2:
		return r.parseTwoParamHandler(handlerFuncType, entry)
	default:
		return entry, ErrUnsupportedParameterCount(numIn)
	}
}

// parseThreeParamHandler 解析三参数处理器（客户端消息或同步消息）
func (r *Router) parseThreeParamHandler(handlerFuncType reflect.Type, entry routerEntry) (routerEntry, error) {
	secondParamType := handlerFuncType.In(1)

	// 如果第二个参数是 Session，则是客户端消息: (ctx, session, request)
	if secondParamType == typeOfSession {
		entry.handlerType = handlerTypeClient
		requestType := handlerFuncType.In(2)
		if requestType.Kind() != reflect.Pointer && requestType != typeOfByteArray {
			return entry, ErrClientHandlerThirdParameter(requestType.String())
		}
		entry.requestType = requestType
		return entry, nil
	}

	// 内部同步消息: (ctx, request, response)
	entry.handlerType = handlerTypeSync
	requestType := handlerFuncType.In(1)
	if requestType.Kind() != reflect.Pointer {
		return entry, ErrSyncHandlerSecondParameter(requestType.String())
	}
	entry.requestType = requestType

	responseType := handlerFuncType.In(2)
	if responseType.Kind() != reflect.Pointer {
		return entry, ErrSyncHandlerThirdParameter(responseType.String())
	}
	entry.responseType = responseType
	return entry, nil
}

// parseTwoParamHandler 解析两参数处理器（异步消息）
func (r *Router) parseTwoParamHandler(handlerFuncType reflect.Type, entry routerEntry) (routerEntry, error) {
	entry.handlerType = handlerTypeAsync

	requestType := handlerFuncType.In(1)
	if requestType.Kind() != reflect.Pointer {
		return entry, ErrAsyncHandlerSecondParameter(requestType.String())
	}
	entry.requestType = requestType

	// 异步消息必须返回 error
	numOut := handlerFuncType.NumOut()
	if numOut != 1 {
		return entry, ErrAsyncHandlerReturnCount(numOut)
	}
	if !handlerFuncType.Out(0).Implements(typeOfError) {
		return entry, ErrAsyncHandlerReturnType()
	}
	entry.returnsError = true
	return entry, nil
}

func (r *Router) Handle(ctx iface.IContext, msgId int64, session iface.ISession, data []byte) ([]byte, error) {
	r.mu.RLock()
	entry, ok := r.routes[msgId]
	r.mu.RUnlock()

	if !ok {
		return nil, ErrMessageHandlerNotFound
	}

	args := []reflect.Value{reflect.ValueOf(ctx)}

	switch entry.handlerType {
	case handlerTypeClient:
		return r.handleClientMessage(ctx, msgId, session, data, entry, args)
	case handlerTypeSync:
		return r.handleSyncMessage(ctx, msgId, data, entry, args)
	case handlerTypeAsync:
		return r.handleAsyncMessage(ctx, msgId, data, entry, args)
	default:
		return nil, ErrUnknownHandlerType(msgId)
	}
}

// handleClientMessage 处理客户端消息
func (r *Router) handleClientMessage(ctx iface.IContext, msgId int64, session iface.ISession, data []byte, entry routerEntry, args []reflect.Value) ([]byte, error) {
	if session == nil {
		return nil, ErrSessionIsNil(msgId)
	}
	args = append(args, reflect.ValueOf(session))

	requestValue, err := r.createRequestValue(entry.requestType, data, ctx)
	if err != nil {
		return nil, ErrUnmarshalRequest(msgId, err)
	}
	args = append(args, requestValue)

	return r.callHandler(entry.handler, args)
}

// handleSyncMessage 处理同步消息
func (r *Router) handleSyncMessage(ctx iface.IContext, msgId int64, data []byte, entry routerEntry, args []reflect.Value) ([]byte, error) {
	requestValue, err := r.createRequestValue(entry.requestType, data, ctx)
	if err != nil {
		return nil, ErrUnmarshalRequest(msgId, err)
	}
	args = append(args, requestValue)

	responseValue := reflect.New(entry.responseType.Elem())
	args = append(args, responseValue)

	if err := r.callHandlerError(entry.handler, args); err != nil {
		return nil, err
	}

	responseData, err := iface.Marshal(ctx.GetSerializer(), responseValue.Interface())
	if err != nil {
		return nil, ErrMarshalResponse(msgId, err)
	}
	return responseData, nil
}

// handleAsyncMessage 处理异步消息
func (r *Router) handleAsyncMessage(ctx iface.IContext, msgId int64, data []byte, entry routerEntry, args []reflect.Value) ([]byte, error) {
	requestValue, err := r.createRequestValue(entry.requestType, data, ctx)
	if err != nil {
		return nil, ErrUnmarshalRequest(msgId, err)
	}
	args = append(args, requestValue)

	return r.callHandler(entry.handler, args)
}

// callHandler 调用处理器并检查错误
func (r *Router) callHandler(handler reflect.Value, args []reflect.Value) ([]byte, error) {
	results := handler.Call(args)
	if len(results) > 0 && !results[0].IsNil() {
		if err, ok := results[0].Interface().(error); ok {
			return nil, err
		}
	}
	return nil, nil
}

// callHandlerError 调用处理器并返回错误（用于同步消息）
func (r *Router) callHandlerError(handler reflect.Value, args []reflect.Value) error {
	results := handler.Call(args)
	if len(results) > 0 && !results[0].IsNil() {
		if err, ok := results[0].Interface().(error); ok {
			return err
		}
	}
	return nil
}

// createRequestValue 创建请求值，支持 []byte 直接赋值或其他类型反序列化
func (r *Router) createRequestValue(requestType reflect.Type, data []byte, ctx iface.IContext) (reflect.Value, error) {
	// 如果消息类型是 []byte，则不需要反序列化，直接使用原始数据
	if requestType == typeOfByteArray {
		return reflect.ValueOf(data), nil
	}
	// 其他类型需要反序列化
	requestValue := reflect.New(requestType.Elem())
	if err := iface.Unmarshal(ctx.GetSerializer(), data, requestValue.Interface()); err != nil {
		return reflect.Value{}, ErrUnmarshalFailed(err)
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
