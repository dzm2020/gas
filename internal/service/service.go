package service

import (
	"fmt"
	"gas/internal/actor"
	"gas/internal/iface"

	"github.com/duke-git/lancet/v2/maputil"
)

type Service struct {
	Name         string
	ActorFactory func() iface.IActor
	Options      []iface.Option
	actorSystem  *actor.System
	actors       *maputil.ConcurrentMap[uint64, *iface.Pid] // serviceID -> actor PID
}

// getOrCreateActor 获取或创建actor
func (s *Service) getOrCreateActor(serviceID uint64) (*iface.Pid, error) {
	existingPid, exists := s.actors.Get(serviceID)
	if exists {
		return existingPid, nil
	}
	// 需要复制actor模板
	actorInstance := s.ActorFactory()
	if actorInstance == nil {
		return nil, fmt.Errorf("failed to clone actor template")
	}
	pid, process := s.actorSystem.Spawn(actorInstance, s.Options...)
	if process == nil {
		return nil, fmt.Errorf("spawn actor failed for serviceID=%d", serviceID)
	}
	s.actors.Set(serviceID, pid)
	return pid, nil
}
