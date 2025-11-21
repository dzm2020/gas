package actor

import (
	"encoding/json"
	"fmt"
	"gas/examples/pb"
	"gas/internal/iface"
	"testing"
	"time"
)

//	func Hello(ctx IContext, request proto.Message, response proto.Message) error {
//		fmt.Printf("OnMessage:%+v\n", msg)
//
//		return
//	}
func OnMessage(ctx IContext, request, reply *pb.TestMessage) error {
	fmt.Printf("OnMessage:%+v\n", request)
	reply.Data = "222222222222222222"
	return nil
}

type Service struct {
	Actor
	msg string
}

//func (s *Service) OnMessage(ctx IContext, request *pb.TestMessage) error {
//	fmt.Printf("OnMessage:%+v\n", msg)
//	return nil
//}

func TestActor(t *testing.T) {
	Init(1)

	r := NewRouter()
	r.Register(1, OnMessage)

	pid, _ := Spawn(func() IActor {
		return &Service{}
	}, WithRouter(r))

	bin, _ := json.Marshal(&pb.TestMessage{Data: "111111111111111"})
	reply := &pb.TestMessage{}

	msg := &iface.Message{
		To:   pid,
		Id:   1,
		Data: bin,
	}

	Request(msg, reply, time.Second*10)

	time.Sleep(time.Second * 10)
}
