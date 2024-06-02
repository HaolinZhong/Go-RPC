package service

import (
	"go/ast"
	"log"
	"reflect"
	"sync/atomic"
)

// a Service is in fact a struct. It has a Name, a certain type, an actual instance, and methods associated with the instance.
// It is a self-made reflection of a struct conforming to certain standards.
type Service struct {
	Name   string
	typ    reflect.Type
	rcvr   reflect.Value
	Method map[string]*MethodType
}

// NewService : creating a new Service is the process of reflecting an instance
func NewService(rcvr interface{}) *Service {
	s := new(Service)
	s.rcvr = reflect.ValueOf(rcvr)
	s.Name = reflect.Indirect(s.rcvr).Type().Name()
	s.typ = reflect.TypeOf(rcvr)
	if !ast.IsExported(s.Name) {
		log.Fatalf("rpc server: %s is not a valid Service Name", s.Name)
	}
	s.registerMethods()
	return s
}

func (s *Service) registerMethods() {
	s.Method = make(map[string]*MethodType)
	for i := 0; i < s.typ.NumMethod(); i++ {
		method := s.typ.Method(i)
		mType := method.Type

		// method should be conformed to certain standards: 3 args(struct, arg, reply), 1 error return; args should be exported
		if mType.NumIn() != 3 || mType.NumOut() != 1 {
			continue
		}
		if mType.Out(0) != reflect.TypeOf((*error)(nil)).Elem() {
			continue
		}
		argType, replyType := mType.In(1), mType.In(2)
		if !isExportedOrBuiltInType(argType) || !isExportedOrBuiltInType(replyType) {
			continue
		}

		s.Method[method.Name] = &MethodType{
			Method:    method,
			ArgType:   argType,
			ReplyType: replyType,
		}
	}
}

func (s *Service) Call(m *MethodType, argv, replyv reflect.Value) error {
	atomic.AddUint64(&m.callCount, 1)
	f := m.Method.Func
	returnv := f.Call([]reflect.Value{s.rcvr, argv, replyv})
	if errInter := returnv[0].Interface(); errInter != nil {
		return errInter.(error)
	}
	return nil
}

func isExportedOrBuiltInType(t reflect.Type) bool {
	return ast.IsExported(t.Name()) || t.PkgPath() == ""
}
