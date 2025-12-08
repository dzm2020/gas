package iface

type TaskMessage struct {
	Task Task
}

func (m *Message) Response(data []byte, err error) {
	return
}

type SyncMessage struct {
	*Message
	response func(message *RespondMessage)
}

func (m *SyncMessage) Response(data []byte, err error) {
	if m.response == nil {
		return
	}
	response := &RespondMessage{
		Data: data,
	}
	if err != nil {
		response.Error = err.Error()
	}
	m.response(response)
}

func (m *SyncMessage) SetResponse(f func(*RespondMessage)) {
	m.response = f
}

type IMessage interface {
	GetId() int64
	GetData() []byte
	GetFrom() *Pid
	GetTo() *Pid
	Response(data []byte, err error)
}
