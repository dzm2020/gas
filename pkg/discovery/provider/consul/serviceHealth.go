package consul

import (
	"context"
	"fmt"

	"github.com/dzm2020/gas/pkg/discovery/iface"
	"github.com/dzm2020/gas/pkg/lib/stopper"

	"sync"
	"time"

	"github.com/dzm2020/gas/pkg/glog"
	"github.com/dzm2020/gas/pkg/lib/grs"

	"github.com/duke-git/lancet/v2/convertor"
	"github.com/hashicorp/consul/api"
	"go.uber.org/zap"
)

func newHealthKeeper(ctx context.Context, wg *sync.WaitGroup, client *api.Client,
	config *Config, member *iface.Member) *healthKeeper {
	h := &healthKeeper{
		wg:      wg,
		client:  client,
		config:  config,
		member:  convertor.DeepClone(member),
		checkId: fmt.Sprintf("service:%d", member.GetID()),
	}
	h.ctx, h.cancel = context.WithCancel(ctx)
	return h
}

type healthKeeper struct {
	stopper.Stopper

	client *api.Client
	config *Config

	wg *sync.WaitGroup

	checkId     string
	member      *iface.Member
	onceChecker sync.Once
	ctx         context.Context
	cancel      context.CancelFunc
}

func (m *healthKeeper) Register() error {
	config := m.config
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
	if err := m.client.Agent().ServiceRegister(registration); err != nil {
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
	ticker := time.NewTicker(m.config.HealthTTL / 2)
	defer func() {
		ticker.Stop()
		_ = m.deregister()
	}()
	m.updateTTL()
	for !m.IsStop() {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.updateTTL()
		}
	}
}

func (m *healthKeeper) updateTTL() {
	options := &api.QueryOptions{}
	options = options.WithContext(m.ctx)
	if err := m.client.Agent().UpdateTTLOpts(m.checkId, "", "pass", options); err != nil {
		if m.IsStop() {
			return
		}
		glog.Error("定时更新TTL失败", zap.Uint64("memberId", m.member.GetID()), zap.Error(err))
	}
}

func (m *healthKeeper) deregister() (err error) {
	if m.Stop() {
		return
	}

	memberId := m.member.GetID()
	m.cancel()
	err = m.client.Agent().ServiceDeregister(convertor.ToString(memberId))
	glog.Info("注销服务", zap.Uint64("memberId", memberId))
	return err
}
