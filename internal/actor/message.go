package actor

import (
	"gas/internal/iface"
)

type TaskMessage struct {
	task iface.Task
}

type SyncMessage struct {
	*iface.Message
	response func(message *iface.RespondMessage)
}

func (m *SyncMessage) Response(data []byte, err error) {
	if m.response == nil {
		return
	}
	response := &iface.RespondMessage{
		Data: data,
	}
	if err != nil {
		response.Error = err.Error()
	}
	m.response(response)
}

func (m *SyncMessage) SetResponse(f func(*iface.RespondMessage)) {
	m.response = f
}
