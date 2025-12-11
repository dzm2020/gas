package actor

import (
	"gas/internal/iface"
	"gas/pkg/lib/glog"
	"reflect"
	"sync"

	"go.uber.org/zap"
)

type Router struct {
	mu     sync.RWMutex
	routes map[int64]routerEntry
}

type handlerType int

const (
	handlerTypeClient handlerType = iota // 客户端消息: (ctx, session, request) error
	handlerTypeSync                      // 同步消息: (ctx, request, response) error
	handlerTypeAsync                     // 异步消息: (ctx, request) error
)

type routerEntry struct {
	handlerType  handlerType
	handler      reflect.Value
	requestType  reflect.Type // 请求类型（可能是[]byte或指针类型）
	responseType reflect.Type // 响应类型（仅同步消息使用，指针类型）
	isByteRequest bool        // request是否为[]byte类型
}

func NewRouter() iface.IRouter {
	return &Router{
		routes: make(map[int64]routerEntry),
	}
}

var (
	typeOfActorContext = reflect.TypeOf((*iface.IContext)(nil)).Elem()
	typeOfError        = reflect.TypeOf((*error)(nil)).Elem()
	typeOfSession      = reflect.TypeOf((*iface.ISession)(nil)).Elem()
	typeOfByteArray    = reflect.TypeOf(([]byte)(nil))
)

