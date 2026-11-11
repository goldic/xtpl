package xtpl

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"math"
	"net/url"
	"reflect"
	"strings"
	"time"

	"github.com/goldic/js"
)

func init() {

	AddFuncEx("set", func(ctx Context, args []reflect.Value) reflect.Value {
		ctx.Set(toString(args[1].Interface()), args[0])
		return args[0]
	})

	AddFuncEx("get", func(ctx Context, args []reflect.Value) reflect.Value {
		return ctx.Get(toString(args[1].Interface()))
	})

	AddFuncEx("include", func(ctx Context, args []reflect.Value) reflect.Value {
		name := toString(args[1].Interface())
		tpl := ctx.Lookup(name)
		if tpl == nil {
			panic(fmt.Errorf("xtpl: template %q not found", name))
		}
		check(tpl.Execute(ctx, args[0].Interface()))
		return vNil
	})

	// set cache capacity for context (by default 10). if capacity ≤ 0 than cache is not using
	AddFuncEx("cache", func(ctx Context, args []reflect.Value) reflect.Value {
		size := 10
		if len(args) > 1 {
			size = toInt(args[1].Interface())
		}
		ctx.(*context).setCache(size)
		return vNil
	})

	AddFuncEx("trace", func(ctx Context, args []reflect.Value) reflect.Value {
		fmt.Fprintf(ctx, "%v", ctx)
		return vNil
	})

	//--- types funcs ---
	AddFunc("empty", IsEmpty)
	AddFunc("not", IsEmpty)
	AddFunc("nil", func(v any) any { return nil })
	AddFunc("bool", func(v any) bool { return !IsEmpty(v) })
	AddFunc("str", toString)
	AddFunc("string", toString)
	AddFunc("int", toInt)
	AddFunc("uint", js.ToUint64)
	AddFunc("float", toFloat64)

	//--- math funcs ---
	AddFunc("abs", func(a any) float64 { return math.Abs(toFloat64(a)) })
	AddFunc("mod", func(a, b any) any { return toInt(a) % toInt(b) })
	AddFunc("fmod", func(a, b any) float64 { return math.Mod(toFloat64(a), toFloat64(b)) })
	AddFunc("ceil", func(a any) float64 { return math.Ceil(toFloat64(a)) })
	AddFunc("floor", func(a any) float64 { return math.Floor(toFloat64(a)) })
	AddFunc("round", func(a any) float64 { return math.Round(toFloat64(a)) })
	AddFunc("sub", func(a, b any) float64 { return toFloat64(a) - toFloat64(b) })
	AddFunc("div", func(a, b any) float64 { return toFloat64(a) / toFloat64(b) })

	AddFunc("sum,add", func(v ...any) (sum float64) {
		for _, v := range v {
			sum += toFloat64(v)
		}
		return
	})

	AddFunc("mul", func(v ...any) float64 {
		p := 1.
		for _, v := range v {
			p *= toFloat64(v)
		}
		return p
	})

	//--- string funcs ---
	AddFunc("raw", Raw)
	AddFunc("html", HTML)
	AddFunc("json", JSON)
	AddFunc("fmt,printf", func(v any, format string) string { return fmt.Sprintf(format, v) })
	AddFunc("toUpper", func(v any) string { return strings.ToUpper(toString(v)) })
	AddFunc("toLower", func(v any) string { return strings.ToLower(toString(v)) })
	AddFunc("escURL", func(a any) string { return url.QueryEscape(toString(a)) })
	AddFunc("trim", func(v any) string { return strings.TrimSpace(toString(v)) })
	AddFunc("trimSfx", func(v any, sfx string) string { return strings.TrimSuffix(toString(v), sfx) })
	AddFunc("trimPfx", func(v any, pfx string) string { return strings.TrimPrefix(toString(v), pfx) })
	AddFunc("split", func(v any, sep string) []string { return strings.Split(toString(v), sep) })
	AddFunc("join", func(v any, sep string) string { return strings.Join(toStrings(v), sep) })
	AddFunc("concat", func(args ...any) string { return strings.Join(toStrings(args), "") })

	AddFunc("substr", func(v any, i int, length ...int) string {
		rr := []rune(toString(v))
		if i < 0 {
			i = max(0, i+len(rr))
		}
		rr = rr[min(i, len(rr)):]
		if len(length) > 0 {
			rr = rr[:max(0, min(len(rr), length[0]))]
		}
		return string(rr)
	})

	AddFunc("len", func(v any) int {
		vv := reflect.ValueOf(v)
		if k := vv.Kind(); k == reflect.Slice || k == reflect.Array || k == reflect.String || k == reflect.Map {
			return vv.Len()
		}
		return 0
	})

	AddFunc("and", func(a ...any) (res any) {
		for _, v := range a {
			if res = v; IsEmpty(res) {
				return
			}
		}
		return
	})

	AddFunc("or", func(a ...any) any {
		for _, v := range a {
			if !IsEmpty(v) {
				return v
			}
		}
		return nil
	})

	AddFunc("if", func(v, a any, b ...any) any {
		if !IsEmpty(v) {
			return a
		}
		if len(b) > 0 {
			return b[0]
		}
		return nil
	})

	AddFunc("case", func(v any, vv ...any) any {
		for ; len(vv) > 1; vv = vv[2:] {
			if js.Cmp(v, vv[0]) == 0 {
				return vv[1]
			}
		}
		if len(vv) > 0 {
			return vv[0]
		}
		return nil
	})

	AddFunc("eq,equal", func(a, b any) bool { return js.Cmp(a, b) == 0 })
	AddFunc("lt", func(a, b any) bool { return js.Cmp(a, b) < 0 })
	AddFunc("gt", func(a, b any) bool { return js.Cmp(a, b) > 0 })
	AddFunc("lte", func(a, b any) bool { return js.Cmp(a, b) <= 0 })
	AddFunc("gte", func(a, b any) bool { return js.Cmp(a, b) >= 0 })
	AddFunc("cmp", func(a, b any) int { return js.Cmp(a, b) })

	AddFunc("idx,index", func(v any, i int) any {
		if a := reflect.ValueOf(v); a.Kind() == reflect.Array || a.Kind() == reflect.Slice {
			if i >= 0 && i < a.Len() {
				return a.Index(i).Interface()
			}
		}
		return nil
	})

	AddFunc("map", func(v any, expr string) any {
		res := make([]any, 0)
		if ex, err := ParseExpression(expr); err == nil {
			forEach(v, func(_, vi any) {
				res = append(res, ex.Eval(vi))
			})
		}
		return res
	})

	AddFunc("filter", func(v any) any {
		res := make([]any, 0)
		forEach(v, func(k, vi any) {
			if !IsEmpty(vi) {
				res = append(res, vi)
			}
		})
		return res
	})

	//---- time funcs -----
	AddFunc("timestamp", func(v ...any) float64 {
		return timestamp()
	})

	AddFunc("timeformat,tfmt", func(v any, timeFormat string) string {
		t := js.NewValue(v).Time()
		if timeFormat == "" {
			return t.String()
		}
		return t.Format(timeFormat)
	})
}

