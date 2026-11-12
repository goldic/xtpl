package xtpl

import (
	"encoding/json"
	"testing"
	"time"
)

func TestParseExpr(t *testing.T) {
	for _, strExpr := range []string{
		` `,
		` 123 `,
		` "abc 123" `,
		`  Param.Param.abs( ) `,
		` .Param.Param.abs( ) `,
		` .Param .Param .abs( 123 ) `,
		`  Param .Param .abs( 123 ) `,
		` .Param . Param . abs( +123, -123.78 , " abc 123").Abc`,
	} {
		expr, err := ParseExpression(strExpr)
		assertNoErr(t, err)
		require(t, expr != nil)
	}
}

func TestExpression_ToString(t *testing.T) {

	for _, v := range []struct {
		expr   string
		arg    any
		result string
	}{
		// const
		{`  `, nil, ``},
		{` 0 `, nil, `0`},
		{` "0" `, nil, `0`},
		{` 1234 `, nil, `1234`},
		{` 01234 `, nil, `1234`},
		{` +1234 `, nil, `1234`},
		{` +12.340 `, nil, `12.34`},
		{` +12.340 .printf('%.4f') `, nil, `12.3400`},
		{` -123.7000 `, nil, `-123.7`},
		{` " abc " `, nil, ` abc `},

		// method, field
		{` . `, 123, `123`},
		{` .a `, nil, ``},
		{` .a `, OBJ{}, ``},
		{` .a `, OBJ{"a": nil}, ``},
		{` .Param `, OBJ{"Param": "abc"}, `abc`},
		{` Param `, OBJ{"Param": "abc"}, `abc`},
		{`.Param.a.toUpper`, OBJ{"Param": OBJ{"a": "abc"}}, `ABC`},
		{`Param.a.toUpper`, OBJ{"Param": OBJ{"a": "abc"}}, `ABC`},
		{` Param.toUpper`, OBJ{"Param": 123}, `123`},
		{` Param .Str  .toUpper`, OBJ{"Param": Object{Str: "abc"}}, `ABC`},
		{` Param .String .toUpper`, OBJ{"Param": Object{Str: "abc"}}, ``},
		{` Param .String .toUpper`, OBJ{"Param": &Object{Str: "abc"}}, `ABC`},
		{` Param .Str_2 .toUpper`, OBJ{"Param": Object{Str: "abc"}}, `ABC`},
		{` Param .Str_2 .toUpper`, OBJ{"Param": &Object{Str: "abc"}}, `ABC`},
		{` Param .String .toUpper`, OBJ{}, ``},
		{` Param .String .toUpper`, OBJ{"Param": nil}, ``},
		{` Param .String .toUpper`, nil, ``},
		{` Param .String .or( "abc" )`, nil, `abc`},
		{` Param.printf("%+.3f")`, OBJ{"Param": 12.34}, `+12.340`},
		{` Param . printf('%+.3f')`, OBJ{"Param": 12.34}, `+12.340`},
		{` Par.0.concat(.Par1.Sep, .Par2) `, OBJ{"Par": []any{"Hello"}, "Par1": OBJ{"Sep": ", "}, "Par2": "World!"}, `Hello, World!`},

		// nil object
		{` user.name `, nil, ``},
		{` user.name `, OBJ(nil), ``},
		{` user.name `, (*Object)(nil), ``},
		{` user.name `, (*User)(nil), ``},
		{` user.name `, OBJ{"user": nil}, ``},
		{` user.name `, OBJ{"user": OBJ(nil)}, ``},
		{` user.name `, OBJ{"user": (*User)(nil)}, ``},
		{` user `, nil, ``},
		{` user `, OBJ(nil), ``},
		{` user `, (*Object)(nil), ``},
		{` user `, (*User)(nil), ``},
		{` user `, OBJ{"user": nil}, ``},
		{` user `, OBJ{"user": OBJ(nil)}, ``},
		{` user `, OBJ{"user": (*User)(nil)}, ``},

		// getter-object
		{` Get("name") `, &User{OBJ{"name": "Bob"}}, `Bob`},
		{` .name `, &User{OBJ{"name": "Alice"}}, `Alice`},
		{` .unknown `, &User{OBJ{"name": "Alice"}}, ``},
		{` . `, &User{OBJ{"name": "Alice"}}, `{"name":"Alice"}`},

		// json
		{`.Obj `, Object{Obj: &OBJ{"a": 1, "b": 2}}, `{"a":1,"b":2}`},
		{`.Obj `, &Object{Obj: OBJ{"a": 1, "b": 2}}, `{"a":1,"b":2}`},
		{`.Param .String .or( "abc" ).json`, nil, `"abc"`},
		{`.Param.json`, OBJ{"Param": 123}, `123`},
		{`.Param.json`, OBJ{}, `null`},
		{`.Param.json`, nil, `null`},

		// html
		{`.Str.html`, Object{Str: "<ab&cd>"}, `&lt;ab&amp;cd&gt;`},
		{`.html`, OBJ{"a": 1, "b": 2}, `{&#34;a&#34;:1,&#34;b&#34;:2}`},

		// errors
		{`123 #`, nil, ErrorInvalidExpression.Error()},
		{`...<invalid-Expression>...`, nil, ErrorInvalidExpression.Error()},

		// funcs
		{`.Abc.str`, OBJ{}, ``},
		{`.Abc.int`, OBJ{}, `0`},
		{`.Abc.int`, OBJ{"Abc": "abc"}, `0`},
		{`.Abc.int`, OBJ{"Abc": "123"}, `123`},
		{`.Abc.int`, OBJ{"Abc": "123.4"}, `123`},
		{`.Names.1.toUpper`, OBJ{"Names": []string{"Alice", "Bob"}}, `BOB`},
		{`.Names.split(",").1.toUpper`, OBJ{"Names": "Alice,Bob,Carol"}, `BOB`},
		{`.Par.if("ok","empty").toUpper`, OBJ{}, `EMPTY`},
		{`.Par.if("ok","empty").toUpper`, OBJ{"Par": 0}, `EMPTY`},
		{`.Par.if("ok","empty").toUpper`, OBJ{"Par": 123}, `OK`},
		{`.Par.case(1,"one",2,"two","any")`, OBJ{"Par": 2}, `two`},
		{`.Par.case(1,"one",2,"two","any")`, OBJ{"Par": 3}, `any`},
		{`.Par.gte(10.0)`, OBJ{"Par": 10}, `1`},
		{`.Par.gt(10).if("TRUE","FALSE")`, OBJ{"Par": 123}, `TRUE`},
		{`.Arr.map(".int").join(", ")`, OBJ{"Arr": []float64{0.1, 1.2, 2.45, 3.4}}, `0, 1, 2, 3`},
		{`.split(",").map(".trim.int").join(", ")`, "0.1, 1.2, 2.45, 3.4", `0, 1, 2, 3`},
		{`.split(",").map(".trim.int").filter.join(", ")`, "0.1, -0.2, 2.45, 3.4", `2, 3`},
		{`.filter.json`, []any{0, nil, "", "0", 1, 0.0, []any{}, false, true}, `["0",1,[],true]`},
		{`.split(",").len`, `a,b,c,d,e`, `5`},
		{`.len`, OBJ{"a": 1, "b": 2}, `2`},
		{`.substr(7,5)`, "Hello, World!", `World`},
		{`.substr(8,3)`, "Привет, Мир!", `Мир`},
		{`.substr(8)`, "Привет, Мир!", `Мир!`},
		{`.substr(-4)`, "Привет, Мир!", `Мир!`},
		{`.bool`, 123, `1`},
		{`.bool`, OBJ{}, `1`},
		{`.bool`, []any{}, `1`},
		{`.bool`, 0, ``},
		{`.bool`, 0.0, ``},
		{`.bool`, time.Time{}, ``},
		{`.bool`, nil, ``},
		{`.bool`, (*OBJ)(nil), ``},
		{`.bool`, ([]any)(nil), ``},
		{`.bool`, OBJ(nil), ``},
		{`.len.bool`, []any{}, ``},
		{`.len.bool`, OBJ{}, ``},

		// vars
		{`.Par.$.Abc`, OBJ{"Par": 123, "Abc": 456}, `456`},
		{`.Par.set("$abc").$.$abc`, OBJ{"Par": 123, "Abc": 456}, `123`},
		{`.Par.set("$abc").$.var("$abc")`, OBJ{"Par": 123, "Abc": 456}, `123`},
	} {
		expr := NewExpression(v.expr)
		assertEqual(t, v.result, expr.EvalStr(v.arg))
	}
}

type OBJ = map[string]any

type Object struct {
	Int   int
	Int64 int64
	Str   string
	Bool  bool
	Obj   any
}

func (o *Object) String() any {
	return o.Str
}

func (o Object) Str_2() any {
	return o.Str
}

func (o *Object) Param() int {
	return o.Int
}

func (o *Object) Sum() int64 {
	return int64(o.Int) + o.Int64
}

type User struct {
	d map[string]any
}

func (u *User) Get(name string) any {
	return u.d[name]
}

func (u *User) MarshalJSON() ([]byte, error) {
	return json.Marshal(u.d)
}
