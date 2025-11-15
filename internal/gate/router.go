package gate

import (
	"errors"
	"fmt"
	"gas/internal/gate/protocol"
	"gas/pkg/actor"
	"reflect"
	"sync"

	"google.golang.org/protobuf/proto"
)

type IRouter interface {
	Register(cmd uint8, act uint8, handler interface{}) error
	Handle(session *Session, msg *protocol.Message) error
}

var ErrMessageHandlerNotFound = errors.New("gate: message handler not found")

type protoMessageRouter struct {
	mu     sync.RWMutex
	routes map[uint16]routerEntry
}

type routerEntry struct {
	msgType      reflect.Type
	handler      reflect.Value
	returnsError bool
}

func NewProtoMessageRouter() IRouter {
	return &protoMessageRouter{
		routes: make(map[uint16]routerEntry),
	}
}

var (
	typeOfActorContext = reflect.TypeOf((*actor.IContext)(nil)).Elem()
	typeOfSession      = reflect.TypeOf((*Session)(nil))
	typeOfError        = reflect.TypeOf((*error)(nil)).Elem()
	typeOfProtoMessage = reflect.TypeOf((*proto.Message)(nil)).Elem()
)

func (r *protoMessageRouter) Register(cmd uint8, act uint8, handler interface{}) error {
	if handler == nil {
		return fmt.Errorf("gate: handler is nil")
	}

	handlerValue := reflect.ValueOf(handler)
	handlerType := handlerValue.Type()

	if handlerType.Kind() != reflect.Func {
		return fmt.Errorf("gate: handler must be a function, got %s", handlerType.Kind())
	}

	if handlerType.NumIn() != 3 {
		return fmt.Errorf("gate: handler must accept exactly 3 parameters (ctx, session, message)")
	}

	if handlerType.In(0) != typeOfActorContext {
		return fmt.Errorf("gate: handler first parameter must be actor.IContext")
	}

	if handlerType.In(1) != typeOfSession {
		return fmt.Errorf("gate: handler second parameter must be *gate.Session")
	}

	msgType := handlerType.In(2)
	if msgType.Kind() != reflect.Pointer {
		return fmt.Errorf("gate: handler third parameter must be pointer to proto message, got %s", msgType)
	}
	if !msgType.Implements(typeOfProtoMessage) {
		return fmt.Errorf("gate: handler third parameter must implement proto.Message")
	}

	numOut := handlerType.NumOut()
	if numOut > 1 {
		return fmt.Errorf("gate: handler can return at most one value")
	}

	returnsError := false
	if numOut == 1 {
		if !handlerType.Out(0).Implements(typeOfError) {
			return fmt.Errorf("gate: handler return type must be error")
		}
		returnsError = true
	}

	key := protocol.CmdAct(cmd, act)

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.routes[key]; exists {
		return fmt.Errorf("gate: handler already registered for cmd=%d act=%d", cmd, act)
	}

	r.routes[key] = routerEntry{
		msgType:      msgType,
		handler:      handlerValue,
		returnsError: returnsError,
	}
	return nil
}

func (r *protoMessageRouter) Handle(session *Session, msg *protocol.Message) error {
	if msg == nil {
		return fmt.Errorf("gate: message is nil")
	}
	if session == nil {
		return fmt.Errorf("gate: session is nil")
	}

	key := protocol.CmdAct(msg.Cmd, msg.Act)

	r.mu.RLock()
	entry, ok := r.routes[key]
	r.mu.RUnlock()

	if !ok {
		return ErrMessageHandlerNotFound
	}

	msgValue := reflect.New(entry.msgType.Elem())
	protoMsg, ok := msgValue.Interface().(proto.Message)
	if !ok {
		return fmt.Errorf("gate: message type for cmd=%d act=%d does not implement proto.Message", msg.Cmd, msg.Act)
	}

	if len(msg.Data) > 0 {
		if err := proto.Unmarshal(msg.Data, protoMsg); err != nil {
			return fmt.Errorf("gate: unmarshal proto message (cmd=%d act=%d) failed: %w", msg.Cmd, msg.Act, err)
		}
	}

	if session.agent == nil {
		return fmt.Errorf("gate: session agent is nil")
	}

	return session.agent.PushTask(func(ctx actor.IContext) error {
		args := []reflect.Value{
			reflect.ValueOf(ctx),
			reflect.ValueOf(session),
			msgValue,
		}

		results := entry.handler.Call(args)
		if entry.returnsError && len(results) == 1 && !results[0].IsNil() {
			if err, ok := results[0].Interface().(error); ok {
				return err
			}
			return fmt.Errorf("gate: handler returned non-error value")
		}

		return nil
	})
}