func (r *Router) Register(msgId int64, handler interface{}) {
	if handler == nil {
		glog.Error("路由注册失败: 处理器为空", zap.Int64("msgId", msgId))
		return
	}

	handlerValue := reflect.ValueOf(handler)
	handlerFuncType := handlerValue.Type()

	if handlerFuncType.Kind() != reflect.Func {
		glog.Error("路由注册失败: 处理器必须是函数类型",
			zap.Int64("msgId", msgId),
			zap.String("实际类型", handlerFuncType.Kind().String()))
		return
	}

	numIn := handlerFuncType.NumIn()
	if numIn < 2 || numIn > 3 {
		glog.Error("路由注册失败: 处理器必须接受 2 或 3 个参数",
			zap.Int64("msgId", msgId),
			zap.Int("实际参数数量", numIn))
		return
	}

	// 第一个参数必须是 IContext
	if handlerFuncType.In(0) != typeOfActorContext {
		glog.Error("路由注册失败: 处理器的第一个参数必须是 actor.IContext",
			zap.Int64("msgId", msgId))
		return
	}

	entry, err := r.parseHandler(handlerValue, handlerFuncType, numIn)
	if err != nil {
		glog.Error("路由注册失败: 解析处理器签名失败",
			zap.Int64("msgId", msgId),
			zap.Error(err))
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.routes[msgId]; exists {
		glog.Error("路由注册失败: 该消息ID的处理器已注册",
			zap.Int64("msgId", msgId))
		return
	}

	r.routes[msgId] = entry
}

// parseHandler 解析处理器函数签名
func (r *Router) parseHandler(handlerValue reflect.Value, handlerFuncType reflect.Type, numIn int) (routerEntry, error) {
	var entry routerEntry
	entry.handler = handlerValue

	switch numIn {
	case 2:
		return r.parseTwoParamHandler(handlerFuncType, entry)
	case 3:
		return r.parseThreeParamHandler(handlerFuncType, entry)
	default:
		return entry, ErrUnsupportedParameterCount(numIn)
	}
}

// parseTwoParamHandler 解析两参数处理器（异步消息）
// 签名: (ctx iface.IContext, request) error
func (r *Router) parseTwoParamHandler(handlerFuncType reflect.Type, entry routerEntry) (routerEntry, error) {
	entry.handlerType = handlerTypeAsync

	requestType := handlerFuncType.In(1)
	// 检查request类型：必须是[]byte或指针类型
	if requestType == typeOfByteArray {
		entry.isByteRequest = true
		entry.requestType = requestType
	} else if requestType.Kind() == reflect.Pointer {
		entry.isByteRequest = false
		entry.requestType = requestType
	} else {
		return entry, ErrAsyncHandlerSecondParameter(requestType.String())
	}

	// 检查返回值：必须返回error
	numOut := handlerFuncType.NumOut()
	if numOut != 1 {
		return entry, ErrHandlerReturnCount(numOut)
	}
	if !handlerFuncType.Out(0).Implements(typeOfError) {
		return entry, ErrHandlerReturnType()
	}

	return entry, nil
}

// parseThreeParamHandler 解析三参数处理器（客户端消息或同步消息）
func (r *Router) parseThreeParamHandler(handlerFuncType reflect.Type, entry routerEntry) (routerEntry, error) {
	secondParamType := handlerFuncType.In(1)

	// 如果第二个参数是 Session，则是客户端消息: (ctx, session, request) error
	if secondParamType == typeOfSession {
		entry.handlerType = handlerTypeClient
		requestType := handlerFuncType.In(2)
		// 检查request类型：必须是[]byte或指针类型
		if requestType == typeOfByteArray {
			entry.isByteRequest = true
			entry.requestType = requestType
		} else if requestType.Kind() == reflect.Pointer {
			entry.isByteRequest = false
			entry.requestType = requestType
		} else {
			return entry, ErrClientHandlerThirdParameter(requestType.String())
		}
	} else {
		// 同步消息: (ctx, request, response) error
		entry.handlerType = handlerTypeSync
		requestType := handlerFuncType.In(1)
		// 检查request类型：必须是[]byte或指针类型
		if requestType == typeOfByteArray {
			entry.isByteRequest = true
			entry.requestType = requestType
		} else if requestType.Kind() == reflect.Pointer {
			entry.isByteRequest = false
			entry.requestType = requestType
		} else {
			return entry, ErrSyncHandlerSecondParameter(requestType.String())
		}

		responseType := handlerFuncType.In(2)
		// response必须是指针类型
		if responseType.Kind() != reflect.Pointer {
			return entry, ErrSyncHandlerThirdParameter(responseType.String())
		}
		entry.responseType = responseType
	}

	// 检查返回值：必须返回error
	numOut := handlerFuncType.NumOut()
	if numOut != 1 {
		return entry, ErrHandlerReturnCount(numOut)
	}
	if !handlerFuncType.Out(0).Implements(typeOfError) {
		return entry, ErrHandlerReturnType()
	}

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
// 签名: (ctx iface.IContext, session iface.ISession, request) error
func (r *Router) handleClientMessage(ctx iface.IContext, msgId int64, session iface.ISession, data []byte, entry routerEntry, args []reflect.Value) ([]byte, error) {
	if session == nil {
		return nil, ErrSessionIsNil(msgId)
	}
	args = append(args, reflect.ValueOf(session))

	requestValue, err := r.createRequestValue(entry.requestType, entry.isByteRequest, data, ctx)
	if err != nil {
		return nil, ErrUnmarshalRequest(msgId, err)
	}
	args = append(args, requestValue)

	return r.callHandler(entry.handler, args)
}

// handleSyncMessage 处理同步消息
// 签名: (ctx iface.IContext, request, response) error
func (r *Router) handleSyncMessage(ctx iface.IContext, msgId int64, data []byte, entry routerEntry, args []reflect.Value) ([]byte, error) {
	requestValue, err := r.createRequestValue(entry.requestType, entry.isByteRequest, data, ctx)
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
// 签名: (ctx iface.IContext, request) error
func (r *Router) handleAsyncMessage(ctx iface.IContext, msgId int64, data []byte, entry routerEntry, args []reflect.Value) ([]byte, error) {
	requestValue, err := r.createRequestValue(entry.requestType, entry.isByteRequest, data, ctx)
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
func (r *Router) createRequestValue(requestType reflect.Type, isByteRequest bool, data []byte, ctx iface.IContext) (reflect.Value, error) {
	// 如果消息类型是 []byte，则不需要反序列化，直接使用原始数据
	if isByteRequest {
		return reflect.ValueOf(data), nil
	}
	// 其他类型（指针类型）需要反序列化
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
