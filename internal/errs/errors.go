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

	ErrMessageMethodIsNil = fmt.Errorf("msg method is nil")
)

// 系统相关错误
var (
	// ErrSystemShuttingDown 系统正在关闭
	ErrSystemShuttingDown = errors.New("system is shutting down")
)

// 等待器相关错误
var (
	// ErrWaiterTimeout 等待超时错误
	ErrWaiterTimeout = errors.New("waiter timeout")
)

func ErrUnsupportedParameterCount(count int) error {
	return fmt.Errorf("actor: unsupported parameter count: %d", count)
}

func ErrSyncHandlerThirdParameter(responseType string) error {
	return fmt.Errorf("actor: sync handler third parameter (response) must be pointer, got %s", responseType)
}

func ErrHandlerReturnType() error {
	return fmt.Errorf("actor: handler return type must be error")
}

func ErrAsyncHandlerThirdParameter(typ string) error {
	return fmt.Errorf("actor: async handler third parameter (request) must be pointer or []byte, got %s", typ)
}

func ErrClientHandlerFourthParameter(typ string) error {
	return fmt.Errorf("actor: client handler fourth parameter (request) must be pointer or []byte, got %s", typ)
}

func ErrSyncHandlerFourthParameter(responseType string) error {
	return fmt.Errorf("actor: sync handler fourth parameter (response) must be pointer, got %s", responseType)
}

func ErrActorIsNil() error {
	return fmt.Errorf("actor: actor is nil")
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

// 系统相关错误构造函数

func ErrNameCannotBeEmpty() error {
	return fmt.Errorf("name cannot be empty")
}

func ErrRemoteRegistryNameFailed(err error) error {
	return fmt.Errorf("remote registry name failed: %w", err)
}

func ErrNameAlreadyRegistered(name string) error {
	return fmt.Errorf("name '%s' is already registered", name)
}

func ErrNameChangeNotAllowed() error {
	return fmt.Errorf("name change is not allowed")
}

// 上下文相关错误构造函数

func ErrUnsupportedMessageType(msgType string) error {
	return fmt.Errorf("unsupported message type: %s", msgType)
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
	ErrMessageTargetIsNil               = errors.New("message target (To) is nil")
	ErrMessageTargetInvalid             = errors.New("message target (To) is invalid: both serviceId and name are empty")
	ErrSyncMessageIsNil                 = errors.New("sync message is nil")
	ErrSyncMessageResponseCallbackIsNil = errors.New("sync message response callback is nil")
)
