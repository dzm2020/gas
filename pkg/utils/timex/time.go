/**
 * @Author: dingQingHui
 * @Description:
 * @File: time
 * @Version: 1.0.0
 * @Date: 2024/12/11 16:51
 */

package timex

import "time"

func Now() int64 {
	return time.Now().Unix()
}
