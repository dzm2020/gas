/**
 * @Author: dingQingHui
 * @Description:
 * @File: timingwheel
 * @Version: 1.0.0
 * @Date: 2024/11/28 14:06
 */

package asynctime

import (
	"github.com/RussellLuo/timingwheel"
	"time"
)

var tw = timingwheel.NewTimingWheel(10*time.Millisecond, 3600)

func init() {
	tw.Start()
}

func AfterFunc(d time.Duration, f func()) *timingwheel.Timer {
	return tw.AfterFunc(d, f)
}
