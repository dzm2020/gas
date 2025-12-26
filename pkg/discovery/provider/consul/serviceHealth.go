package consul

import (
	"context"
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

func newHealthKeeper(ctx context.Context, client *api.Client, config *Config, member *iface.Member) *healthKeeper {
	return &healthKeeper{
		client: client,
		config: config,
		member: member,
		ctx:    ctx,
	}
}

type healthKeeper struct {
	client    *api.Client
	config    *Config
	ctx       context.Context
	status    string
	member    *iface.Member
	once      sync.Once
	isRunning atomic.Bool
}

func (m *healthKeeper) Register() error {
	registration := &api.AgentServiceRegistration{
		ID:      convertor.ToString(m.member.GetID()),
		Name:    m.member.GetKind(),
		Address: m.member.GetAddress(),
		Port:    m.member.GetPort(),
		Tags:    m.member.GetTags(),
		Meta:    m.member.GetMeta(),
		Check: &api.AgentServiceCheck{
			TTL:                            m.config.HealthTTL.String(),
			DeregisterCriticalServiceAfter: m.config.DeregisterInterval.String(),
		},
	}
	if err := m.client.Agent().ServiceRegister(registration); err != nil {
		return err
	}
	m.runUpdateTTLLoop()
	return nil
}

func (m *healthKeeper) runUpdateTTLLoop() {
	// 如果已经运行，直接返回
	if !m.isRunning.CompareAndSwap(false, true) {
		return
	}
	grs.Go(func(ctx context.Context) {
		m.updateTTLLoop()
	})
}

func (m *healthKeeper) updateTTLLoop() {
	defer func() {
		_ = m.deregister()
	}()
	ticker := time.NewTicker(m.config.HealthTTL / 2)
	defer ticker.Stop()
	m.updateTTL()
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.updateTTL()
		}
	}
}

func (m *healthKeeper) updateTTL() {
	memberId := m.member.GetID()
	checkId := "service:" + convertor.ToString(memberId)
	if err := m.client.Agent().UpdateTTL(checkId, "", m.status); err != nil {
		glog.Error("Consul定时更新TTL失败", zap.String("status", "pass"),
			zap.Uint64("memberId", memberId), zap.Error(err))
	}
}

func (m *healthKeeper) deregister() (err error) {
	m.once.Do(func() {
		memberId := m.member.GetID()
		err = m.client.Agent().ServiceDeregister(convertor.ToString(memberId))
	})
	return
}
