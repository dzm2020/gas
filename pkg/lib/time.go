/**
 * @Author: dingQingHui
 * @Description:
 * @File: timingwheel
 * @Version: 1.0.0
 * @Date: 2024/11/28 14:06
 */

package lib

import (
	"sync"
	"time"

	"github.com/RussellLuo/timingwheel"
)

var (
	tw           = timingwheel.NewTimingWheel(1*time.Millisecond, 3600)
	timerID      int64 // 定时器ID生成器
	timers       = make(map[int64]*timingwheel.Timer)
	tickerTimers = make(map[int64]bool) // 存储周期性定时器ID，用于标记是否为周期性定时器
	timerLock    sync.RWMutex
)

func init() {
	tw.Start()
}

func nextTimerID() int64 {
	timerID++
	return timerID
}

// AfterFunc 注册一次性定时器，时间到后通过 pushTask 通知 baseActorContext 然后执行回调
func AfterFunc(duration time.Duration, callback func()) *Timer {
	id := nextTimerID()
	timer := tw.AfterFunc(duration, func() {
		if callback != nil {
			callback()
		}
		delete(timers, id)
	})

	timerLock.Lock()
	defer timerLock.Unlock()
	timers[id] = timer
	return &Timer{
		Id:    id,
		Timer: timer,
	}
}

// TickFunc 注册周期性定时器，每隔指定时间间隔执行一次回调
// interval: 执行间隔时间
// callback: 每次执行的回调函数
// 返回定时器ID，可用于取消定时器
func TickFunc(interval time.Duration, callback func()) *Timer {
	var timer *timingwheel.Timer
	id := nextTimerID()

	timerLock.Lock()
	tickerTimers[id] = true
	timerLock.Unlock()

	// 内部函数：注册下一个周期的定时器
	var scheduleNext func()
	scheduleNext = func() {

		timerLock.RLock()
		if _, exists := tickerTimers[id]; !exists {
			timerLock.RUnlock()
			return
		}
		timerLock.RUnlock()

		timer = tw.AfterFunc(interval, func() {
			if _, exists := tickerTimers[id]; !exists {
				return
			}
			if callback != nil {
				callback()
			}
			scheduleNext()
		})

		timerLock.Lock()
		timers[id] = timer
		timerLock.Unlock()

	}

	scheduleNext()
	return &Timer{
		Id:    id,
		Timer: timer,
	}
}

type Timer struct {
	Id int64
	*timingwheel.Timer
}

func (t *Timer) Stop() bool {

	timerLock.Lock()
	delete(timers, t.Id)
	delete(tickerTimers, t.Id)
	timerLock.Unlock()

	return t.Timer.Stop()
}