var _ = AddFunc("now", func(v any) string { return time.Now().Format("2006-01-02 15:04:05") })
var _ = AddFunc("year", func(v any) int { return js.NewValue(v).Time().Year() })
var _ = AddFunc("day", func(v any) int { return js.NewValue(v).Time().Day() })
var _ = AddFunc("month", func(v any) int { return int(js.NewValue(v).Time().Month()) })

// timestamp returns current time in seconds
func timestamp() float64 {
	return float64(time.Now().UnixMicro()) * 1e-6
}

// IsNil reports whether a is nil, a typed nil, or the internal nil sentinel.
func IsNil(a any) bool {
	return isEmpty(a, false)
}

// IsEmpty reports whether a is nil or a zero value. Non-nil empty slices and
// maps are not empty under this definition.
func IsEmpty(a any) bool {
	return isEmpty(a, true)
}

func isEmpty(a any, isZero bool) bool {
	if a == nil || a == pNil {
		return true
	}
	v := reflect.ValueOf(a)
	switch v.Kind() {
	case reflect.Invalid:
		return true
	case reflect.Chan, reflect.Func, reflect.Map, reflect.Pointer,
		reflect.UnsafePointer, reflect.Interface, reflect.Slice:
		return v.IsNil()
	}
	return isZero && v.IsZero()
}

// HTML returns a reader containing HTML-escaped text. It is not context-aware.
func HTML(v any) io.Reader {
	return bytes.NewBufferString(html.EscapeString(toString(v)))
}

// JSON returns a reader containing JSON. Marshal errors produce empty output.
func JSON(v any) io.Reader {
	if IsNil(v) {
		return bytes.NewBufferString("null")
	}
	b, _ := json.Marshal(v)
	return bytes.NewBuffer(b)
}

// Raw returns a reader for v without HTML escaping.
func Raw(v any) io.Reader {
	if r, ok := v.(io.Reader); ok {
		return r
	}
	return bytes.NewBufferString(toString(v))
}

func toString(val any) string {
	if IsNil(val) {
		return ""
	}
	switch v := val.(type) {
	case bool:
		if v {
			return "1"
		} else {
			return ""
		}
	case time.Time:
		return v.Format("2006-01-02 15:04:05")
	}
	return js.ToStr(val)
}

func forEach(val any, fn func(k, v any)) {
	switch vv := val.(type) {
	case []any:
		for k, v := range vv {
			fn(k, v)
		}
		return
	case interface{ Values() []any }:
		forEach(vv.Values(), fn)
		return
	case interface{ Fetch(func(any)) }:
		k := 0
		vv.Fetch(func(v any) {
			fn(k, v)
			k++
		})
	case interface{ Iterate() any }:
		for k := 0; ; k++ {
			if v := vv.Iterate(); v != nil {
				fn(k, v)
			} else {
				return
			}
		}
	}
	vv := reflect.ValueOf(val)
	switch vv.Kind() {
	case reflect.Slice, reflect.Array, reflect.String:
		for i, n := 0, vv.Len(); i < n; i++ {
			fn(i, vv.Index(i).Interface())
		}
	case reflect.Map:
		for _, k := range vv.MapKeys() {
			fn(k, vv.MapIndex(k).Interface())
		}
	}
	return
}

func toSlice(val any) []any {
	res := make([]any, 0)
	forEach(val, func(_, v any) {
		res = append(res, v)
	})
	return res
}

func toStrings(val any) (res []string) {
	vv := reflect.ValueOf(val)
	if k := vv.Kind(); k != reflect.Slice && k != reflect.Array {
		vv = reflect.ValueOf([]any{val})
	}
	n := vv.Len()
	res = make([]string, n)
	for i := range n {
		res[i] = toString(vv.Index(i).Interface())
	}
	return
}

func toFloat64(v any) float64 {
	return js.ToNum(v)
}

func toInt(v any) int {
	return js.ToInt(v)
}
