package service

import (
	"reflect"
	"sync/atomic"
)

type MethodType struct {
	Method    reflect.Method
	ArgType   reflect.Type
	ReplyType reflect.Type
	callCount uint64
}

func (m *MethodType) GetCallCount() uint64 {
	return atomic.LoadUint64(&m.callCount)
}

func (m *MethodType) NewArgV() reflect.Value {
	var argv reflect.Value
	// reflect.New 函数根据给定的类型（Type）生成一个新的指针，指向该类型的新实例。
	// Elem() 函数是 reflect.Value 类型的一个方法，用于获取一个指针或接口持有的实际元素。
	if m.ArgType.Kind() == reflect.Ptr {
		// 对于指针类型, 就返回指针类型的反射的指针(而不是二级指针)
		argv = reflect.New(m.ArgType.Elem())
	} else {
		// 对于非指针类型, 就返回实例类型的反射的指针
		argv = reflect.New(m.ArgType).Elem()
	}
	return argv
}

func (m *MethodType) NewReplyV() reflect.Value {
	replyv := reflect.New(m.ReplyType.Elem())
	switch m.ReplyType.Elem().Kind() {
	case reflect.Map:
		replyv.Elem().Set(reflect.MakeMap(m.ReplyType.Elem()))
	case reflect.Slice:
		replyv.Elem().Set(reflect.MakeSlice(m.ReplyType.Elem(), 0, 0))
	}
	return replyv
}
