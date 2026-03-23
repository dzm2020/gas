package actor

import (
	"testing"

	"github.com/dzm2020/gas/internal/iface"
	"github.com/dzm2020/gas/pkg/lib/serializer"
)

type stubSubmitter struct {
	tasks []iface.Task
	pids  []*iface.Pid
}

func (s *stubSubmitter) SubmitTask(to *iface.Pid, task iface.Task) error {
	s.pids = append(s.pids, to)
	s.tasks = append(s.tasks, task)
	return nil
}

func TestLocalEventBus_PublishLocal_dispatchesViaSubmitTask(t *testing.T) {
	st := &stubSubmitter{}
	b := newLocalEventBus(1, st)
	pid := iface.NewPid(1, 100)
	var gotTopic string
	var gotPayload []byte
	sub, err := b.Subscribe("t1", pid, func(topic string, payload []byte) {
		gotTopic = topic
		gotPayload = append([]byte(nil), payload...)
	})
	if err != nil {
		t.Fatal(err)
	}
	b.PublishLocal("t1", []byte("hello"))
	if len(st.tasks) != 1 {
		t.Fatalf("want 1 SubmitTask, got %d", len(st.tasks))
	}
	if st.pids[0].GetActorId() != 100 {
		t.Fatalf("wrong pid: %+v", st.pids[0])
	}
	if err := st.tasks[0](nil); err != nil {
		t.Fatal(err)
	}
	if gotTopic != "t1" || string(gotPayload) != "hello" {
		t.Fatalf("got topic=%q payload=%q", gotTopic, gotPayload)
	}
	if err := sub.Unsubscribe(); err != nil {
		t.Fatal(err)
	}
	b.PublishLocal("t1", []byte("x"))
	if len(st.tasks) != 1 {
		t.Fatal("unsub 后不应再投递任务")
	}
}

func TestLocalEventBus_Subscribe_emptyTopic(t *testing.T) {
	b := newLocalEventBus(1, &stubSubmitter{})
	_, err := b.Subscribe("", iface.NewPid(1, 1), func(string, []byte) {})
	if err != ErrEventTopicEmpty {
		t.Fatalf("want ErrEventTopicEmpty, got %v", err)
	}
}

func TestLocalEventBus_Subscribe_nilSubscriber(t *testing.T) {
	b := newLocalEventBus(1, &stubSubmitter{})
	_, err := b.Subscribe("t", nil, func(string, []byte) {})
	if err != ErrEventSubscriberNil {
		t.Fatalf("want ErrEventSubscriberNil, got %v", err)
	}
}

func TestLocalEventBus_Subscribe_wrongNode(t *testing.T) {
	b := newLocalEventBus(1, &stubSubmitter{})
	_, err := b.Subscribe("t", iface.NewPid(999, 1), func(string, []byte) {})
	if err != ErrEventSubscriberNode {
		t.Fatalf("want ErrEventSubscriberNode, got %v", err)
	}
}

func TestLocalEventBus_PublishCluster(t *testing.T) {
	b := newLocalEventBus(1, &stubSubmitter{})
	err := b.PublishCluster("x", []byte("y"))
	if err != ErrEventNoCluster {
		t.Fatalf("want ErrEventNoCluster, got %v", err)
	}
}

func TestNewSystem_IEventBus(t *testing.T) {
	s := NewSystem(1, serializer.Json)
	if s.IEventBus == nil {
		t.Fatal("IEventBus 不应为 nil")
	}
}
