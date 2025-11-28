/**
 * @Author: dingQingHui
 * @Description:
 * @File: func
 * @Version: 1.0.0
 * @Date: 2024/11/26 14:06
 */

package lib

import (
	"fmt"
	"os"
	"runtime/debug"
	"time"
)

func Assert(err error) {
	if err != nil {
		panic(err)
	}
}

func NilAssert(v interface{}) {
	if v == nil {
		panic("object is nil")
	}
}

func PrintCoreDump() {
	err := recover()
	if err == nil {
		return
	}
	sDate := time.Now().Format("20060102150405")
	fileName := fmt.Sprintf("%s-%d-dump", sDate, os.Getpid())
	file, e := os.Create(fileName)
	if e != nil {
		return
	}
	defer file.Close()
	errs := fmt.Sprintf("%v\n", err)
	_, _ = file.WriteString(errs)
	_, _ = file.WriteString("==================\n")
	stack := string(debug.Stack())
	_, _ = file.WriteString(stack)
}
