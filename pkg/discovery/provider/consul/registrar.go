package consul

import (
	"context"
	"sync"

	"github.com/dzm2020/gas/pkg/discovery/iface"
	"github.com/hashicorp/consul/api"
)

type registrar struct {
	client *api.Client
	config *Config

	wg *sync.WaitGroup

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

func (r *registrar) Register(member *iface.Member) error {
	m := r.get(member.GetID())
	if m == nil {
		m = r.getOrCreate(member)
	}
	return m.Register()
}

func (r *registrar) Deregister(memberId uint64) error {
	m := r.get(memberId)
	if m == nil {
		return nil
	}

	r.delete(memberId)

	return m.deregister()
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

	m, ok := r.dict[member.GetID()]
	if ok {
		return m
	}

	m = newHealthKeeper(r.ctx, r.wg, r.client, r.config, member)
	r.dict[member.GetID()] = m
	return m
}

func (r *registrar) Shutdown() {
	r.cancel()
	r.mu.Lock()
	defer r.mu.Unlock()
	r.dict = make(map[uint64]*healthKeeper)
}
