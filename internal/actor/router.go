package actor

import (
	"errors"
	"github.com/dzm2020/gas/internal/iface"
	"github.com/dzm2020/gas/internal/session"
	"github.com/dzm2020/gas/pkg/glog"
	"github.com/dzm2020/gas/pkg/lib/xerror"
	"reflect"
	"sync"
	"unicode"

	"go.uber.org/zap"
)

// 路由相关错误
var (
	ErrMessageHandlerNotFound     = errors.New("actor: 消息处理器未找到")
	ErrHandlerReturnType          = errors.New("actor: 处理器返回类型必须是 error")
	ErrUnknownHandlerType         = errors.New("actor: 未知的处理器类型")
	ErrAsyncHandlerThirdParameter = errors.New("actor: 异步处理器第三个参数 (request) 必须是指针或 []byte")
	ErrSyncHandlerFourParameter   = errors.New("actor: 同步处理器第四个参数 (response) 必须是指针")
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
	handlerTypeSync        handlerType = iota // 同步消息: (实际接收者, ctx, request, response) error
	handlerTypeAsync                          // 异步消息: (实际接收者, ctx, request) error
	handlerTypeSession                        // 会话消息: (实际接收者, ctx, session *session.Session, request) error
	handlerTypeSessionOnly                    // 会话消息: (实际接收者, ctx, session *session.Session) error
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
	typeOfActor        = reflect.TypeOf((*iface.IActor)(nil)).Elem()
	typeOfActorContext = reflect.TypeOf((*iface.IContext)(nil)).Elem()
	typeOfError        = reflect.TypeOf((*error)(nil)).Elem()
	typeOfByteArray    = reflect.TypeOf(([]byte)(nil))
	typeOfSession      = reflect.TypeOf((*session.Session)(nil))
)

// AutoRegister 自动扫描并注册 actor 的所有导出方法
func (r *Router) AutoRegister(actor iface.IActor) {
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
		// 检查方法签名是否符合要求
		if !r.isValidMethodSignature(methodType) {
			continue
		}
		// 创建路由条目
		entry, err := r.createMethodEntry(method, methodType)
		if err != nil {
			glog.Warn("路由注册失败", zap.String("method", method.Name), zap.Error(err))
			continue
		}

		r.mu.Lock()
		r.methodRoutes[method.Name] = entry
		r.mu.Unlock()
		glog.Debug("自动注册路由", zap.String("method", method.Name))
	}
}

func (r *Router) isValidMethodSignature(methodType reflect.Type) bool {
	numIn := methodType.NumIn() - 1 // 减去接收者
	if numIn < 2 || numIn > 3 {
		return false
	}
	if !methodType.In(0).Implements(typeOfActor) {
		return false
	}
	// 第一个实际参数必须是 IContext
	if methodType.In(1) != typeOfActorContext {
		return false
	}
	// 检查返回值：必须返回 error
	if methodType.NumOut() != 1 || !methodType.Out(0).Implements(typeOfError) {
		return false
	}
	return true
}

// parseRequestType 解析请求类型
func parseRequestType(requestType reflect.Type) (reflect.Type, bool, error) {
	if requestType == typeOfByteArray {
		return requestType, true, nil
	}
	if requestType.Kind() == reflect.Pointer {
		return requestType, false, nil
	}
	return nil, false, ErrAsyncHandlerThirdParameter
}

// createMethodEntry 为 actor 的方法创建路由条目
// 方法签名: func (a *Actor) MethodName(ctx IContext, ...) error
func (r *Router) createMethodEntry(method reflect.Method, methodType reflect.Type) (routerEntry, error) {
	var entry routerEntry
	entry.handler = method

	numIn := methodType.NumIn() - 1 // 减去接收者

	switch numIn {
	case 2:
		// 2个参数: (actor, ctx, param1) error
		if err := r.parseTwoParamEntry(&entry, methodType); err != nil {
			return entry, err
		}
	case 3:
		// 3个参数: (actor, ctx, param1, param2) error
		if err := r.parseThreeParamEntry(&entry, methodType); err != nil {
			return entry, err
		}
	default:
		return entry, xerror.Wrapf(ErrUnknownHandlerType, "参数数量=%d", numIn)
	}

	// 检查返回值：必须返回 error
	if methodType.NumOut() != 1 || !methodType.Out(0).Implements(typeOfError) {
		return entry, ErrHandlerReturnType
	}

	return entry, nil
}

