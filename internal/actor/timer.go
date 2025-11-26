package actor

import (
	"fmt"
	"gas/internal/iface"
	"gas/pkg/utils/timex/asynctime"
	"time"

	"github.com/RussellLuo/timingwheel"
)

// timerManager 定时器管理器
type timerManager struct {
	timers       map[int64]*timingwheel.Timer // 存储定时器ID和Timer的映射
	tickerTimers map[int64]bool               // 存储周期性定时器ID，用于标记是否为周期性定时器
	timerID      int64                        // 定时器ID生成器
}

// newTimerManager 创建定时器管理器
func newTimerManager() *timerManager {
	return &timerManager{
		timers:       make(map[int64]*timingwheel.Timer),
		tickerTimers: make(map[int64]bool),
	}
}

// AfterFunc 注册一次性定时器，时间到后通过 pushTask 通知 baseActorContext 然后执行回调
func (tm *timerManager) AfterFunc(process iface.IProcess, duration time.Duration, callback iface.Task) (int64, error) {
	if callback == nil {
		return 0, fmt.Errorf("callback is nil")
	}
	if process == nil {
		return 0, fmt.Errorf("process is nil")
	}

	// 生成定时器ID
	tm.timerID++
	timerID := tm.timerID

	// 创建定时器，到期后通过 PushTask 推送回调任务
	timer := asynctime.AfterFunc(duration, func() {
		// 定时器到期后，从映射中移除
		delete(tm.timers, timerID)
		// 通过 PushTask 推送回调任务
		_ = process.PushTask(callback)
	})

	// 存储定时器
	tm.timers[timerID] = timer

	return timerID, nil
}

// TickFunc 注册周期性定时器，每隔指定时间间隔执行一次回调
// interval: 执行间隔时间
// callback: 每次执行的回调函数
// 返回定时器ID，可用于取消定时器
func (tm *timerManager) TickFunc(process iface.IProcess, interval time.Duration, callback iface.Task) (int64, error) {
	if callback == nil {
		return 0, fmt.Errorf("callback is nil")
	}
	if process == nil {
		return 0, fmt.Errorf("process is nil")
	}

	// 生成定时器ID
	tm.timerID++
	timerID := tm.timerID

	// 标记为周期性定时器
	tm.tickerTimers[timerID] = true

	// 内部函数：注册下一个周期的定时器
	var scheduleNext func()
	scheduleNext = func() {
		// 检查定时器是否还存在（可能已被取消）
		if _, exists := tm.tickerTimers[timerID]; !exists {
			return
		}

		// 创建下一个周期的定时器
		timer := asynctime.AfterFunc(interval, func() {
			// 检查定时器是否还存在
			if _, exists := tm.tickerTimers[timerID]; !exists {
				return
			}

			// 通过 PushTask 推送回调任务
			_ = process.PushTask(callback)

			// 调度下一个周期
			scheduleNext()
		})

		// 存储定时器
		tm.timers[timerID] = timer
	}

	// 启动第一个周期
	scheduleNext()

	return timerID, nil
}

// CancelTimer 取消定时器
func (tm *timerManager) CancelTimer(timerID int64) bool {
	// 如果是周期性定时器，先从标记中删除
	delete(tm.tickerTimers, timerID)

	if timer, ok := tm.timers[timerID]; ok {
		timer.Stop()
		delete(tm.timers, timerID)
		return true
	}
	return false
}

// CancelAllTimers 取消所有定时器
func (tm *timerManager) CancelAllTimers() {
	// 清空周期性定时器标记
	tm.tickerTimers = make(map[int64]bool)

	for timerID, timer := range tm.timers {
		timer.Stop()
		delete(tm.timers, timerID)
	}
}

