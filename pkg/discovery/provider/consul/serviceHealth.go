package consul

import (
	"context"
	"errors"
	"gas/pkg/discovery/iface"
	"gas/pkg/glog"
	"gas/pkg/lib/grs"
	"sync"
	"sync/atomic"
	"time"

	"github.com/duke-git/lancet/v2/convertor"
	"github.com/hashicorp/consul/api"
	"go.uber.org/zap"
)

func newServiceHealth(client *api.Client, config *Config, info *iface.Member) *serviceHealth {
	return &serviceHealth{
		Member:   info,
		client:   client,
		config:   config,
		statusCh: make(chan string, 1024),
		status:   "pass",
	}
}

type serviceHealth struct {
	*iface.Member
	client    *api.Client
	statusCh  chan string
	config    *Config
	status    string
	once      sync.Once
	isRunning atomic.Bool
}

func (m *serviceHealth) Register() error {
	registration := &api.AgentServiceRegistration{
		ID:      convertor.ToString(m.GetID()),
		Name:    m.GetKind(),
		Address: m.GetAddress(),
		Port:    m.GetPort(),
		Tags:    m.GetTags(),
		Meta:    m.GetMeta(),
		Check: &api.AgentServiceCheck{
			TTL:                            m.config.HealthTTL.String(),
			DeregisterCriticalServiceAfter: m.config.DeregisterInterval.String(),
		},
	}
	if err := m.client.Agent().ServiceRegister(registration); err != nil {
		return err
	}
	m.run()
	return nil
}

func (m *serviceHealth) run() {
	// 如果已经运行，直接返回
	if !m.isRunning.CompareAndSwap(false, true) {
		return
	}
	grs.Go(func(ctx context.Context) {
		m.healthLoop(ctx)
	})
}

func (m *serviceHealth) healthLoop(ctx context.Context) {
	ticker := time.NewTicker(m.config.HealthTTL / 2)
	defer ticker.Stop()
	m.updateTTL()
	for {
		select {
		case <-ctx.Done():
			return
		case status, ok := <-m.statusCh:
			if !ok {
				return
			}
			m.status = status
			m.updateTTL()
		case <-ticker.C:
			m.updateTTL()
		}
	}
}

func (m *serviceHealth) updateTTL() {
	memberId := m.GetID()
	checkId := "service:" + convertor.ToString(memberId)
	if err := m.client.Agent().UpdateTTL(checkId, "", m.status); err != nil {
		glog.Error("Consul定时更新TTL失败", zap.String("status", m.status),
			zap.Uint64("memberId", memberId), zap.Error(err))
	}
}

// UpdateStatus 更新成员的健康状态
func (m *serviceHealth) updateStatus(status string) error {
	select {
	case m.statusCh <- status:
	default:
		return errors.New("member status chan is full")
	}
	return nil
}

func (m *serviceHealth) deregister() error {
	m.once.Do(func() {
		close(m.statusCh)

		if err := m.client.Agent().ServiceDeregister(convertor.ToString(m.GetID())); err != nil {
			glog.Error("Consul取消注册节点失败", zap.Uint64("memberId", m.GetID()), zap.Error(err))
		}
	})
	return nil
}
