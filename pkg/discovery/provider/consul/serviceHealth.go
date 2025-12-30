package consul

import (
	"context"
	"fmt"
	"gas/pkg/discovery/iface"
	"sync"
	"sync/atomic"

	"gas/pkg/glog"
	"gas/pkg/lib/grs"
	"time"

	"github.com/duke-git/lancet/v2/convertor"
	"github.com/hashicorp/consul/api"
	"go.uber.org/zap"
)

func newHealthKeeper(provider *Provider, member *iface.Member) *healthKeeper {
	h := &healthKeeper{
		provider: provider,
		member:   convertor.DeepClone(member),
		checkId:  fmt.Sprintf("service:%d", member.GetID()),
	}
	h.ctx, h.cancel = context.WithCancel(provider.ctx)
	return h
}

type healthKeeper struct {
	provider    *Provider
	isStop      atomic.Bool
	checkId     string
	member      *iface.Member
	once        sync.Once
	onceChecker sync.Once
	ctx         context.Context
	cancel      context.CancelFunc
}

func (m *healthKeeper) Register() error {

	config := m.provider.config
	registration := &api.AgentServiceRegistration{
		ID:      convertor.ToString(m.member.GetID()),
		Name:    m.member.GetKind(),
		Address: m.member.GetAddress(),
		Port:    m.member.GetPort(),
		Tags:    m.member.GetTags(),
		Meta:    m.member.GetMeta(),
		Check: &api.AgentServiceCheck{
			CheckID:                        m.checkId,
			TTL:                            config.HealthTTL.String(),
			DeregisterCriticalServiceAfter: config.DeregisterInterval.String(),
		},
	}
	if err := m.provider.Agent().ServiceRegister(registration); err != nil {
		return err
	}

	glog.Info("注册服务", zap.String("service", registration.ID))

	m.onceChecker.Do(func() {
		grs.Go(func(ctx context.Context) {
			glog.Debug("成员健康检测协程运行", zap.Uint64("memberId", m.member.GetID()))
			m.updateTTLLoop()
			_ = m.deregister()
			glog.Debug("成员健康检测协程退出", zap.Uint64("memberId", m.member.GetID()))
		})
	})

	return nil
}

func (m *healthKeeper) updateTTLLoop() {
	config := m.provider.config
	ctx := m.ctx
	ticker := time.NewTicker(config.HealthTTL / 2)
	defer ticker.Stop()
	m.updateTTL()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.updateTTL()
		}
	}
}

func (m *healthKeeper) updateTTL() {
	if err := m.provider.Agent().UpdateTTL(m.checkId, "", "pass"); err != nil {
		if m.isStop.Load() {
			return
		}
		glog.Error("定时更新TTL失败", zap.Uint64("memberId", m.member.GetID()), zap.Error(err))
	}
}

func (m *healthKeeper) deregister() (err error) {
	m.once.Do(func() {
		m.isStop.Store(true)
		memberId := m.member.GetID()
		m.cancel()
		err = m.provider.Agent().ServiceDeregister(convertor.ToString(memberId))
		glog.Info("注销服务", zap.Uint64("memberId", memberId))
	})
	return err
}
