package xtpl

import (
	"bytes"
	"fmt"
	"github.com/goldic/xpug"
	"io"
	"reflect"
	"strings"
)

// Template is a parsed template that can be executed with different data.
type Template struct {
	tpls Templates           //
	e    *Expression         //
	esc  func(any) io.Reader // default escape func
}

var pugPrettyOutput bool

// SetPrettyOutput controls formatting when parsing Pug templates. Configure it
// before parsing templates concurrently.
func SetPrettyOutput(pretty bool) {
	pugPrettyOutput = pretty
}

// SetDefaultEscapeFunc sets the escape function used for non-reader values.
// Configure templates before executing them concurrently.
func (tpl *Template) SetDefaultEscapeFunc(fn func(any) io.Reader) {
	if tpl != nil {
		tpl.esc = fn
	}
}

// NewErrorTemplate creates a template that renders err as text.
func NewErrorTemplate(err string) *Template {
	return &Template{e: &Expression{op: opError, cVal: reflect.ValueOf(err)}}
}

// ParsePugTemplate parses a simplified Pug template as HTML or XML.
func ParsePugTemplate(r io.Reader) (tpl *Template, err error) {
	defer catch(&err)

	data := must(io.ReadAll(r))
	doc := must(xpug.Parse(string(data)))

	output := ""
	if pugPrettyOutput || doc.IsXML() {
		output = doc.PrettyString()
	} else {
		output = doc.String()
	}
	return ParseHTMLTemplate(bytes.NewBufferString(output))
}

// ParseHTMLTemplate parses a template using HTML text escaping by default.
// Escaping is not context-aware; see the package security documentation.
func ParseHTMLTemplate(r io.Reader) (tpl *Template, err error) {
	tpl, err = ParseTextTemplate(r)
	tpl.SetDefaultEscapeFunc(HTML)
	return
}

// ParseTextTemplate parses a template with {{ and }} delimiters and no escaping.
func ParseTextTemplate(r io.Reader) (tpl *Template, err error) {
	defer catch(&err)
	return ParseTemplate(string(must(io.ReadAll(r))), "{{", "}}", nil)
}

// ParseTemplate parses a template with custom delimiters and an escape function.
func ParseTemplate(
	data string,
	delimL, delimR string,
	escFn func(any) io.Reader,
) (tpl *Template, err error) {
	defer catch(&err)
	if delimL == "" || delimR == "" {
		return nil, fmt.Errorf("xtpl: template delimiters must not be empty")
	}

	buf := newBuffer(data)
	pp := []*Expression{{op: opRoot}}
	cur := pp[0]
	push := func(e *Expression) {
		cur.args = append(cur.args, e)
		pp, cur = append(pp, e), e
	}
	pop := func() {
		n := len(pp) - 1
		if n == 0 {
			panic("xtpl: unexpected end of template")
		}
		pp, cur = pp[:n], pp[n-1]
	}
	newExpr := func(s string) *Expression {
		return must(ParseExpression(strings.TrimSpace(s)))
	}
	for !buf.eof() {
		line, ok := buf.readLine(delimL)
		if len(line) > 0 {
			cur.args = append(cur.args, &Expression{op: opData, cdata: []byte(line)})
		}
		if ok {
			if line, ok = buf.readLine(delimR); !ok {
				panic("xtpl: unexpected end of Expression")
			}
			sLine := strings.TrimSpace(line)
			if strings.HasPrefix(sLine, "/*") { // skip {{/* _, _ */}}
				if !strings.HasSuffix(sLine, "*/") {
					panic("xtpl: unterminated comment")
				}
				continue
			}
			pfx := strings.SplitN(sLine, " ", 2)[0]
			switch pfx {
			case "if":
				push(&Expression{
					op:   opIf,
					args: []*Expression{newExpr(strings.TrimPrefix(sLine, pfx))},
				})
			case "range":
				push(&Expression{
					op:   opRange,
					args: []*Expression{newExpr(strings.TrimPrefix(sLine, pfx))},
				})
			case "end":
				pop()
			default:
				cur.args = append(cur.args, newExpr(sLine))
			}
		}
	}
	if len(pp) != 1 {
		return nil, fmt.Errorf("xtpl: unclosed if or range block")
	}
	tpl = &Template{
		e:   pp[0],
		esc: escFn,
	}
	return
}

// Execute renders the template to w using val as its root value.
func (tpl *Template) Execute(w io.Writer, val any) (err error) {
	if tpl == nil {
		return fmt.Errorf("xtpl: cannot execute a nil template")
	}
	defer catch(&err)
	ctx := newContext(w, val)
	ctx.tpl = tpl
	return ctx.exec(tpl.e)
}

// Eval parses and renders a text template with v as its root value.
func Eval(textTemplate string, v any) (res string, err error) {
	tpl, err := ParseTextTemplate(bytes.NewBufferString(textTemplate))
	if err != nil {
		return
	}
	w := bytes.NewBuffer(nil)
	err = tpl.Execute(w, v)
	return w.String(), err
}
