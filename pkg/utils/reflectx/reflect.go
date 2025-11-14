/**
 * @Author: dingQingHui
 * @Description:
 * @File: reflect
 * @Version: 1.0.0
 * @Date: 2024/11/28 15:07
 */

package reflectx

import (
	"reflect"
	"unicode"
	"unicode/utf8"
)

func TypeFullName(v interface{}) string {
	t := reflect.TypeOf(v)
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t.PkgPath() + ":" + t.Name()
}

func IsExportedType(t reflect.Type) bool {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	name := t.Name()
	rune, _ := utf8.DecodeRuneInString(name)
	return unicode.IsUpper(rune) || t.PkgPath() == ""
}

func NewByType(t reflect.Type) interface{} {
	var argv reflect.Value
	if t.Kind() == reflect.Ptr {
		argv = reflect.New(t.Elem())
		return argv.Interface()
	} else {
		argv = reflect.New(t)
		return argv.Elem().Interface()
	}
}

func SuitableMethods(rec interface{}) map[string]reflect.Method {
	typ := reflect.TypeOf(rec)
	methods := make(map[string]reflect.Method)
	for index := 0; index < typ.NumMethod(); index++ {
		fun := typ.Method(index)
		funType := fun.Type
		funName := fun.Name
		if fun.PkgPath != "" {
			continue
		}

		// check extra arg
		isExtraExported := true
		argTypes := make([]reflect.Type, funType.NumIn(), funType.NumIn())
		for i := 0; i < funType.NumIn(); i++ {
			argType := funType.In(i)
			if !IsExportedType(argType) {
				isExtraExported = false
				break
			}
			argTypes[i] = argType
		}

		if !isExtraExported {
			continue
		}
		methods[funName] = fun
	}
	return methods
}
