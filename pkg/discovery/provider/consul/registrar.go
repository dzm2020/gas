package consul

import (
	"errors"
	"gas/pkg/discovery/iface"
	"sync"

	"github.com/hashicorp/consul/api"
)

var (
	errMemberNotFound = errors.New("member not found")
)

type registrar struct {
	client *api.Client
	config *Config
	mu     sync.RWMutex
	dict   map[uint64]*serviceHealth
}

func newConsulRegistrar(client *api.Client, config *Config) *registrar {
	return &registrar{
		client: client,
		config: config,
		dict:   make(map[uint64]*serviceHealth),
	}
}

func (r *registrar) Register(member *iface.Member) (err error) {
	r.mu.Lock()
	m, ok := r.dict[member.GetID()]
	if !ok {
		m = newServiceHealth(r.client, r.config, member)
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

func (r *registrar) UpdateStatus(memberId uint64, status string) error {
	m := r.get(memberId)
	if m == nil {
		return errMemberNotFound
	}

	return m.updateStatus(status)
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

func (r *registrar) get(memberId uint64) *serviceHealth {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.dict[memberId]
}

// cleanup 清理所有注册的成员
func (r *registrar) cleanup() {
	r.mu.Lock()
	members := make([]*serviceHealth, 0, len(r.dict))
	for _, m := range r.dict {
		members = append(members, m)
	}
	r.dict = make(map[uint64]*serviceHealth)
	r.mu.Unlock()

	// 在锁外执行清理，避免死锁
	for _, m := range members {
		_ = m.deregister()
	}
}
