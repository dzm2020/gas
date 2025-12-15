package actor

import (
	"gas/internal/errs"
	"gas/internal/iface"
	"gas/pkg/lib/glog"
	"reflect"
	"sync"
	"unicode"

	"go.uber.org/zap"
)

var actorInterfaceMethods = map[string]bool{
	"OnInit":    true,
	"OnMessage": true,
	"OnStop":    true,
}

type Router struct {
	mu           sync.RWMutex
	methodRoutes map[string]routerEntry // 基于方法名的路由
}

type handlerType int

const (
	handlerTypeClient handlerType = iota // 客户端消息: (实际接收者, ctx, session, request) error
	handlerTypeSync                      // 同步消息: (实际接收者, ctx, request, response) error
	handlerTypeAsync                     // 异步消息: (实际接收者, ctx, request) error
)

type routerEntry struct {
	handlerType   handlerType
	handler       reflect.Method
	requestType   reflect.Type // 请求类型（可能是[]byte或指针类型）
	responseType  reflect.Type // 响应类型（仅同步消息使用，指针类型）
	isByteRequest bool         // request是否为[]byte类型
}

func NewRouter() iface.IRouter {
	return &Router{
		methodRoutes: make(map[string]routerEntry),
	}
}

var (
	globalRouterManager = &routerManager{
		routers: make(map[reflect.Type]iface.IRouter),
	}

	typeOfActorContext = reflect.TypeOf((*iface.IContext)(nil)).Elem()
	typeOfError        = reflect.TypeOf((*error)(nil)).Elem()
	typeOfSession      = reflect.TypeOf((*iface.ISession)(nil)).Elem()
	typeOfByteArray    = reflect.TypeOf(([]byte)(nil))
)

// AutoRegister 自动扫描并注册 actor 的所有导出方法
func (r *Router) AutoRegister(actor iface.IActor) {
	if actor == nil {
		glog.Error("自动注册失败: actorContext 为空")
		return
	}
	r.scanAndRegisterActor(actor)
}

// scanAndRegisterActor 扫描并注册 actor 的导出方法
func (r *Router) scanAndRegisterActor(actor interface{}) {
	actorType := reflect.TypeOf(actor)

	for i := 0; i < actorType.NumMethod(); i++ {
		method := actorType.Method(i)
		// 只处理导出的方法（首字母大写）
		if !unicode.IsUpper(rune(method.Name[0])) {
			continue
		}

		// 过滤掉 IActor 接口的默认方法
		if actorInterfaceMethods[method.Name] {
			continue
		}

		methodType := method.Type

		// 检查返回值：必须返回 error
		if methodType.NumOut() != 1 || !methodType.Out(0).Implements(typeOfError) {
			continue
		}

		// 直接注册方法值，在 Handle 时直接调用
		// 检查方法签名是否符合要求
		if r.isValidMethodSignature(methodType) {
			// 创建路由条目
			entry, err := r.createMethodEntry(method, methodType)
			if err == nil {
				r.mu.Lock()
				r.methodRoutes[method.Name] = entry
				r.mu.Unlock()
				glog.Debug("自动注册路由", zap.String("method", method.Name))
			}
		}
	}
}

// isValidMethodSignature 检查方法签名是否有效
// 方法签名应该是: func (a *Actor) MethodName(ctx IContext, ...) error
// 至少需要 2 个参数（接收者 + ctx），最多 4 个参数（接收者 + ctx + 2个实际参数）
func (r *Router) isValidMethodSignature(methodType reflect.Type) bool {
	numIn := methodType.NumIn() - 1 // 减去接收者
	if numIn < 2 || numIn > 3 {
		return false
	}
	// 第一个实际参数必须是 IContext
	return methodType.In(1) == typeOfActorContext
}

// parseRequestType 解析请求类型
func parseRequestType(requestType reflect.Type) (reflect.Type, bool, error) {
	if requestType == typeOfByteArray {
		return requestType, true, nil
	}
	if requestType.Kind() == reflect.Pointer {
		return requestType, false, nil
	}
	return nil, false, errs.ErrAsyncHandlerThirdParameter(requestType.String())
}

// createMethodEntry 为 actor 的方法创建路由条目
// 方法签名: func (a *Actor) MethodName(ctx IContext, ...) error
func (r *Router) createMethodEntry(method reflect.Method, methodType reflect.Type) (routerEntry, error) {
	var entry routerEntry
	entry.handler = method

	numIn := methodType.NumIn() - 1 // 减去接收者

	switch numIn {
	case 2:
		// (actor, ctx, request) error - 异步消息
		param1Type := methodType.In(2)
		if param1Type == typeOfSession {
			return entry, errs.ErrUnsupportedParameterCount(numIn)
		}
		entry.handlerType = handlerTypeAsync
		requestType, isByte, err := parseRequestType(param1Type)
		if err != nil {
			return entry, err
		}
		entry.requestType = requestType
		entry.isByteRequest = isByte

	case 3:
		// (actor, ctx, param1, param2) error
		param1Type := methodType.In(2)
		param2Type := methodType.In(3)

		if param1Type == typeOfSession {
			// (actor, ctx, session, request) error - 客户端消息
			entry.handlerType = handlerTypeClient
			requestType, isByte, err := parseRequestType(param2Type)
			if err != nil {
				return entry, errs.ErrClientHandlerFourthParameter(param2Type.String())
			}
			entry.requestType = requestType
			entry.isByteRequest = isByte
		} else {
			// (actor, ctx, request, response) error - 同步消息
			entry.handlerType = handlerTypeSync
			requestType, isByte, err := parseRequestType(param1Type)
			if err != nil {
				return entry, errs.ErrSyncHandlerThirdParameter(param1Type.String())
			}
			entry.requestType = requestType
			entry.isByteRequest = isByte

			if param2Type.Kind() != reflect.Pointer {
				return entry, errs.ErrSyncHandlerFourthParameter(param2Type.String())
			}
			entry.responseType = param2Type
		}

	default:
		return entry, errs.ErrUnsupportedParameterCount(numIn)
	}

	// 检查返回值
	if methodType.NumOut() != 1 || !methodType.Out(0).Implements(typeOfError) {
		return entry, errs.ErrHandlerReturnType()
	}

	return entry, nil
}

