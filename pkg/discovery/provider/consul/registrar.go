package consul

import (
	"gas/pkg/discovery/iface"
	"gas/pkg/utils/glog"
	"sync"
	"time"

	"github.com/duke-git/lancet/v2/convertor"
	"github.com/hashicorp/consul/api"
	"go.uber.org/zap"
)

type consulRegistrar struct {
	client *api.Client
	opts   *Options
	stopCh <-chan struct{}

	wg       sync.WaitGroup
	healthMu sync.RWMutex
	healthCh map[uint64]chan string
}

func newConsulRegistrar(client *api.Client, opts *Options, stopCh <-chan struct{}) *consulRegistrar {
	return &consulRegistrar{
		client:   client,
		opts:     opts,
		stopCh:   stopCh,
		healthCh: make(map[uint64]chan string),
	}
}

func (r *consulRegistrar) Add(node *iface.Node) error {
	check := &api.AgentServiceCheck{
		TTL:                            r.opts.HealthTTL.String(),
		DeregisterCriticalServiceAfter: r.opts.DeregisterInterval.String(),
	}
	registration := &api.AgentServiceRegistration{
		ID:      convertor.ToString(node.GetID()),
		Name:    node.GetName(),
		Address: node.GetAddress(),
		Port:    node.GetPort(),
		Tags:    node.GetTags(),
		Meta:    node.GetMeta(),
		Check:   check,
	}
	if err := r.client.Agent().ServiceRegister(registration); err != nil {
		return err
	}

	ch := make(chan string, 1)
	if !r.setHealthChan(node.GetID(), ch) {
		//  update service
		return nil
	}

	r.wg.Add(1)
	go r.healthCheck(node.GetID(), ch)
	return nil
}

func (r *consulRegistrar) UpdateStatus(nodeID uint64, status string) {
	ch := r.getHealthChan(nodeID)
	if ch == nil {
		return
	}
	select {
	case ch <- status:
	default:
	}
}

func (r *consulRegistrar) Remove(nodeID uint64) error {
	r.deleteHealthChan(nodeID)
	return r.client.Agent().ServiceDeregister(convertor.ToString(nodeID))
}

func (r *consulRegistrar) healthCheck(nodeID uint64, ch <-chan string) {
	defer func() {
		if rec := recover(); rec != nil {
			glog.Error("consul registrar healthCheck panic", zap.Uint64("nodeID", nodeID), zap.Any("error", rec))
		}
		r.wg.Done()
	}()
	interval := r.opts.HealthTTL / 2
	if interval <= 0 {
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	defer func() {
		_ = r.Remove(nodeID)
	}()

	_ = r.updateTTL(nodeID, "pass")

	for {
		select {
		case <-r.stopCh:
			return
		case status, ok := <-ch:
			if !ok {
				return
			}
			if err := r.updateTTL(nodeID, status); err != nil {
				return
			}
		case <-ticker.C:
			if err := r.updateTTL(nodeID, "pass"); err != nil {
				return
			}
		}
	}
}

func (r *consulRegistrar) updateTTL(nodeID uint64, status string) error {
	return r.client.Agent().UpdateTTL("service:"+convertor.ToString(nodeID), "", status)
}

func (r *consulRegistrar) setHealthChan(nodeID uint64, ch chan string) bool {
	r.healthMu.Lock()
	if _, ok := r.healthCh[nodeID]; ok {
		return false
	}
	r.healthCh[nodeID] = ch
	r.healthMu.Unlock()
	return true
}

func (r *consulRegistrar) getHealthChan(nodeID uint64) chan string {
	r.healthMu.RLock()
	defer r.healthMu.RUnlock()
	return r.healthCh[nodeID]
}

func (r *consulRegistrar) deleteHealthChan(nodeID uint64) {
	r.healthMu.Lock()
	if ch, ok := r.healthCh[nodeID]; ok {
		close(ch)
		delete(r.healthCh, nodeID)
	}
	r.healthMu.Unlock()
}

func (r *consulRegistrar) shutdown() {
	r.healthMu.Lock()
	for id, ch := range r.healthCh {
		close(ch)
		delete(r.healthCh, id)
	}
	r.healthMu.Unlock()
}

func (r *consulRegistrar) Wait() {
	r.wg.Wait()
}
