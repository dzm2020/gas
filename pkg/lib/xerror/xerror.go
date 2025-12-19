package xerror

import (
	"fmt"
	"os"
	"runtime/debug"
	"time"
)

// Wrap 包装错误，添加上下文信息
// 如果 err 为 nil，返回 nil
// 格式: message: err
func Wrap(err error, message string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", message, err)
}

// Wrapf 包装错误，使用格式化字符串添加上下文信息
// 如果 err 为 nil，返回 nil
// 格式: formatted message: err
func Wrapf(err error, format string, args ...interface{}) error {
	if err == nil {
		return nil
	}
	message := fmt.Sprintf(format, args...)
	return fmt.Errorf("%s: %w", message, err)
}

func Assert(err error) {
	if err != nil {
		panic(err)
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
