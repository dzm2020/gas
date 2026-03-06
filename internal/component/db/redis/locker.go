package redis

import (
	"errors"

	"github.com/duke-git/lancet/v2/retry"
)

var (
	lockLua = `
if redis.call('SETNX', KEYS[1], ARGV[1]) == 1 then
    redis.call('EXPIRE', KEYS[1], ARGV[2])
    return 1
else
    return 0
end
`

	unlockLua = `
if redis.call('GET', KEYS[1]) == ARGV[1] then
    redis.call('DEL', KEYS[1])
    return 1
else
    return 0
end
`
)

func init() {
	RegisterScript("LockScript", lockLua)
	RegisterScript("UnlockScript", unlockLua)
}

func Lock(rid int, key, val string, expire int) error {
	return lock(rid, key, val, expire)
}

func LockWithRetry(rid int, key, val string, expire int, option ...retry.Option) error {
	return retry.Retry(func() error {
		return Lock(rid, key, val, expire)
	}, option...)
}

func lock(rid int, key, val string, expire int) error {
	res, err := RunScript(rid, "LockScript", []string{key}, val, expire)
	if err != nil {
		return err
	}

	// 转换结果
	locked, ok := res.(int64)
	if !ok {
		return errors.New("脚本返回结果类型错误")
	}

	if locked != 1 {
		return errors.New("已加锁")
	}
	return nil
}

func Unlock(rid int, key, val string) error {
	res, err := RunScript(rid, "UnlockScript", []string{key}, val)
	if err != nil {
		return err
	}
	// 转换结果
	unlocked, ok := res.(int64)
	if !ok {
		return errors.New("脚本返回结果类型错误")
	}
	if unlocked != 1 {
		return errors.New("解锁失败")
	}
	return nil
}
