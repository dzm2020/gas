package iface

type TaskMessage struct {
	Task Task
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
