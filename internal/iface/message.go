package iface

import (
	"errors"
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
	_ IMessageValidator = (*Message)(nil)
	_ IMessageValidator = (*SyncMessage)(nil)
	_ IMessageValidator = (*TaskMessage)(nil)
)

type TaskMessage struct {
	Task Task
}

// Validate 验证任务消息是否合法
func (m *TaskMessage) Validate() error {
	if m == nil {
		return errors.New("task message is nil")
	}
	if m.Task == nil {
		return errors.New("task is nil")
	}
	return nil
}

func BuildMessage(ser lib.ISerializer, from, to *Pid, msgId int64, request interface{}) (*Message, error) {
	message := &Message{
		To:   to,
		From: from,
		Id:   msgId,
	}
	data, err := marshal(ser, request)
	if err != nil {
		return nil, err
	}
	message.Data = data
	return message, nil
}

func marshal(serializer lib.ISerializer, request interface{}) ([]byte, error) {
	// 处理 nil 情况
	if request == nil {
		return []byte{}, nil
	}

	switch t := request.(type) {
	case []byte:
		return t, nil
	default:
		data, err := serializer.Marshal(request)
		if err != nil {
			return nil, err
		}
		return data, nil
	}
}

func (m *Message) Response(data []byte, err error) {
	return
}

// Validate 验证消息内容是否合法
func (m *Message) Validate() error {
	if m == nil {
		return errors.New("message is nil")
	}
	// 验证目标进程
	if m.GetTo() == nil {
		return errors.New("message target (To) is nil")
	}
	// 验证目标进程 ID 是否有效
	if m.GetTo().GetServiceId() == 0 && m.GetTo().GetName() == "" {
		return errors.New("message target (To) is invalid: both serviceId and name are empty")
	}
	return nil
}

type SyncMessage struct {
	*Message
	response func(message *Response)
}

// Validate 验证同步消息是否合法
func (m *SyncMessage) Validate() error {
	if m == nil {
		return errors.New("sync message is nil")
	}
	if m.Message == nil {
		return errors.New("sync message inner message is nil")
	}
	// 验证内部消息
	if err := m.Message.Validate(); err != nil {
		return err
	}
	// 同步消息必须设置响应回调
	if m.response == nil {
		return errors.New("sync message response callback is nil")
	}
	return nil
}

func (m *SyncMessage) Response(data []byte, err error) {
	if m.response == nil {
		return
	}
	response := &Response{
		Data: data,
	}
	if err != nil {
		response.Error = err.Error()
	}
	m.response(response)
}

func (m *SyncMessage) SetResponse(f func(*Response)) {
	m.response = f
}

// NewErrorResponse 创建错误响应消息
// errMsg: 错误消息
// 返回: 错误响应消息对象
func NewErrorResponse(errMsg string) *Response {
	return &Response{
		Error: errMsg,
	}
}
