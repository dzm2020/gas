package iface

// EventHandler 事件回调；由总线通过 SubmitTask 投递后，在订阅该 topic 的 Actor 邮箱协程中执行。
type EventHandler func(topic string, payload []byte)

// IEventSubscription 取消本地订阅。
type IEventSubscription interface {
	Unsubscribe() error
}

// IEventBus Actor 事件总线：本地 PublishLocal；集群 PublishCluster（经 MQ 广播，各节点再投递本地订阅者）。
// Subscribe 的 handler 始终在 subscriber 所在 Actor 的邮箱协程中执行。
type IEventBus interface {
	Subscribe(topic string, subscriber *Pid, handler EventHandler) (IEventSubscription, error)
	PublishLocal(topic string, payload []byte)
	PublishCluster(topic string, payload []byte) error
}
