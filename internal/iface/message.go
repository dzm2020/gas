package iface

import (
	"gas/internal/errs"
	"gas/pkg/lib"
)

// IMessageValidator 消息验证接口，用于判断消息内容是否合法
type IMessageValidator interface {
	// Validate 验证消息内容是否合法
	// 返回: 验证错误，nil 表示消息合法
	Validate() error
}

// 编译时检查，确保所有消息类型都实现了 IMessageValidator 接口
var (
	_ IMessageValidator = (*ActorMessage)(nil)
	_ IMessageValidator = (*TaskMessage)(nil)
)

func NewTaskMessage(task Task) *TaskMessage {
	return &TaskMessage{
		Task: task,
	}
}

type TaskMessage struct {
	Task Task
}

// Validate 验证任务消息是否合法
func (m *TaskMessage) Validate() error {
	if m == nil {
		return errs.ErrTaskMessageIsNil
	}
	if m.Task == nil {
		return errs.ErrTaskIsNilInMsg
	}
	return nil
}

// NewActorMessage 创建新的消息
// from: 发送方进程 ID
// to: 接收方进程 ID
// methodName: 方法名
func NewActorMessage(from, to *Pid, methodName string, data []byte) *ActorMessage {
	message := &ActorMessage{
		Message: &Message{
			To:      to,
			From:    from,
			Method:  methodName,
			Data:    data,
			Session: &Session{},
		},
	}
	return message
}

func (m *Message) Response(data []byte, err error) {
	return
}

type ActorMessage struct {
	*Message
	response ResponseFunc
}

// Validate 验证同步消息是否合法
func (m *ActorMessage) Validate() error {
	if m == nil {
		return errs.ErrSyncMessageIsNil
	}
	// 验证目标进程
	if m.GetTo() == nil {
		return errs.ErrMessageTargetIsNil
	}

	if m.GetMethod() == "" {
		return errs.ErrMessageMethodIsNil
	}

	// 验证目标进程 ID 是否有效
	if m.GetTo().GetServiceId() == 0 && m.GetTo().GetName() == "" {
		return errs.ErrMessageTargetInvalid
	}

	return nil
}

func (m *ActorMessage) Response(data []byte, err error) {
	if m.response == nil {
		return
	}
	m.response(data, err)
}

func (m *ActorMessage) SetResponse(f ResponseFunc) {
	m.response = f
}

func (m *ActorMessage) GetResponse() ResponseFunc {
	return m.response
}

// NewErrorResponse 创建错误响应消息
// errMsg: 错误消息
// 返回: 错误响应消息对象
func NewErrorResponse(err error) *Response {
	return &Response{
		Error: err.Error(),
	}
}

type ResponseFunc func(data []byte, err error)

func (p *Pid) IsLocal() bool {
	return p.NodeId == GetNode().GetID()
}

func (p *Pid) IsGlobalName() bool {
	return lib.IsFirstLetterUppercase(p.GetName())
}
