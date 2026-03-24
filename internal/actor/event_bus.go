package actor

import (
	"errors"
	"sync"
	"sync/atomic"

	"github.com/dzm2020/gas/internal/iface"
	"github.com/dzm2020/gas/pkg/glog"
	"go.uber.org/zap"
)

const (
	// EventNATSSubjectPrefix 集群事件在消息队列中的 subject 前缀，与节点收件箱 subject（数字 nodeId）隔离。
	// 完整 subject 为 EventNATSSubjectPrefix + topic，例如 gas.event.order.created。
	EventNATSSubjectPrefix = "gas.event."
	// EventNATSSubscribePattern 各节点对集群事件的通配订阅模式（需 JetStream 以外 Core NATS 通配符）。
	EventNATSSubscribePattern = "gas.event.>"
)

var (
	ErrEventTopicEmpty     = errors.New("event: topic 不能为空")
	ErrEventNoCluster      = errors.New("event: 非集群模式或消息队列未就绪，无法 PublishCluster")
	ErrEventUnsubscribed   = errors.New("event: 已取消订阅")
	ErrEventSubscriberNil  = errors.New("event: 订阅者 Pid 不能为空")
	ErrEventSubscriberNode = errors.New("event: 订阅者必须为本节点 Actor")
)

// taskSubmitter 为事件投递到 Actor 邮箱所需的最小能力（由 *System 实现）。
type taskSubmitter interface {
	SubmitTask(to *iface.Pid, task iface.Task) error
}

type subRecord struct {
	id         uint64
	subscriber *iface.Pid
	fn         iface.EventHandler
}

var _ iface.IEventBus = (*localEventBus)(nil)

// eventBus 仅负责本节点：订阅表、PublishLocal、向本机 Actor 邮箱投递回调。
type localEventBus struct {
	mu        sync.RWMutex
	subs      map[string]map[uint64]*subRecord // topic -> 订阅 id -> 记录；Unsubscribe 为 O(1)
	next      uint64
	selfNode  uint64
	submitter taskSubmitter
}

func newLocalEventBus(selfNode uint64, s taskSubmitter) *localEventBus {
	return &localEventBus{
		subs:      make(map[string]map[uint64]*subRecord),
		selfNode:  selfNode,
		submitter: s,
	}
}

func (b *localEventBus) Subscribe(topic string, subscriber *iface.Pid, handler iface.EventHandler) (iface.IEventSubscription, error) {
	if topic == "" {
		return nil, ErrEventTopicEmpty
	}
	if subscriber == nil {
		return nil, ErrEventSubscriberNil
	}
	if subscriber.GetNodeId() != b.selfNode {
		return nil, ErrEventSubscriberNode
	}
	if handler == nil {
		return nil, errors.New("event: handler 不能为空")
	}
	id := atomic.AddUint64(&b.next, 1)
	b.mu.Lock()
	byID := b.subs[topic]
	if byID == nil {
		byID = make(map[uint64]*subRecord)
		b.subs[topic] = byID
	}
	byID[id] = &subRecord{id: id, subscriber: subscriber, fn: handler}
	b.mu.Unlock()
	return &eventSubscription{bus: b, topic: topic, id: id}, nil
}

func (b *localEventBus) PublishLocal(topic string, payload []byte) {
	if topic == "" {
		return
	}
	b.dispatch(topic, payload)
}

func (b *localEventBus) dispatch(topic string, payload []byte) {
	b.mu.RLock()
	sub := b.submitter
	byID := b.subs[topic]
	if len(byID) == 0 {
		b.mu.RUnlock()
		return
	}
	jobs := make([]*subRecord, 0, len(byID))
	for _, r := range byID {
		jobs = append(jobs, r)
	}
	b.mu.RUnlock()
	if sub == nil || len(jobs) == 0 {
		return
	}
	for _, rec := range jobs {
		topicCopy := topic
		pid := rec.subscriber
		fn := rec.fn
		if err := sub.SubmitTask(pid, func(_ iface.IContext) error {
			fn(topicCopy, payload)
			return nil
		}); err != nil {
			glog.Error("event: 投递到订阅 Actor 失败", zap.Error(err), zap.String("topic", topicCopy), zap.Any("subscriber", pid))
		}
	}
}

func (b *localEventBus) remove(topic string, id uint64) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	byID, ok := b.subs[topic]
	if !ok || len(byID) == 0 {
		return ErrEventUnsubscribed
	}
	if _, exists := byID[id]; !exists {
		return ErrEventUnsubscribed
	}
	delete(byID, id)
	if len(byID) == 0 {
		delete(b.subs, topic)
	}
	return nil
}

func (*localEventBus) PublishCluster(string, []byte) error {
	return ErrEventNoCluster
}

type eventSubscription struct {
	bus   *localEventBus
	topic string
	id    uint64
}

func (s *eventSubscription) Unsubscribe() error {
	return s.bus.remove(s.topic, s.id)
}
