package iface

import "time"

type IRemote interface {
	Send(message *Message) error
	Request(message *Message, timeout time.Duration) *RespondMessage
}
