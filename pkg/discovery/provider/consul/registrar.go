package consul

import (
	"context"
	"errors"
	"sync"

	"github.com/dzm2020/gas/pkg/discovery/iface"
	"github.com/dzm2020/gas/pkg/lib/stopper"
	"github.com/hashicorp/consul/api"
)

type registrar struct {
	stopper.Stopper

	client *api.Client
	config *Config

	wg     *sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc

	mu   sync.RWMutex
	dict map[uint64]*healthKeeper
}

func newRegistrar(ctx context.Context, wg *sync.WaitGroup,
	client *api.Client, config *Config) *registrar {
	r := &registrar{
		wg:     wg,
		client: client,
		config: config,
		dict:   make(map[uint64]*healthKeeper),
	}

	r.ctx, r.cancel = context.WithCancel(ctx)
	return r
}

func (r *registrar) run() {
}

func (r *registrar) Register(member *iface.Member) error {
	keeper := r.get(member.GetID())
	if keeper == nil {
		keeper = r.getOrCreate(member)
	}
	return keeper.Register()
}

func (r *registrar) Update(member *iface.Member) error {
	keeper := r.get(member.GetID())
	if keeper == nil {
		return errors.New("member is not registered")
	}
	return keeper.Update(member)
}

func (r *registrar) Deregister(memberId uint64) error {
	keeper := r.get(memberId)
	if keeper == nil {
		return nil
	}

	r.delete(memberId)

	return keeper.shutdown()
}

func (r *registrar) get(memberId uint64) *healthKeeper {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.dict[memberId]
}

func (r *registrar) delete(memberId uint64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.dict, memberId)
}

func (r *registrar) getOrCreate(member *iface.Member) *healthKeeper {
	r.mu.Lock()
	defer r.mu.Unlock()

	keeper, ok := r.dict[member.GetID()]
	if ok {
		return keeper
	}

	keeper = newHealthKeeper(r.ctx, r.wg, r.client, r.config, member)
	r.dict[member.GetID()] = keeper
	return keeper
}

func (r *registrar) shutdown() {
	if !r.Stop() {
		return
	}
	r.cancel()
	r.mu.Lock()
	defer r.mu.Unlock()
	r.dict = make(map[uint64]*healthKeeper)
}
