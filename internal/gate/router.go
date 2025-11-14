package gate

import (
	"errors"
	"fmt"
	"gas/internal/protocol"
	"reflect"
	"sync"

	"google.golang.org/protobuf/proto"
)

type IRouter interface {
	Register(cmd uint8, act uint8, prototype proto.Message, handlerName string) error
	Handle(session *Session, msg *protocol.Message) error
}

var ErrMessageHandlerNotFound = errors.New("gate: message handler not found")

type MessageHandler func(session *Session, message proto.Message) error

type protoMessageRouter struct {
	mu     sync.RWMutex
	routes map[uint16]routerEntry
}

type routerEntry struct {
	msgType     reflect.Type
	handlerName string
}

func NewProtoMessageRouter() IRouter {
	return &protoMessageRouter{
		routes: make(map[uint16]routerEntry),
	}
}

func (r *protoMessageRouter) Register(cmd uint8, act uint8, prototype proto.Message, handlerName string) error {
	if prototype == nil {
		return fmt.Errorf("gate: prototype message is nil")
	}
	if len(handlerName) <= 0 {
		return fmt.Errorf("gate: message handler is nil")
	}

	msgType := reflect.TypeOf(prototype)
	if msgType.Kind() != reflect.Pointer {
		return fmt.Errorf("gate: prototype must be a pointer, got %s", msgType)
	}

	elem := msgType.Elem()
	if elem.Kind() != reflect.Struct {
		return fmt.Errorf("gate: prototype must point to a struct, got %s", elem.Kind())
	}

	key := protocol.CmdAct(cmd, act)

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.routes[key]; exists {
		return fmt.Errorf("gate: handler already registered for cmd=%d act=%d", cmd, act)
	}

	r.routes[key] = routerEntry{
		msgType:     elem,
		handlerName: handlerName,
	}
	return nil
}

func (r *protoMessageRouter) Handle(session *Session, msg *protocol.Message) error {
	if msg == nil {
		return fmt.Errorf("gate: message is nil")
	}
	key := protocol.CmdAct(msg.Cmd, msg.Act)
	r.mu.RLock()
	entry, ok := r.routes[key]
	r.mu.RUnlock()

	if !ok {
		return ErrMessageHandlerNotFound
	}

	instance := reflect.New(entry.msgType).Interface()
	protoMsg, ok := instance.(proto.Message)
	if !ok {
		return fmt.Errorf("gate: registered prototype for cmd=%d act=%d does not implement proto.Message", msg.Cmd, msg.Act)
	}

	if len(msg.Data) > 0 {
		if err := proto.Unmarshal(msg.Data, protoMsg); err != nil {
			return fmt.Errorf("gate: unmarshal proto message (cmd=%d act=%d) failed: %w", msg.Cmd, msg.Act, err)
		}
	}
	return session.actor.Post(entry.handlerName, session, protoMsg)
}
