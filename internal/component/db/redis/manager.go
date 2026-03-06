package redis

import (
	"sync"
)

var (
	// dbs Redis 客户端存储映射：key=数据库 ID，value=*Client
	dbs sync.Map
)

func Get(id int) *Client {
	client, ok := dbs.Load(id)
	if !ok {
		return nil
	}
	return client.(*Client)
}

func Add(id int, conf *Config) error {
	if Has(id) {
		return nil
	}
	client, err := newClient(conf)
	if err != nil {
		return err
	}
	dbs.Store(id, client)
	return nil
}

func Has(id int) bool {
	_, ok := dbs.Load(id)
	return ok
}

// Range 遍历所有客户端并执行回调函数
func Range(fn func(client *Client)) {
	dbs.Range(func(key, value interface{}) bool {
		fn(value.(*Client))
		return true
	})
}

func Close() {
	dbs.Range(func(key, value interface{}) bool {
		_ = value.(*Client).Close()
		dbs.Delete(key)
		return true
	})
}
