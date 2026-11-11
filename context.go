package xtpl

import (
	"io"
	"reflect"
)

// Context provides output, template lookup, and variables to context functions.
type Context interface {
	io.Writer
	Lookup(name string) *Template
	Get(name string) reflect.Value
	Set(name string, v reflect.Value)
}

type context struct {
	io.Writer
	tpl   *Template
	v     reflect.Value
	vars  map[string]reflect.Value
	cache map[string]reflect.Value // {exprPath:resultValue,...}
}

const (
	ctxGlobalVarName = "$"
	ctxParentVarName = "$parent"
	ctxKeyVarName    = "$key"
)

func newContext(w io.Writer, val any) *context {
	v := reflect.ValueOf(val)
	return &context{
		Writer: w,
		v:      v,
		vars: map[string]reflect.Value{
			ctxGlobalVarName: v,
			"$startTime":     reflect.ValueOf(timestamp()),
		},
	}
}

func (ctx *context) setCache(capacity int) {
	if capacity > 0 {
		ctx.cache = make(map[string]reflect.Value, capacity)
	} else {
		ctx.cache = nil
	}
}

func (ctx *context) newSubCtx() *context {
	return &context{
		tpl:    ctx.tpl,
		Writer: ctx.Writer,
		vars: map[string]reflect.Value{
			ctxGlobalVarName: ctx.vars[ctxGlobalVarName],
			ctxParentVarName: ctx.v,
		},
	}
}

func (ctx *context) Get(name string) reflect.Value {
	return ctx.vars[name]
}

func (ctx *context) Set(name string, v reflect.Value) {
	ctx.vars[name] = v
	clear(ctx.cache)
}

func (ctx *context) Lookup(name string) *Template {
	return ctx.tpl.tpls.Lookup(name)
}

func (ctx *context) execute(ee []*Expression) (err error) {
	for _, e := range ee {
		if err = ctx.exec(e); err != nil {
			return
		}
	}
	return
}

func (ctx *context) exec(e *Expression) (err error) {
	switch e.op {
	case opData: // micro-optimization
		_, err = ctx.Writer.Write(e.cdata)

	case opRoot:
		err = ctx.execute(e.args)

	case opIf:
		if !IsEmpty(ctx.eval(e.args[0])) {
			err = ctx.execute(e.args[1:])
		}

	case opRange:
		c := ctx.newSubCtx()
		forEach(ctx.eval(e.args[0]), func(key, v any) {
			c.v = reflect.ValueOf(v)
			c.Set(ctxKeyVarName, reflect.ValueOf(key))
			check(c.execute(e.args[1:]))
		})

	default:
		val := ctx.eval(e)
		r, ok := val.(io.Reader) // is io.Reader
		if !ok {
			if t := ctx.tpl; t != nil && t.esc != nil {
				r = t.esc(val)
			} else {
				r = Raw(val)
			}
		}
		defer func() {
			if c, _ := r.(io.Closer); c != nil {
				c.Close()
			}
		}()
		_, err = io.Copy(ctx.Writer, r)
	}
	return
}

func (ctx *context) eval(e *Expression) any {
	if v := ctx.val(e); isValid(v) {
		return v.Interface()
	}
	return nil
}

var (
	pNil = (*any)(nil)
	vNil = reflect.ValueOf(pNil)
)

func isValid(v reflect.Value) bool {
	return v.Kind() != reflect.Invalid && v.IsValid()
}

func (ctx *context) calArgs(args []*Expression) []reflect.Value {
	res := make([]reflect.Value, len(args))
	for i, e := range args {
		if v := ctx.val(e); isValid(v) {
			res[i] = reflect.ValueOf(v.Interface())
		} else {
			res[i] = vNil
		}
	}
	return res
}

func (ctx *context) val(e *Expression) (res reflect.Value) {
	if e == nil {
		return ctx.v
	}
	switch e.op {
	case opError, opConst:
		return e.cVal

	case opMethod:
		name := e.expr
		if name == "" { // self
			return ctx.v
		}
		//---- cache expressions ----
		if ctx.cache != nil && e.path != "" {
			if res, ok := ctx.cache[e.path]; ok {
				return res
			}
			defer func() { // cache result
				ctx.cache[e.path] = res
			}()
		}
		//----
		aa := ctx.calArgs(e.args)
		if name[0] == '$' { // get .$var
			return ctx.vars[name]
		}
		if fn, ok := ctxFuncs[name]; ok {
			return fn(ctx, aa)
		}
		// defined function
		if fn, ok := funcs[name]; ok {
			safeCall(func() {
				rr := fn.Call(aa)
				res = rr[0]
			})
			return
		}
		// <object>.<Method|Field>
		return fieldOfValue(aa[0], name, aa[1:])
	}
	return
}

func fieldOfValue(v reflect.Value, name string, args []reflect.Value) (res reflect.Value) {
	if !isValid(v) {
		return
	}
	kind := v.Kind()
	switch kind {
	case reflect.Chan, reflect.Func, reflect.Map, reflect.Pointer,
		reflect.UnsafePointer, reflect.Interface, reflect.Slice:
		if v.IsNil() {
			return
		}
	}
	// has method
	switch kind {
	case reflect.Pointer, reflect.Struct, reflect.Interface:
		// todo: ? cache method by type+name
		if m, ok := v.Type().MethodByName(golangName(name)); ok {
			if vv := v.Method(m.Index).Call(args); len(vv) > 0 {
				return vv[0]
			}
			return
		}
	}
	switch kind {
	case reflect.Pointer, reflect.Interface:
		if i, ok := v.Interface().(Getter); ok {
			return reflect.ValueOf(i.Get(name))
		}
		return fieldOfValue(v.Elem(), name, args)

	case reflect.Map:
		return v.MapIndex(reflect.ValueOf(name))

	case reflect.Struct:
		if f, ok := v.Type().FieldByName(golangName(name)); ok {
			return v.FieldByIndex(f.Index)
		}

	case reflect.Array, reflect.Slice:
		return v.Index(atoi(name))
	}
	return
}

func golangName(s string) string {
	if c := s[0]; c >= 'a' && c <= 'z' {
		ss := []byte(s)
		ss[0] -= 'a' - 'A'
		return string(ss)
	}
	return s
}