// parseTwoParamEntry 解析2个参数的方法签名
// 可能的签名:
//   - (actor, ctx, session *session.Session) error - 仅会话消息
//   - (actor, ctx, request) error - 异步消息
func (r *Router) parseTwoParamEntry(entry *routerEntry, methodType reflect.Type) error {
	param1Type := methodType.In(2)
	if param1Type == typeOfSession {
		// (actor, ctx, session *session.Session) error - 仅会话消息
		entry.handlerType = handlerTypeSessionOnly
	} else {
		// (actor, ctx, request) error - 异步消息
		entry.handlerType = handlerTypeAsync
		requestType, isByte, err := parseRequestType(param1Type)
		if err != nil {
			return xerror.Wrap(err, "解析异步消息请求类型失败")
		}
		entry.requestType = requestType
		entry.isByteRequest = isByte
	}
	return nil
}

// parseThreeParamEntry 解析3个参数的方法签名
// 可能的签名:
//   - (actor, ctx, session *session.Session, request) error - 会话消息
//   - (actor, ctx, request, response) error - 同步消息
func (r *Router) parseThreeParamEntry(entry *routerEntry, methodType reflect.Type) error {
	param1Type := methodType.In(2)
	param2Type := methodType.In(3)

	if param1Type == typeOfSession {
		// (actor, ctx, session *session.Session, request) error - 会话消息
		entry.handlerType = handlerTypeSession
		requestType, isByte, err := parseRequestType(param2Type)
		if err != nil {
			return xerror.Wrap(err, "解析会话消息请求类型失败")
		}
		entry.requestType = requestType
		entry.isByteRequest = isByte
	} else {
		// (actor, ctx, request, response) error - 同步消息
		entry.handlerType = handlerTypeSync
		requestType, isByte, err := parseRequestType(param1Type)
		if err != nil {
			return xerror.Wrap(err, "解析同步消息请求类型失败")
		}
		entry.requestType = requestType
		entry.isByteRequest = isByte

		// 验证响应类型必须是指针
		if param2Type.Kind() != reflect.Pointer {
			return xerror.Wrapf(ErrSyncHandlerFourParameter, "响应类型=%v", param2Type)
		}
		entry.responseType = param2Type
	}
	return nil
}

// Handle 基于方法名处理消息，从 message.Data 反序列化参数
func (r *Router) Handle(ctx iface.IContext, methodName string, session iface.ISession, data []byte) ([]byte, error) {
	r.mu.RLock()
	entry, ok := r.methodRoutes[methodName]
	r.mu.RUnlock()

	if !ok {
		return nil, xerror.Wrapf(ErrMessageHandlerNotFound, "method=%s", methodName)
	}

	switch entry.handlerType {
	case handlerTypeSync:
		return r.handleSyncMessage(ctx, methodName, data, entry)
	case handlerTypeAsync:
		return r.handleAsyncMessage(ctx, methodName, data, entry)
	case handlerTypeSession:
		return r.handleSessionMessage(ctx, methodName, session, data, entry)
	case handlerTypeSessionOnly:
		return r.handleSessionOnlyMessage(ctx, methodName, session, entry)
	default:
		return nil, ErrUnknownHandlerType
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

	callArgs := []reflect.Value{reflect.ValueOf(actor), reflect.ValueOf(ctx)}

	// 会话消息需要 session 参数
	if entry.handlerType == handlerTypeSession || entry.handlerType == handlerTypeSessionOnly {
		callArgs = append(callArgs, reflect.ValueOf(session))
	}

	// 反序列化请求参数（仅当需要request时）
	if entry.handlerType != handlerTypeSessionOnly {
		requestValue, err := r.createRequestValue(entry.requestType, entry.isByteRequest, data, ctx)
		if err != nil {
			return nil, xerror.Wrap(err, "构建请求参数失败")
		}
		callArgs = append(callArgs, requestValue)
	}

	// 同步消息需要 response
	if entry.handlerType == handlerTypeSync {
		responseValue := reflect.New(entry.responseType.Elem())
		callArgs = append(callArgs, responseValue)
	}

	return callArgs, nil
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
	responseData, err := ctx.Node().Marshal(responseValue.Interface())
	if err != nil {
		return nil, xerror.Wrap(err, "序列化响应失败")
	}

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

// handleSessionMessage 处理会话消息
func (r *Router) handleSessionMessage(ctx iface.IContext, methodName string, session iface.ISession, data []byte, entry routerEntry) ([]byte, error) {
	callArgs, err := r.buildCallArgs(ctx, entry, session, data)
	if err != nil {
		return nil, err
	}
	return r.callHandler(entry.handler, callArgs)
}

// handleSessionOnlyMessage 处理仅会话消息（无request参数）
func (r *Router) handleSessionOnlyMessage(ctx iface.IContext, methodName string, session iface.ISession, entry routerEntry) ([]byte, error) {
	callArgs, err := r.buildCallArgs(ctx, entry, session, nil)
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

	if err := ctx.Node().Unmarshal(data, requestValue.Interface()); err != nil {
		return reflect.Value{}, xerror.Wrapf(err, "反序列化请求参数失败 (type=%v)", requestType)
	}
	return requestValue, nil
}
