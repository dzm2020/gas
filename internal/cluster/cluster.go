package cluster

import (
	"context"
	"errors"
	"fmt"
	"gas/internal/iface"
	discovery "gas/pkg/discovery/iface"
	"gas/pkg/glog"
	"gas/pkg/lib"
	messageQue "gas/pkg/messageQue/iface"
	"time"

	"github.com/duke-git/lancet/v2/convertor"
	"go.uber.org/zap"
	"golang.org/x/exp/slices"
)

var (
	ErrMethodNotImplemented = errors.New("cluster push task not yet implemented")
	ErrNotFoundMember       = errors.New("not found member")
)

var _ iface.ICluster = (*Cluster)(nil)

type Cluster struct {
	name string
	node iface.INode
	dis  discovery.IDiscovery
	mq   messageQue.IMessageQue
}

func (r *Cluster) PushTask(pid *iface.Pid, f iface.Task) error {
	return ErrMethodNotImplemented
}

func (r *Cluster) PushTaskAndWait(pid *iface.Pid, timeout time.Duration, task iface.Task) error {
	return ErrMethodNotImplemented
}

func (r *Cluster) Start(ctx context.Context) error {
	//  启动组件
	if err := r.mq.Run(ctx); err != nil {
		return err
	}
	if err := r.dis.Run(ctx); err != nil {
		return err
	}
	//  注册节点
	if err := r.dis.Add(r.node.Info()); err != nil {
		return err
	}
	//  订阅消息
	if err := r.subscribe(); err != nil {
		return err
	}
	return nil
}

func (r *Cluster) makeSubject(nodeId uint64) string {
	return fmt.Sprintf("%s%d", r.name, nodeId)
}

func (r *Cluster) subscribe() error {
	nodeId := r.node.GetID()
	subject := r.makeSubject(nodeId)
	_, err := r.mq.Subscribe(subject, r)
	return err
}

func (r *Cluster) HandlerAsyncMessage(data []byte) error {
	message := &iface.Message{}
	if err := r.node.Unmarshal(data, message); err != nil {
		return err
	}
	msg := &iface.ActorMessage{Message: message}
	system := r.node.System()
	return system.Send(msg)
}

func (r *Cluster) HandlerSyncMessage(request []byte) (rspBytes []byte) {
	var bytes []byte
	var err error
	defer func() {
		response := iface.NewResponse(bytes, err)
		rspBytes, _ = r.node.Marshal(response)
	}()
	message := &iface.Message{}
	if err = r.node.Unmarshal(request, message); err != nil {
		return
	}
	msg := &iface.ActorMessage{Message: message}
	system := r.node.System()
	bytes, err = system.Call(msg)
	return
}

// Send 发送消息到集群节点
func (r *Cluster) Send(msg *iface.ActorMessage) (err error) {
	if err = msg.Validate(); err != nil {
		return
	}

	toNodeId := msg.To.GetNodeId()

	if m := r.dis.GetById(toNodeId); m == nil {
		return ErrNotFoundMember
	}

	bytes, mErr := r.node.Marshal(msg)
	if mErr != nil {
		return mErr
	}

	subject := r.makeSubject(toNodeId)
	return r.mq.Publish(subject, bytes)
}

func (r *Cluster) Call(msg *iface.ActorMessage) (bin []byte, err error) {
	if err = msg.Validate(); err != nil {
		return
	}

	toNodeId := msg.To.GetNodeId()

	if m := r.dis.GetById(toNodeId); m == nil {
		err = ErrNotFoundMember
		return
	}

	data, w := r.node.Marshal(msg.Message)
	if w != nil {
		err = w
		return
	}

	subject := r.makeSubject(toNodeId)
	bytes, w := r.mq.Request(subject, data, lib.NowDelay(msg.GetDeadline(), 0))
	if w != nil {
		w = err
		return
	}

	response := &iface.Response{}
	if err = r.node.Unmarshal(bytes, response); err != nil {
		return
	}

	bin = response.GetData()
	err = response.GetError()
	return
}

func (r *Cluster) UpdateMember() error {
	return r.dis.Add(r.node.Info())
}

func (r *Cluster) Select(tag string, strategy discovery.RouteStrategy) uint64 {
	if strategy == nil {
		strategy = discovery.RouteRandom
	}
	// 通过服务发现获取节点列表
	members := r.dis.GetAll()
	if len(members) == 0 {
		return 0
	}

	var selected []*discovery.Member
	for _, node := range members {
		if !slices.Contains(node.Tags, tag) {
			continue
		}
		selected = append(selected, node)
	}

	// 使用路由策略选择节点
	selectedNode := strategy(selected)
	if selectedNode == nil {
		return 0
	}

	return selectedNode.GetID()
}

// Broadcast 向服务的所有节点广播消息
func (r *Cluster) Broadcast(tag string, message *iface.ActorMessage) {
	members := r.dis.GetAll()
	if len(members) == 0 {
		return
	}
	for _, member := range members {
		if !slices.Contains(member.Tags, tag) {
			continue
		}
		msg := convertor.DeepClone(message)
		msg.To = &iface.Pid{
			NodeId: member.GetID(),
			Name:   tag,
		}
		// 发送消息
		if err := r.Send(msg); err != nil {
			glog.Error("集群通信: 广播消息到节点失败",
				zap.Uint64("nodeId", member.GetID()),
				zap.String("tag", tag), zap.Error(err))
		}
	}
	return
}

// Shutdown 关闭所有订阅
func (r *Cluster) Shutdown(ctx context.Context) error {
	if err := r.dis.Shutdown(ctx); err != nil {
		return err
	}
	if err := r.mq.Shutdown(ctx); err != nil {
		return err
	}
	return nil
}
