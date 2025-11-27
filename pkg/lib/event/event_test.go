/**
 * @Author: dingQingHui
 * @Description:
 * @File: event_test
 * @Version: 1.0.0
 * @Date: 2023/11/13 10:33
 */

package event

import (
	"fmt"
	"testing"
)

func TestName(t *testing.T) {
	listener := NewListener[string]()

	listener.Register(func(i string) {
		fmt.Printf("event:1 value:%v\n", i)
	})
	handler1 := func(i string) {
		fmt.Printf("handler1 event:1 value:%v\n", i)
	}
	listener.Register(handler1)

	listener.Register(func(i string) {
		fmt.Printf("event:1 value:%v\n", i)
	})

	listener.Notify("111111111111111111")
	listener.Notify("2222222222222222")
	listener.UnRegister(handler1)
	listener.Notify("333333333333")
	fmt.Println(111111111111111)

	listener1 := NewListener[int]()
	listener1.Register(func(i int) {
		fmt.Printf("event:1 value:%v\n", i)
	})
	listener1.Notify(333333333333)
}
