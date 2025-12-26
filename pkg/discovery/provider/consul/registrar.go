package consul

import (
	"context"
	"gas/pkg/discovery/iface"
	"sync"

	"github.com/hashicorp/consul/api"
)

type registrar struct {
	ctx    context.Context
	client *api.Client
	config *Config
	mu     sync.RWMutex
	dict   map[uint64]*healthKeeper
}

func newRegistrar(ctx context.Context, client *api.Client, config *Config) *registrar {
	return &registrar{
		client: client,
		config: config,
		ctx:    ctx,
		dict:   make(map[uint64]*healthKeeper),
	}
}

func (r *registrar) Register(member *iface.Member) (err error) {
	r.mu.Lock()
	m, ok := r.dict[member.GetID()]
	if !ok {
		m = newHealthKeeper(r.ctx, r.client, r.config, member)
		r.dict[member.GetID()] = m
	}
	r.mu.Unlock()

	if err = m.Register(); err != nil {
		// 注册失败，从字典中移除，避免资源泄漏
		r.mu.Lock()
		if existing, exists := r.dict[member.GetID()]; exists && existing == m {
			delete(r.dict, member.GetID())
		}
		r.mu.Unlock()
		return err
	}
	return nil
}

func (r *registrar) Deregister(memberId uint64) error {
	m := r.get(memberId)
	if m == nil {
		return nil
	}

	r.mu.Lock()
	delete(r.dict, memberId)
	r.mu.Unlock()

	return m.deregister()
}

func (r *registrar) get(memberId uint64) *healthKeeper {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.dict[memberId]
}
