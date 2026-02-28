package actor

import (
	"time"

	"github.com/dzm2020/gas/internal/iface"
	"github.com/dzm2020/gas/pkg/cluster"
	"github.com/dzm2020/gas/pkg/lib"
)

type ClusterSystem struct {
	selfNodeID string // 本节点 ID
	transport  cluster.ICluster
}

func (s *ClusterSystem) clusterNamed(name string) error {
	info := s.node.Info()
	info.Tags = append(info.Tags, name)
	return s.transport.Update(info)
}

func (s *ClusterSystem) clusterUnname(name string) error {
	cluster := s.node.Cluster()
	if cluster == nil {
		return ErrClusterIsNil
	}
	info := s.node.Info()

	info.Tags = slices.DeleteFunc(info.Tags, func(s string) bool {
		return s == name
	})

	return cluster.Update(info)
}

// SubmitTask 提交异步任务到指定进程
func (s *ClusterSystem) SubmitTask(to *iface.Pid, task iface.Task) error {
	msg := iface.NewTaskMessage(task)
	return s.sendToProcess(to, msg)
}

// SubmitTaskAndWait 提交同步任务到指定进程，等待执行完成
func (s *ClusterSystem) SubmitTaskAndWait(to *iface.Pid, task iface.Task, timeout time.Duration) (err error) {
	waiter := lib.NewChanWaiter[[]byte](timeout)

	syncTask := func(ctx iface.IContext) error {
		taskErr := task(ctx)
		waiter.Done(nil, taskErr)
		return taskErr
	}

	msg := iface.NewTaskMessage(syncTask)
	if err = s.sendToProcess(to, msg); err != nil {
		waiter.Done(nil, err)
		return err
	}

	_, err = waiter.Wait()
	return
}
