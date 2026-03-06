package redis

import (
	"fmt"
	"sync"

	"github.com/go-redis/redis/v8"
)

var (
	// scripts 脚本映射：key=脚本名称，value=Script 实例
	scripts = make(map[string]*redis.Script)
	// scriptsMu 脚本映射的并发安全锁
	scriptsMu sync.RWMutex
)

// RegisterScript 注册新的 Lua 脚本（新版本脚本系统）
// name: 脚本名称
// lua: Lua 脚本内容
func RegisterScript(name string, lua string) {
	scriptsMu.Lock()
	defer scriptsMu.Unlock()
	scripts[name] = redis.NewScript(lua)
}

// RunScript 执行已注册的脚本（新版本脚本系统）
// 自动处理 EvalSha 回退：优先使用 SHA1 哈希，失败时自动回退到完整脚本
func RunScript(rid int, name string, keys []string, args ...interface{}) (interface{}, error) {
	scriptsMu.RLock()
	script, ok := scripts[name]
	scriptsMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("脚本[%s]未注册", name)
	}

	client := Get(rid)

	if client == nil {
		return nil, fmt.Errorf("客户端不存在 idx[%d]", rid)
	}

	// 调用原生 Run 方法（自动处理 EvalSha 回退）
	return script.Run(client.Context(), client, keys, args...).Result()
}

// loadAllScripts 预加载所有脚本到 Redis（可选，提升首次执行性能）
func loadAllScripts(client *Client) error {
	scriptsMu.RLock()
	defer scriptsMu.RUnlock()

	for name, script := range scripts {
		_, err := script.Load(client.Context(), client).Result()
		if err != nil {
			return fmt.Errorf("加载脚本[%s]失败: %w", name, err)
		}
	}
	return nil
}
