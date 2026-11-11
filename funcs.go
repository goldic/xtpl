package xtpl

import (
	"reflect"
	"strings"
)

var funcs = map[string]reflect.Value{}
var ctxFuncs = map[string]cntxFunc{}

type cntxFunc = func(ctx Context, args []reflect.Value) reflect.Value

// AddFuncEx registers a context function. Register functions during initialization,
// before executing templates concurrently.
func AddFuncEx(name string, fn func(ctx Context, args []reflect.Value) reflect.Value) error {
	ctxFuncs[name] = fn
	return nil
}

// AddFunc registers a function under comma-separated names. It panics if fn is
// not a function. Register functions before executing templates concurrently.
func AddFunc(name string, fn any) error {
	if v, ok := fn.(cntxFunc); ok {
		return AddFuncEx(name, v)
	}
	v := reflect.ValueOf(fn)
	if v.Kind() != reflect.Func {
		panic("xtpl.AddFunc: invalid func")
	}
	for _, name := range strings.Split(name, ",") {
		funcs[name] = v
	}
	return nil
}
