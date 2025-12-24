package messageQue

import (
	"context"
	"fmt"
	"gas/pkg/messageQue/provider/nats"
	"testing"
	"time"
)

const Topic = "topic1"

type testSubscriber struct{}

func (t *testSubscriber) OnMessage(request []byte) ([]byte, error) {
	fmt.Println("Received message: ")
	return request, nil
}

func TestMessageQue(t *testing.T) {

	client := nats.New(nil)
	err := client.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	sub, err := client.Subscribe(Topic, &testSubscriber{})
	if err != nil {
		t.Fatal(err)
	}

	defer sub.Unsubscribe()

	client.Publish(Topic, []byte("111111111111111111"))

	res, err := client.Request(Topic, []byte("111111111111111111"), time.Second)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Printf("res:%v\n", string(res))

	time.Sleep(10 * time.Second)
}
