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
		return errs.ErrTaskMessageIsNil
	}
	if m.Task == nil {
		return errs.ErrTaskIsNilInMsg
	}
	return nil
}

func NewMessage(from, to *Pid, msgId int64) *Message {
	message := &Message{
		To:   to,
		From: from,
		Id:   msgId,
	}
	return message
}

func (m *Message) Response(data []byte, err error) {
	return
}

// Validate 验证消息内容是否合法
func (m *Message) Validate() error {
	if m == nil {
		return errs.ErrMessageIsNilInMsg
	}
	// 验证目标进程
	if m.GetTo() == nil {
		return errs.ErrMessageTargetIsNil
	}
	// 验证目标进程 ID 是否有效
	if m.GetTo().GetServiceId() == 0 && m.GetTo().GetName() == "" {
		return errs.ErrMessageTargetInvalid
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
		return errs.ErrSyncMessageIsNil
	}
	if m.Message == nil {
		return errs.ErrSyncMessageInnerIsNil
	}
	// 验证内部消息
	if err := m.Message.Validate(); err != nil {
		return err
	}
	// 同步消息必须设置响应回调
	if m.response == nil {
		return errs.ErrSyncMessageResponseCallbackIsNil
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

// Marshal 将请求对象序列化为字节数组
// serializer: 序列化器
// request: 要序列化的请求对象，可以是任意类型或 []byte
// 返回: 序列化后的字节数组和错误
func Marshal(serializer lib.ISerializer, request interface{}) ([]byte, error) {
	// 处理 nil 情况
	if request == nil {
		return []byte{}, nil
	}

	// 如果已经是 []byte 类型，直接返回
	if data, ok := request.([]byte); ok {
		return data, nil
	}

	// 使用序列化器序列化
	return serializer.Marshal(request)
}

// Unmarshal 将字节数组反序列化为目标对象
// serializer: 序列化器
// data: 要反序列化的字节数组
// reply: 目标对象指针，用于接收反序列化后的数据
// 返回: 反序列化错误
func Unmarshal(serializer lib.ISerializer, data []byte, reply interface{}) error {
	// 如果数据为空，直接返回
	if len(data) == 0 {
		return nil
	}

	// 如果目标对象为空，直接返回
	if reply == nil {
		return nil
	}

	// 如果目标对象是指向 []byte 的指针，直接将数据赋值
	if ptr, ok := reply.(*[]byte); ok {
		*ptr = data
		return nil
	}

	// 使用序列化器反序列化
	return serializer.Unmarshal(data, reply)
}
