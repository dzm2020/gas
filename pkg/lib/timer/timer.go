package timer

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

func AfterFunc(duration time.Duration, callback func()) *Timer {
	t := tw.AfterFunc(duration, func() {
		if callback != nil {
			callback()
		}
	})
	return &Timer{Timer: t}
}

func DeadlineToTimeout(sec, nsec int64) time.Duration {
	targetTime := time.Unix(sec, nsec)
	return targetTime.Sub(time.Now())
}
