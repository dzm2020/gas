package gate

import (
	"gas/internal/protocol"
)

type NetworkConnect struct {
	Agent *Session
}

type NetworkMessage struct {
	Agent   *Session
	Message *protocol.Message
}

type NetworkClose struct {
	Agent *Session
}
