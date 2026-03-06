package redis

import (
	"context"
	"testing"
	"time"

	"github.com/duke-git/lancet/v2/retry"
	"github.com/go-redis/redis/v8"
)

func Init(t *testing.T) {
	// 测试配置
	config := &Config{
		Address:  []string{"127.0.0.1:6379"},
		Password: "",
		PoolSize: 10,
	}

	// 测试 Add
	err := Add(1, config)
	if err != nil {
		t.Skipf("跳过测试：无法连接到 Redis: %v", err)
		return
	}

}

func TestRedis(t *testing.T) {
	// 清理测试环境
	defer Close()
	Init(t)

	// 测试 Has
	if !Has(1) {
		t.Error("Has(1) 应该返回 true")
	}
	if Has(999) {
		t.Error("Has(999) 应该返回 false")
	}

	// 测试 Get
	client := Get(1)
	if client == nil {
		t.Fatal("Get(1) 应该返回非 nil 客户端")
	}
	if Get(999) != nil {
		t.Error("Get(999) 应该返回 nil")
	}

	// 测试 Range
	count := 0
	Range(func(c *Client) {
		count++
	})
	if count == 0 {
		t.Error("Range 应该至少遍历到一个客户端")
	}

	// 测试 IsNil 和 Error
	err := client.Get(context.Background(), "not:exist:key").Err()
	if !IsNil(err) {
		t.Error("IsNil 应该返回 true 当键不存在时")
	}
	if Error(err) {
		t.Error("Error(redis.Nil) 应该返回 false")
	}

	err = client.Set(context.Background(), "test:key", "test:value", 0).Err()
	if err != nil {
		t.Fatalf("Set 失败: %v", err)
	}
	err = client.Get(context.Background(), "test:key").Err()
	if IsNil(err) {
		t.Error("IsNil 应该返回 false 当键存在")
	}

	if Error(nil) {
		t.Error("Error(nil) 应该返回 false")
	}

	err = redis.ErrClosed
	if !Error(err) {
		t.Error("Error(redis.ErrClosed) 应该返回 true")
	}

}

func TestScript(t *testing.T) {
	// 清理测试环境
	defer Close()
	Init(t)

	client := Get(1)

	// 测试 RegisterScript 和 RunScript
	testScript := "return ARGV[1]"
	RegisterScript("TestScript", testScript)

	result, err := RunScript(1, "TestScript", []string{}, "hello")
	if err != nil {
		t.Errorf("RunScript 执行失败: %v", err)
	}
	if result != "hello" {
		t.Errorf("RunScript 返回结果错误，期望 'hello'，得到 %v", result)
	}

	_, err = RunScript(1, "NotExistScript", []string{})
	if err == nil {
		t.Error("RunScript 应该返回错误当脚本未注册")
	}

	_, err = RunScript(999, "TestScript", []string{})
	if err == nil {
		t.Error("RunScript 应该返回错误当客户端不存在时")
	}

	// 测试 Lock 和 Unlock
	key := "lockKey"
	val := "lockValue"
	expire := 100

	err = Lock(1, key, val, expire)
	if err != nil {
		t.Errorf("Lock 失败: %v", err)
	}

	err = Lock(1, key, "other:value", expire)
	if err == nil {
		t.Error("Lock 应该返回错误当锁已被占用")
	}

	err = Unlock(1, key, val)
	if err != nil {
		t.Errorf("Unlock 失败: %v", err)
	}

	err = Unlock(1, key, "wrong:value")
	if err == nil {
		t.Error("Unlock 应该返回错误当使用错误的值")
	}

	// 测试 LockWithRetry
	err = LockWithRetry(1, key, val, expire,
		retry.RetryTimes(3),
		retry.RetryWithLinearBackoff(time.Millisecond*10),
	)
	if err != nil {
		t.Errorf("LockWithRetry 失败: %v", err)
	}
	_ = Unlock(1, key, val)

	// 清理
	_ = client.Del(context.Background(), "test:key").Err()
}
