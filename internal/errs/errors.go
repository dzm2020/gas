package errs

import (
	"errors"
	"fmt"
)

// ========== Actor 相关错误 ==========

// 路由相关错误
var (
	// ErrMessageHandlerNotFound 消息处理器未找到错误
	ErrMessageHandlerNotFound = errors.New("actor message handler not found")
)

// 进程相关错误
var (
	// ErrProcessExiting 进程正在退出
	ErrProcessExiting = errors.New("process is exiting")
	// ErrProcessNotFound 进程未找到
	ErrProcessNotFound = errors.New("process not found")
	// ErrTaskIsNil 任务为空
	ErrTaskIsNil = errors.New("task is nil")
	// ErrMessageIsNil 消息为空
	ErrMessageIsNil = errors.New("message is nil")
)

// 系统相关错误
var (
	// ErrSystemShuttingDown 系统正在关闭
	ErrSystemShuttingDown = errors.New("system is shutting down")
	// ErrNodeIsNil 节点为空
	ErrNodeIsNil = errors.New("node is nil")
	// ErrRemoteIsNil 远程接口为空
	ErrRemoteIsNil = errors.New("remote is nil")

	ErrActorSystemIsNil = errors.New("actor system is nil")
)

// 消息相关错误
var (
	// ErrNoMessageToForward 没有消息可转发
	ErrNoMessageToForward = errors.New("no message to forward")
	// ErrFailedToBuildMessage 构建消息失败
	ErrFailedToBuildMessage = errors.New("failed to build message")
)

// 等待器相关错误
var (
	// ErrWaiterTimeout 等待超时错误
	ErrWaiterTimeout = errors.New("waiter timeout")
)

// 路由注册相关错误构造函数

func ErrHandlerIsNil() error {
	return fmt.Errorf("actor: handler is nil")
}

func ErrHandlerMustBeFunction(got string) error {
	return fmt.Errorf("actor: handler must be a function, got %s", got)
}

func ErrHandlerParameterCount(got int) error {
	return fmt.Errorf("actor: handler must accept 2 or 3 parameters, got %d", got)
}

func ErrHandlerFirstParameterMustBeContext() error {
	return fmt.Errorf("actor: handler first parameter must be actor.IContext")
}

func ErrHandlerAlreadyRegistered(msgId int64) error {
	return fmt.Errorf("actor: handler already registered for msgId=%d", msgId)
}

func ErrUnsupportedParameterCount(count int) error {
	return fmt.Errorf("actor: unsupported parameter count: %d", count)
}

func ErrClientHandlerThirdParameter(requestType string) error {
	return fmt.Errorf("actor: client handler third parameter (request) must be pointer or []byte, got %s", requestType)
}

func ErrSyncHandlerSecondParameter(requestType string) error {
	return fmt.Errorf("actor: sync handler second parameter (request) must be pointer or []byte, got %s", requestType)
}

func ErrSyncHandlerThirdParameter(responseType string) error {
	return fmt.Errorf("actor: sync handler third parameter (response) must be pointer, got %s", responseType)
}

func ErrHandlerReturnCount(got int) error {
	return fmt.Errorf("actor: handler must return exactly one value (error), got %d", got)
}

func ErrHandlerReturnType() error {
	return fmt.Errorf("actor: handler return type must be error")
}

func ErrAsyncHandlerSecondParameter(typ string) error {
	return fmt.Errorf("actor: handler second paramter must be point ,got %s", typ)
}

func ErrUnknownHandlerType(msgId int64) error {
	return fmt.Errorf("actor: unknown handler type for msgId:%d", msgId)
}

func ErrSessionIsNil(msgId int64) error {
	return fmt.Errorf("actor: session is nil for msgId:%d", msgId)
}

func ErrUnmarshalRequest(msgId int64, err error) error {
	return fmt.Errorf("actor: unmarshal request message msgId:%d failed: %w", msgId, err)
}

func ErrMarshalResponse(msgId int64, err error) error {
	return fmt.Errorf("actor: marshal response message msgId:%d failed: %w", msgId, err)
}

func ErrUnmarshalFailed(err error) error {
	return fmt.Errorf("unmarshal failed: %w", err)
}

// 系统相关错误构造函数

func ErrNameCannotBeEmpty() error {
	return fmt.Errorf("name cannot be empty")
}

func ErrNodeNotInitialized() error {
	return fmt.Errorf("node is not initialized")
}

func ErrRemoteNotInitialized() error {
	return fmt.Errorf("remote is not initialized")
}

func ErrRemoteRegistryNameFailed(err error) error {
	return fmt.Errorf("remote registry name failed: %w", err)
}

func ErrNameAlreadyRegistered(name string) error {
	return fmt.Errorf("name '%s' is already registered", name)
}

