package lib

import (
	"time"

	"github.com/RussellLuo/timingwheel"
)

var (
	tw = timingwheel.NewTimingWheel(1*time.Millisecond, 3600)
)

type Timer struct {
	*timingwheel.Timer
}

func init() {
	tw.Start()
}

// AfterFunc 注册一次性定时器，时间到后通过 pushTask 通知 baseActorContext 然后执行回调
func AfterFunc(duration time.Duration, callback func()) *Timer {
	t := tw.AfterFunc(duration, func() {
		if callback != nil {
			callback()
		}
	})
	return &Timer{Timer: t}
}

func NowDelay(sec, nsec int64) time.Duration {
	targetTime := time.Unix(sec, nsec)
	return targetTime.Sub(time.Now())
}

func TimeAdd() {
	
}
