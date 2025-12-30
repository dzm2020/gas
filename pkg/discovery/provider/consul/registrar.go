package consul

import (
	"gas/pkg/discovery/iface"
	"sync"
)

type registrar struct {
	provider *Provider
	mu       sync.RWMutex
	dict     map[uint64]*healthKeeper
}

func newRegistrar(provider *Provider) *registrar {
	return &registrar{
		provider: provider,
		dict:     make(map[uint64]*healthKeeper),
	}
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

	m = newHealthKeeper(r.provider, member)
	r.dict[member.GetID()] = m
	return m
}
