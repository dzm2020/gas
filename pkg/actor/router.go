package actor

import (
	"gas/pkg/utils/reflectx"
	"reflect"
)

func NewRouter(actor IActor) *Router {
	r := new(Router)
	r.dict = reflectx.SuitableMethods(actor)
	return r
}

type Router struct {
	dict map[string]reflect.Method
}

func (r *Router) Handle(name string, args ...interface{}) []interface{} {
	method := r.dict[name]
	argValues := make([]reflect.Value, len(args))
	for i, arg := range args {
		argValues[i] = reflect.ValueOf(arg)
	}

	// 调用方法（传入参数）
	returnValues := method.Func.Call(argValues)

	// 转换返回值为 interface{} 类型
	result := make([]interface{}, len(returnValues))
	for i, ret := range returnValues {
		result[i] = ret.Interface()
	}
	return result
}
