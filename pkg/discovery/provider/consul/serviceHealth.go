package consul

import (
	"context"
	"errors"
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

func newHealthKeeper(ctx context.Context, wg *sync.WaitGroup,
	client *api.Client, config *Config, member *iface.Member) *healthKeeper {

	clonedMember := convertor.DeepClone(member)
	if clonedMember == nil {
		glog.Error("DeepClone member failed", zap.Uint64("memberId", member.GetID()))
		clonedMember = member
	}
	keeper := &healthKeeper{
		wg:      wg,
		client:  client,
		config:  config,
		Member:  clonedMember,
		checkId: fmt.Sprintf("service:%d", member.GetID()),
	}
	keeper.ctx, keeper.cancel = context.WithCancel(ctx)
	return keeper
}

type healthKeeper struct {
	stopper.Stopper
	*iface.Member

	client *api.Client
	config *Config

	wg     *sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc

	checkId string
	once    sync.Once
}

func (keeper *healthKeeper) Register() error {
	var err error
	keeper.once.Do(func() {
		if err = keeper.register(); err != nil {
			return
		}

		keeper.wg.Add(1)
		grs.Go(func(ctx context.Context) {
			keeper.updateTTLLoop()
			keeper.wg.Done()
		})
	})
	return err
}

func (keeper *healthKeeper) Update(member *iface.Member) error {
	keeper.Member = member
	return keeper.register()
}

func (keeper *healthKeeper) register() error {
	config := keeper.config
	registration := &api.AgentServiceRegistration{
		ID:      convertor.ToString(keeper.GetID()),
		Name:    keeper.GetKind(),
		Address: keeper.GetAddress(),
		Port:    keeper.GetPort(),
		Tags:    keeper.GetTags(),
		Meta:    keeper.GetMeta(),
		Check: &api.AgentServiceCheck{
			CheckID:                        keeper.checkId,
			TTL:                            config.HealthTTL.String(),
			DeregisterCriticalServiceAfter: config.DeregisterInterval.String(),
		},
	}
	return keeper.client.Agent().ServiceRegister(registration)
}

func (keeper *healthKeeper) updateTTLLoop() {
	ticker := time.NewTicker(keeper.config.HealthTTL / 2)
	defer func() {
		ticker.Stop()
		_ = keeper.shutdown()
	}()
	keeper.updateTTL()
	for !keeper.IsStop() {
		select {
		case <-keeper.ctx.Done():
			return
		case <-ticker.C:
			keeper.updateTTL()
		}
	}
}

func (keeper *healthKeeper) updateTTL() {
	options := &api.QueryOptions{}
	options = options.WithContext(keeper.ctx)
	if err := keeper.client.Agent().UpdateTTLOpts(keeper.checkId, "", "pass", options); err != nil {
		if !errors.Is(err, context.Canceled) {
			glog.Error("定时更新TTL失败", zap.Uint64("memberId", keeper.GetID()), zap.Error(err))
		}
	}
}

func (keeper *healthKeeper) deregister() error {
	err := keeper.client.Agent().ServiceDeregister(convertor.ToString(keeper.GetID()))
	if err != nil {
		glog.Error("注销服务失败", zap.Uint64("memberId", keeper.GetID()), zap.Error(err))
		return err
	}
	return nil
}

func (keeper *healthKeeper) shutdown() error {
	if !keeper.Stop() {
		return nil // 已经被其他 goroutine 停止
	}
	keeper.cancel()
	return keeper.deregister()
}
