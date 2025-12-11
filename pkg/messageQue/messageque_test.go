package messageQue

import (
	"fmt"
	"gas/pkg/messageQue/provider/nats"
	"testing"
	"time"
)

const Topic = "topic1"

func TestMessageQue(t *testing.T) {

	client, err := nats.New([]string{"127.0.0.1:4222"}, nil)
	if err != nil {
		t.Fatal(err)
	}

	sub, err := client.Subscribe(Topic, func(data []byte, reply func([]byte) error) {
		fmt.Println("Received message: ", string(data))
		if reply != nil {
			_ = reply(data)
		}
	})
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
