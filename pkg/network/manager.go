package network

import (
	"sync"
	"sync/atomic"
)

var (
	dict  sync.Map // key: sessionID, value: *Connection
	count atomic.Int32
)

func Add(e IEntity) {
	dict.Store(e.ID(), e)
	count.Add(1)
}

func Remove(id uint64) {
	dict.Delete(id)
	count.Add(-1)
}

func Get(id uint64) IEntity {
	val, ok := dict.Load(id)
	if !ok {
		return nil
	}
	return val.(IEntity)
}

func Count() int32 {
	return count.Load()
}
