package iface

import (
	"errors"
	"fmt"

	"github.com/dzm2020/gas/internal/pb"
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
	_ IMessage = (*ActorMessage)(nil)
	_ IMessage = (*TaskMessage)(nil)
)

type (
	IMessage interface {
		Validate() error
	}
	TaskMessage struct {
		Task Task
	}

	ActorMessage struct {
		*pb.Message
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
		Message: &pb.Message{
			To:      to,
			From:    from,
			Method:  methodName,
			Data:    data,
			Session: &pb.Session{},
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
	if m.GetTo().GetActorId() == 0 && m.GetTo().GetActorName() == "" {
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

func NewPid(nodeId uint64, actorId uint64) *Pid {
	return &Pid{
		NodeId:  nodeId,
		ActorId: actorId,
	}
}
func NewPidWithName(name string, nodeId uint64) *Pid {
	return &Pid{
		ActorName: name,
		NodeId:    nodeId,
	}
}

func NewResponse(data []byte, err error) *Response {
	response := &pb.Response{
		Data: data,
	}
	if err != nil {
		response.ErrMsg = err.Error()
	}
	return &Response{Response: response}
}

type Response struct {
	*pb.Response
}

func (r *Response) GetError() error {
	if r.ErrMsg == "" {
		return nil
	}
	return errors.New(r.GetErrMsg())
}
