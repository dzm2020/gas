package iface

import (
	"errors"
	"fmt"
	"gas/pkg/lib"
)

var (
	ErrMessageMethodIsNil   = fmt.Errorf("msg method is nil")
	ErrTaskMessageIsNil     = errors.New("task message is nil")
	ErrTaskIsNilInMsg       = errors.New("task is nil")
	ErrMessageTargetIsNil   = errors.New("message target (To) is nil")
	ErrMessageTargetInvalid = errors.New("message target (To) is invalid: both serviceId and name are empty")
	ErrSyncMessageIsNil     = errors.New("sync message is nil")
)

// 编译时检查，确保所有消息类型都实现了 IMessageValidator 接口
var (
	_ IMessageValidator = (*ActorMessage)(nil)
	_ IMessageValidator = (*TaskMessage)(nil)
)

type (
	IMessageValidator interface {
		Validate() error
	}
	TaskMessage struct {
		Task Task
	}

	ActorMessage struct {
		*Message
		response ResponseFunc
	}

	ResponseFunc func(data []byte, err error)
)

func NewTaskMessage(task Task) *TaskMessage {
	return &TaskMessage{
		Task: task,
	}
}

// Validate 验证任务消息是否合法
func (m *TaskMessage) Validate() error {
	if m == nil {
		return ErrTaskMessageIsNil
	}
	if m.Task == nil {
		return ErrTaskIsNilInMsg
	}
	return nil
}

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

// Validate 验证同步消息是否合法
func (m *ActorMessage) Validate() error {
	if m == nil {
		return ErrSyncMessageIsNil
	}
	// 验证目标进程
	if m.GetTo() == nil {
		return ErrMessageTargetIsNil
	}

	if m.GetMethod() == "" {
		return ErrMessageMethodIsNil
	}

	// 验证目标进程 ID 是否有效
	if m.GetTo().GetServiceId() == 0 && m.GetTo().GetName() == "" {
		return ErrMessageTargetInvalid
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

func NewPid(nodeId uint64, serviceId uint64) *Pid {
	return &Pid{
		NodeId:    nodeId,
		ServiceId: serviceId,
	}
}
func NewPidWithName(name string, nodeId uint64) *Pid {
	return &Pid{
		Name:   name,
		NodeId: nodeId,
	}
}

func (p *Pid) IsGlobalName() bool {
	return lib.IsFirstLetterUppercase(p.GetName())
}

func NewResponse(data []byte, err error) *Response {
	response := &Response{
		Data: data,
	}
	if err != nil {
		response.ErrMsg = err.Error()
	}
	return response
}

func (r *Response) GetError() error {
	if r.ErrMsg == "" {
		return nil
	}
	return errors.New(r.GetErrMsg())
}