func ErrProcessAlreadyHasName(processId uint64) error {
	return fmt.Errorf("process %d already has a registered name", processId)
}

func ErrNameChangeNotAllowed() error {
	return fmt.Errorf("name change is not allowed")
}

// 上下文相关错误构造函数

func ErrUnsupportedMessageType(msgType string) error {
	return fmt.Errorf("unsupported message type: %s", msgType)
}

func ErrMsgIsNil() error {
	return fmt.Errorf("msg is nil")
}

func ErrRequestFailed(errMsg string) error {
	return fmt.Errorf("request failed: %s", errMsg)
}

func ErrUnmarshalReplyFailed(err error) error {
	return fmt.Errorf("unmarshal reply failed: %w", err)
}

func ErrInvalidMessage(err error) error {
	return fmt.Errorf("invalid message: %w", err)
}

// ========== Gate 相关错误 ==========

var (
	ErrAgentFactoryNil       = errors.New("gate: agent factory is nil")
	ErrAgentNoBindConnection = errors.New("no bind connection")
	ErrInvalidMessageType    = errors.New("gate: invalid message type")
)

// ========== Component 相关错误 ==========

func ErrComponentCannotBeNil() error {
	return fmt.Errorf("component cannot be nil")
}

func ErrComponentNameCannotBeEmpty() error {
	return fmt.Errorf("component name cannot be empty")
}

func ErrCannotRegisterComponentAfterStarted() error {
	return fmt.Errorf("cannot register component after manager has started")
}

func ErrComponentAlreadyRegistered(name string) error {
	return fmt.Errorf("component with name '%s' already registered", name)
}

func ErrManagerAlreadyStarted() error {
	return fmt.Errorf("manager has already been started")
}

func ErrManagerStoppedCannotRestart() error {
	return fmt.Errorf("manager has been stopped and cannot be restarted")
}

func ErrFailedToStartComponent(name string, err error) error {
	return fmt.Errorf("failed to start component '%s': %w", name, err)
}

// ========== Config 相关错误 ==========

func ErrReadConfigFileFailed(err error) error {
	return fmt.Errorf("read config file failed: %w", err)
}

func ErrUnmarshalConfigFailed(err error) error {
	return fmt.Errorf("unmarshal config failed: %w", err)
}

// ========== Remote 相关错误 ==========

func ErrSubscribeToNodeFailed(nodeId uint64, err error) error {
	return fmt.Errorf("subscribe to game-node %d failed: %w", nodeId, err)
}

func ErrMarshalMessageFailed(err error) error {
	return fmt.Errorf("marshal message failed: %w", err)
}

func ErrPublishToRemoteNodeFailed(toNodeId uint64, err error) error {
	return fmt.Errorf("publish to remote game-node %d failed: %w", toNodeId, err)
}

func ErrNoNodesFoundForService(service string) error {
	return fmt.Errorf("no nodes found for service: %s", service)
}

func ErrRouteStrategyReturnedNilNode(service string) error {
	return fmt.Errorf("route strategy returned nil node for service: %s", service)
}

func ErrCreateDiscoveryFailed(err error) error {
	return fmt.Errorf("create discovery failed: %w", err)
}

func ErrCreateMessageQueueFailed(err error) error {
	return fmt.Errorf("create message queue failed: %w", err)
}

// ========== Node 相关错误 ==========

func ErrRegisterComponentFailed(name string, err error) error {
	return fmt.Errorf("注册组件失败 name:%v err:%v", name, err)
}

func ErrStartComponentFailed(err error) error {
	return fmt.Errorf("启动组件失败 err:%w", err)
}

// ========== Codec 相关错误 ==========

func ErrInvalidCodecMessageType() error {
	return fmt.Errorf("invalid message type")
}

// ========== Message 相关错误 ==========
var (
	ErrTaskMessageIsNil                 = errors.New("task message is nil")
	ErrTaskIsNilInMsg                   = errors.New("task is nil")
	ErrMessageIsNilInMsg                = errors.New("message is nil")
	ErrMessageTargetIsNil               = errors.New("message target (To) is nil")
	ErrMessageTargetInvalid             = errors.New("message target (To) is invalid: both serviceId and name are empty")
	ErrSyncMessageIsNil                 = errors.New("sync message is nil")
	ErrSyncMessageInnerIsNil            = errors.New("sync message inner message is nil")
	ErrSyncMessageResponseCallbackIsNil = errors.New("sync message response callback is nil")
)

// ========== Session 相关错误 ==========

var (
	ErrNoMessageToForwardInSession = errors.New("no message to forward")
)

// ========== System 相关错误 ==========

func ErrMessageIsNilInSystem() error {
	return fmt.Errorf("message is nil")
}