// Handle 基于方法名处理消息，从 message.Data 反序列化参数
func (r *Router) Handle(ctx iface.IContext, methodName string, session iface.ISession, data []byte) ([]byte, error) {
	r.mu.RLock()
	entry, ok := r.methodRoutes[methodName]
	r.mu.RUnlock()

	if !ok {
		return nil, errs.ErrMessageHandlerNotFound
	}

	switch entry.handlerType {
	case handlerTypeClient:
		return r.handleClientMessage(ctx, methodName, session, data, entry)
	case handlerTypeSync:
		return r.handleSyncMessage(ctx, methodName, data, entry)
	case handlerTypeAsync:
		return r.handleAsyncMessage(ctx, methodName, data, entry)
	default:
		return nil, errs.ErrUnknownHandlerType(0)
	}
}

// callHandler 调用处理器并检查错误
func (r *Router) callHandler(handler reflect.Method, args []reflect.Value) ([]byte, error) {
	results := handler.Func.Call(args)
	if len(results) > 0 && !results[0].IsNil() {
		if err, ok := results[0].Interface().(error); ok {
			return nil, err
		}
	}
	return nil, nil
}

// HasRoute 判断指定方法名的路由是否存在
func (r *Router) HasRoute(methodName string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, exists := r.methodRoutes[methodName]
	return exists
}

// buildCallArgs 构建调用参数
func (r *Router) buildCallArgs(ctx iface.IContext, entry routerEntry, session iface.ISession, data []byte) ([]reflect.Value, error) {
	actor := ctx.Actor()
	if actor == nil {
		return nil, errs.ErrActorIsNil()
	}

	callArgs := []reflect.Value{reflect.ValueOf(actor), reflect.ValueOf(ctx)}

	// 客户端消息需要 session
	if entry.handlerType == handlerTypeClient {
		if session == nil {
			return nil, errs.ErrSessionIsNil(0)
		}
		callArgs = append(callArgs, reflect.ValueOf(session))
	}

	// 反序列化请求参数
	requestValue, err := r.createRequestValue(entry.requestType, entry.isByteRequest, data, ctx)
	if err != nil {
		return nil, errs.ErrUnmarshalRequest(0, err)
	}
	callArgs = append(callArgs, requestValue)

	// 同步消息需要 response
	if entry.handlerType == handlerTypeSync {
		responseValue := reflect.New(entry.responseType.Elem())
		callArgs = append(callArgs, responseValue)
	}

	return callArgs, nil
}

// handleClientMessage 处理客户端消息
func (r *Router) handleClientMessage(ctx iface.IContext, methodName string, session iface.ISession, data []byte, entry routerEntry) ([]byte, error) {
	callArgs, err := r.buildCallArgs(ctx, entry, session, data)
	if err != nil {
		return nil, err
	}
	return r.callHandler(entry.handler, callArgs)
}

// handleSyncMessage 处理同步消息
func (r *Router) handleSyncMessage(ctx iface.IContext, methodName string, data []byte, entry routerEntry) ([]byte, error) {
	callArgs, err := r.buildCallArgs(ctx, entry, nil, data)
	if err != nil {
		return nil, err
	}

	// 调用处理器
	results := entry.handler.Func.Call(callArgs)
	if len(results) > 0 && !results[0].IsNil() {
		if err, ok := results[0].Interface().(error); ok {
			return nil, err
		}
	}

	// 序列化响应（response 是最后一个参数）
	responseValue := callArgs[len(callArgs)-1]
	responseData := ctx.Node().Marshal(responseValue.Interface())

	return responseData, nil
}

// handleAsyncMessage 处理异步消息
func (r *Router) handleAsyncMessage(ctx iface.IContext, methodName string, data []byte, entry routerEntry) ([]byte, error) {
	callArgs, err := r.buildCallArgs(ctx, entry, nil, data)
	if err != nil {
		return nil, err
	}
	return r.callHandler(entry.handler, callArgs)
}

// createRequestValue 从 Data 创建请求值
func (r *Router) createRequestValue(requestType reflect.Type, isByteRequest bool, data []byte, ctx iface.IContext) (reflect.Value, error) {
	// 如果消息类型是 []byte，则不需要反序列化，直接使用原始数据
	if isByteRequest {
		return reflect.ValueOf(data), nil
	}

	// 其他类型（指针类型）需要反序列化
	requestValue := reflect.New(requestType.Elem())

	ctx.Node().Unmarshal(data, requestValue.Interface())
	return requestValue, nil
}
