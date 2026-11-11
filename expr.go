package xtpl

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
)

// Expression is a parsed expression that can be evaluated against a value.
type Expression struct {
	op    int
	expr  string
	args  []*Expression
	cVal  reflect.Value
	cdata []byte
	path  string // cache path
}

// Getter supplies named properties to expressions.
type Getter interface {
	Get(string) any
}

const (
	opError = iota + 1
	opConst
	opMethod

	opData
	opRoot
	opIf
	opRange
)

// ErrorInvalidExpression is rendered by NewExpression when parsing fails.
var ErrorInvalidExpression = errors.New("<!-- INVALID xtpl.Expression -->")

var errExpr = &Expression{op: opError, cVal: reflect.ValueOf(ErrorInvalidExpression)}

//var DebugMode = false

// NewExpression parses s, returning an expression that renders ErrorInvalidExpression on failure.
func NewExpression(s string) *Expression {
	e, err := ParseExpression(s)
	if err != nil {
		//if DebugMode {
		//	return &Expression{Typ: typError, Expr: fmt.Sprintf("(INVALID xtpl.EXPRESSION: `%v`)", err)}
		//}
		return errExpr
	}
	return e
}

// ParseExpression parses an expression and reports syntax errors.
func ParseExpression(s string) (expr *Expression, err error) {
	defer catch(&err)

	if s = strings.TrimSpace(s); strings.HasPrefix(s, "$") {
		s = "." + s
	}
	b := newBuffer(s)
	if expr = b.readExpr(nil); expr == nil {
		expr = &Expression{op: opConst, cVal: vNil}
	}
	if !b.eof() {
		return nil, fmt.Errorf("xtpl: unexpected character %q at byte %d", b.cur, b.i)
	}
	return
}

type buffer struct {
	r   []byte
	cur byte
	i   int
	n   int
}

func newBuffer(s string) *buffer {
	r := []byte(s)
	b := &buffer{r: r, n: len(r)}
	if b.n > 0 {
		b.cur = r[0]
	}
	return b
}

func (b *buffer) eof() bool {
	return b.cur == 0
}

func (b *buffer) nextEscSpace() {
	b.next()
	b.escapeSpaces()
}

func (b *buffer) next() {
	if b.i++; b.i < b.n {
		b.cur = b.r[b.i]
	} else {
		b.cur = 0
	}
}

func (b *buffer) escapeSpaces() {
	for isSpace(b.cur) { // read spaces
		b.next()
	}
}

func (b *buffer) readLine(delim string) (res string, ok bool) {
	n := bytes.Index(b.r[b.i:], []byte(delim))
	if ok = n >= 0; !ok {
		n = b.n - b.i
	}
	res = string(b.r[b.i : b.i+n])
	if b.i += n + len(delim); b.i < b.n {
		b.cur = b.r[b.i]
	} else {
		b.cur = 0
	}
	return
}

func (b *buffer) readN(n int) string {
	return b.read(func(byte) bool {
		n--
		return n >= 0
	})
}

func (b *buffer) read(alphabet func(byte) bool) string {
	var rr []byte
	for ; b.cur > 0 && alphabet(b.cur); b.next() {
		rr = append(rr, b.cur)
	}
	return string(rr)
}

func (b *buffer) readName() string {
	return b.read(isNameAlpha)
}

func (b *buffer) readExpr(par *Expression) (e *Expression) {
	b.escapeSpaces()
	cur := b.cur
	if cur == 0 || cur == ',' || cur == ')' {
		return
	}
	switch cur {

	case '.': // .Method|.Field
		b.nextEscSpace()
		fallthrough

	case '_', '$', // Method|Field|$var
		'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k', 'l', 'm',
		'n', 'o', 'p', 'q', 'r', 's', 't', 'u', 'v', 'w', 'x', 'y', 'z',
		'A', 'B', 'C', 'E', 'D', 'F', 'G', 'H', 'I', 'J', 'K', 'L', 'M',
		'N', 'O', 'P', 'Q', 'R', 'S', 'T', 'U', 'V', 'W', 'X', 'Y', 'Z':
		e = &Expression{op: opMethod, expr: b.readName(), args: []*Expression{par}}
		if e.expr == "" && b.cur == '.' {
			e.expr = ctxParentVarName
		}
		if b.cur == '(' { // read args
			for !b.eof() {
				b.nextEscSpace()
				if a := b.readExpr(nil); a != nil {
					e.args = append(e.args, a)
				}
				if b.cur != ',' {
					break
				}
			}
			if b.cur != ')' {
				panic("xtpl: unexpected arguments")
			}
			b.nextEscSpace()
		}

		//----- set path ---
		var pp []string
		for _, a := range e.args {
			if a != nil {
				pp = append(pp, a.path)
			}
		}
		if e.path = strings.Join(pp, ","); len(pp) > 1 {
			e.path = "(" + e.path + ")"
		}
		e.path += "." + e.expr

	case '"', '\'', '`': // string
		b.next() // read '
		v := b.read(func(c byte) bool { return cur != c })
		if b.eof() {
			panic("xtpl: unterminated string")
		}
		b.next() // read '
		e = &Expression{
			op:   opConst,
			cVal: reflect.ValueOf(v),
			path: fmt.Sprintf("(`%v`)", v),
		}

	case '+', '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9': // num
		var v any
		s := b.readN(1) + b.read(isNum)
		if b.cur == '.' { // is float64
			s += b.readN(1) + b.read(isNum)
			v = must(strconv.ParseFloat(s, 64))
		} else { // is int
			v = must(strconv.Atoi(s))
		}
		e = &Expression{
			op:   opConst,
			cVal: reflect.ValueOf(v),
			path: fmt.Sprintf("(`%v`)", v),
		}

	default:
		panic("xtpl: unexpected end of Expression")
	}
	b.escapeSpaces()
	if nxt := b.readExpr(e); nxt != nil {
		return nxt
	}
	return
}

// EvalStr renders an expression, formatting execution errors as HTML comments.
func (e *Expression) EvalStr(val any) string {
	w := bytes.NewBuffer(nil)
	if err := e.Execute(w, val); err != nil {
		return fmt.Sprintf("<!--(Error: %v)-->", err)
	}
	return w.String()
}

// Eval evaluates an expression. Panics from evaluation are not recovered.
func (e *Expression) Eval(val any) any {
	return newContext(nil, val).eval(e)
}

// Execute renders the expression to w using val as its root value.
func (e *Expression) Execute(w io.Writer, val any) (err error) {
	defer catch(&err)
	return newContext(w, val).exec(e)
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\r' || c == '\n'
}

func isNum(c byte) bool {
	return c >= '0' && c <= '9'
}

func isAlpha(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '_' || c == '$'
}

func isNameAlpha(c byte) bool {
	return isAlpha(c) || isNum(c)
}

func atoi(v string) int {
	i, _ := strconv.Atoi(v)
	return i
}

func catch(err *error) {
	if r := recover(); r != nil && *err == nil {
		if cause, ok := r.(error); ok {
			*err = fmt.Errorf("xtpl-error: %w", cause)
		} else {
			*err = fmt.Errorf("xtpl-error: %v", r)
		}
	}
}

func safeCall(fn func()) {
	defer func() {
		recover()
	}()
	fn()
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}
