package iface

import (
	"gas/internal/errs"
	"gas/pkg/lib"
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

	ISession interface {
		SetContext(ctx IContext)
		Response(request interface{}) error
		ResponseCode(code int64) error
		Forward(to interface{}, method string) error
		Push(cmd, act uint16, request interface{}) error
		Close() error
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
		return errs.ErrTaskMessageIsNil
	}
	if m.Task == nil {
		return errs.ErrTaskIsNilInMsg
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

// NewErrorResponse 创建错误响应消息
// errMsg: 错误消息
// 返回: 错误响应消息对象
func NewErrorResponse(err error) *Response {
	return &Response{
		Error: err.Error(),
	}
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

func (p *Pid) IsLocal() bool {
	return p.NodeId == GetNode().GetID()
}

func (p *Pid) IsGlobalName() bool {
	return lib.IsFirstLetterUppercase(p.GetName())
}
