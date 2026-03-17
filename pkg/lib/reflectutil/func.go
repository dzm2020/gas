package reflectutil

import "reflect"

// NewInstance 通过反射创建传入对象的新实例，返回该对象的指针
// 参数：任意类型的对象（值/指针都支持）
// 返回值：新对象的指针（interface{} 类型，可断言为原类型指针）
func NewInstance(obj interface{}) interface{} {
	// 1. 获取传入对象的反射类型
	objType := reflect.TypeOf(obj)

	// 2. 兼容处理：如果传入的是指针类型，获取指针指向的底层类型
	if objType.Kind() == reflect.Ptr {
		objType = objType.Elem()
	}

	// 3. 反射创建该类型的指针（等价于 Go 原生 new(类型)）
	newPtrValue := reflect.New(objType)

	// 4. 将反射值转换为空接口，返回指针
	return newPtrValue.Interface()
}
