package cluster

import (
	"context"
	"errors"
	"fmt"
	"gas/internal/iface"
	discovery "gas/pkg/discovery/iface"
	"gas/pkg/lib"
	"gas/pkg/lib/glog"
	messageQue "gas/pkg/messageQue/iface"
	"time"

	"github.com/duke-git/lancet/v2/convertor"
	"go.uber.org/zap"
	"golang.org/x/exp/slices"
)

var _ iface.ICluster = (*Cluster)(nil)

func New(discovery discovery.IDiscovery, messageQue messageQue.IMessageQue, subjectPrefix string) *Cluster {
	return &Cluster{
		discovery:         discovery,
		messageQue:        messageQue,
		nodeSubjectPrefix: subjectPrefix,
	}
}

// Cluster 集群通信管理器，负责处理跨节点的消息传递
type Cluster struct {
	discovery         discovery.IDiscovery
	messageQue        messageQue.IMessageQue
	nodeSubjectPrefix string
}

func (r *Cluster) PushTask(pid *iface.Pid, f iface.Task) error {
	return errors.New("cluster push task not yet implemented")
}

func (r *Cluster) PushTaskAndWait(pid *iface.Pid, timeout time.Duration, task iface.Task) error {
	return errors.New("cluster push task not yet implemented")
}

func (r *Cluster) Start(ctx context.Context) error {
	if err := r.messageQue.Run(ctx); err != nil {
		return err
	}
	if err := r.discovery.Run(ctx); err != nil {
		return err
	}
	if err := r.discovery.Add(iface.GetNode().Info()); err != nil {
		return err
	}
	if err := r.listen(); err != nil {
		return err
	}
	return nil
}

func (r *Cluster) makeTopic(nodeId uint64) string {
	return fmt.Sprintf("%s%d", r.nodeSubjectPrefix, nodeId)
}

func (r *Cluster) listen() error {
	nodeId := iface.GetNode().GetID()
	handler := func(data []byte, reply func([]byte) error) {
		if err := r.onRemoteProcess(data, reply); err != nil {
			glog.Error("集群通信: 处理集群消息失败", zap.Error(err))
		}
	}
	subject := r.makeTopic(nodeId)
	_, err := r.messageQue.Subscribe(subject, handler)
	return err
}

// onRemoteProcess 处理集群消息
func (r *Cluster) onRemoteProcess(data []byte, reply func([]byte) error) error {
	node := iface.GetNode()

	message := &iface.Message{}
	node.Unmarshal(data, message)

	glog.Debug("集群通信: 处理集群消息", zap.Any("message", message))

	msg := &iface.ActorMessage{Message: message}

	if msg.GetAsync() {
		return node.Send(msg)
	}

	bin, w := node.Call(msg)

	response := &iface.Response{
		Data:  bin,
		Error: w.Error(),
	}
	if reply != nil {
		return reply(node.Marshal(response))
	}

	return nil
}

// sendError 发送错误响应
func (r *Cluster) sendError(reply func([]byte) error, err error) {
	if reply == nil {
		return
	}
	glog.Error("集群通信: 发送错误响应", zap.Error(err))
	errorResponse := iface.NewErrorResponse(err)
	if w := reply(iface.GetNode().Marshal(errorResponse)); w != nil {
		glog.Error("集群通信: 回复消息", zap.Error(w))
	}
}

// Send 发送消息到集群节点
func (r *Cluster) Send(rpcMessage *iface.ActorMessage) (err error) {
	if err = rpcMessage.Validate(); err != nil {
		return
	}

	toNodeId := rpcMessage.To.GetNodeId()

	if m := r.discovery.GetById(toNodeId); m == nil {
		err = errors.New("not found member")
		return
	}

	data := iface.GetNode().Marshal(rpcMessage)
	subject := r.makeTopic(toNodeId)

	return r.messageQue.Publish(subject, data)
}

func (r *Cluster) Call(rpcMessage *iface.ActorMessage) (bin []byte, err error) {

	if err = rpcMessage.Validate(); err != nil {
		return
	}

	toNodeId := rpcMessage.To.GetNodeId()

	if m := r.discovery.GetById(toNodeId); m == nil {
		err = errors.New("not found member")
		return
	}

	data := iface.GetNode().Marshal(rpcMessage.Message)
	subject := r.makeTopic(toNodeId)

	rspData, w := r.messageQue.Request(subject, data, lib.NowDelay(rpcMessage.Deadline, 0))
	if w != nil {
		err = w
		return
	}

	response := &iface.Response{}
	iface.GetNode().Unmarshal(rspData, response)

	bin, err = response.Data, errors.New(response.Error)
	return
}

func (r *Cluster) UpdateNode() error {
	return r.discovery.Add(iface.GetNode().Info())
}

func (r *Cluster) Select(service string, strategy iface.RouteStrategy) *iface.Pid {
	if strategy == nil {
		strategy = iface.RouteRandom
	}

	// 通过服务发现获取节点列表
	members := r.discovery.GetAll()
	if len(members) == 0 {
		return nil
	}

	var filteredMembers []*discovery.Member
	for _, node := range members {
		if !slices.Contains(node.Tags, service) {
			continue
		}
		filteredMembers = append(filteredMembers, node)
	}

	// 使用路由策略选择节点
	selectedNode := strategy(filteredMembers)
	if selectedNode == nil {
		return nil
	}

	// 设置消息的目标节点ID
	return &iface.Pid{
		NodeId: selectedNode.GetID(),
		Name:   service,
	}
}

// Broadcast 向服务的所有节点广播消息
func (r *Cluster) Broadcast(service string, message *iface.ActorMessage) {
	members := r.discovery.GetAll()
	if len(members) == 0 {
		return
	}
	// 向所有节点发送消息
	for _, member := range members {
		if !slices.Contains(member.Tags, service) {
			continue
		}

		rpcMessage := convertor.DeepClone(message)
		rpcMessage.To = &iface.Pid{
			NodeId: member.GetID(),
			Name:   service,
		}
		// 发送消息
		if err := r.Send(rpcMessage); err != nil {
			glog.Error("集群通信: 广播消息到节点失败", zap.Uint64("nodeId", member.GetID()), zap.String("kind", service), zap.Error(err))
		}
	}
	return
}

// Shutdown 关闭所有订阅
func (r *Cluster) Shutdown(ctx context.Context) error {
	if err := r.discovery.Shutdown(ctx); err != nil {
		return err
	}
	if err := r.messageQue.Shutdown(ctx); err != nil {
		return err
	}
	return nil
}
