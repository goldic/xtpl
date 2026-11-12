package xtpl

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	htmltemplate "html/template"
	"io"
	"io/fs"
	"math/rand"
	"strings"
	"testing"
)

func TestPugSecurityExample(t *testing.T) {
	files := must(fs.Sub(testFS, "testdata/security"))

	templates, err := ParseFS(files)
	assertNoErr(t, err)

	data := map[string]any{
		"message":   `</script><img src=x onerror=alert(1)>`,
		"attribute": `" onmouseover="alert(1)`,
		"unsafeURL": `javascript:alert(1)`,
		"safeURL":   `https://example.com/path?a=1&b=2`,
		"query":     `a&next=javascript:alert(1)`,
		"srcset":    `javascript:alert(1) 1x, /safe.png 2x`,
		"payload":   map[string]any{"text": `</script><svg onload=alert(1)>`},
		"color":     `red; background:url(javascript:alert(1))`,
		"trusted":   Raw(`<strong>Trusted HTML</strong>`),
	}
	var output bytes.Buffer
	assertNoErr(t, templates.Execute(&output, data))
	want := strings.TrimSpace(mustReadFile(files, "expected.golden"))
	assertEqual(t, want, output.String())
}

func TestPugContextEscaping(t *testing.T) {
	cases := []struct {
		name, source string
		value        any
		want         string
	}{
		{"html text", `p {{.}}`, `<img src=x onerror=alert(1)>`, `<p>&lt;img src=x onerror=alert(1)&gt;</p>`},
		{"html attribute", `p(title="{{.}}") text`, `" onclick="alert(1)`, `<p title="&#34; onclick=&#34;alert(1)">text</p>`},
		{"url javascript", `a(href="{{.}}") link`, `javascript:alert(1)`, `<a href="#ZgotmplZ">link</a>`},
		{"url mixed case", `a(href="{{.}}") link`, `JaVaScRiPt:alert(1)`, `<a href="#ZgotmplZ">link</a>`},
		{"url data", `a(href="{{.}}") link`, `data:text/html,<script>alert(1)</script>`, `<a href="#ZgotmplZ">link</a>`},
		{"url protocol with whitespace", `a(href="{{.}}") link`, "java\tscript:alert(1)", `<a href="#ZgotmplZ">link</a>`},
		{"safe absolute url", `a(href="{{.}}") link`, `https://example.org/a?x=1&y=2`, `<a href="https://example.org/a?x=1&amp;y=2">link</a>`},
		{"safe relative url", `a(href="{{.}}") link`, `/about#details`, `<a href="/about#details">link</a>`},
		{"url query", `a(href="/search?q={{.}}") link`, `x&admin=1`, `<a href="/search?q=x%26admin%3d1">link</a>`},
		{"javascript value", "script\n  | const value = {{.}};", `</script><img src=x>`, `<script>const value = "\u003c/script\u003e\u003cimg src=x\u003e";</script>`},
		{"javascript string", "script\n  | const value = \"{{.}}\";", `</script><img src=x>`, `<script>const value = "\u003c\/script\u003e\u003cimg src=x\u003e";</script>`},
		{"css safe", `div(style="color:{{.}}") color`, "red", `<div style="color:red">color</div>`},
		{"css injection", `div(style="color:{{.}}") color`, `red; background:url(javascript:alert(1))`, `<div style="color:ZgotmplZ">color</div>`},
		{"reader escapes", `p {{.}}`, strings.NewReader(`<b>reader</b>`), `<p>&lt;b&gt;reader&lt;/b&gt;</p>`},
		{"raw explicitly bypasses", `p {{.}}`, Raw(`<b>trusted</b>`), `<p><b>trusted</b></p>`},
		{"missing chain", `p {{missing.one.two}}`, nil, `<p></p>`},
		{"empty string", `p(title="{{.}}") {{.}}`, "", `<p title=""></p>`},
		{"trusted raw function", `p {{.raw}}`, `<b>trusted</b>`, `<p><b>trusted</b></p>`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tpl, err := ParsePugTemplate(strings.NewReader(tc.source))
			assertNoErr(t, err)
			var output bytes.Buffer
			assertNoErr(t, tpl.Execute(&output, tc.value))
			assertEqual(t, tc.want, output.String())
		})
	}
}

func TestPugRejectsAmbiguousContexts(t *testing.T) {
	cases := []struct{ name, source, message string }{
		{"split url scheme", `a(href="{{a}}{{b}}:alert(1)") link`, "fixed safe prefix"},
		{"split static scheme", `a(href="java{{b}}") link`, "fixed safe prefix"},
		{"url suffix", `a(href="{{scheme}}:{{body}}") link`, "fixed safe prefix"},
		{"dynamic srcset pieces", `img(srcset="/a.png 1x, {{other}} 2x")`, "srcset"},
		{"css url", `div(style="background:url({{a}}{{b}}:alert(1))")`, "dynamic CSS"},
		{"css split keyword", `div(style="background:{{a}}{{b}}(javascript:alert(1))")`, "dynamic CSS"},
		{"css property name", `div(style="{{property}}:red")`, "dynamic CSS"},
		{"css import", "style\n  | @import \"{{.}}\";", "dynamic CSS"},
		{"css url in stylesheet", "style\n  | p { background: url({{.}}); }", "dynamic CSS"},
		{"css encoded keyword", `div(style="background:u\72l({{.}})")`, "dynamic CSS"},
		{"include in attribute", `a(title="{{include('child')}}") text`, "include"},
		{"include in URL", `a(href="{{include('child')}}") link`, "include"},
		{"include in script", "script\n  | {{include('child')}}", "include"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParsePugTemplate(strings.NewReader(tc.source))
			assertErrContains(t, err, tc.message)
		})
	}
}

type checkedReadCloser struct {
	io.Reader
	closed bool
}

func (r *checkedReadCloser) Close() error { r.closed = true; return nil }

type brokenReader struct{ err error }

func (r brokenReader) Read([]byte) (int, error) { return 0, r.err }

func TestPugReaderLifecycle(t *testing.T) {
	tpl, err := ParsePugTemplate(strings.NewReader(`p {{.}}`))
	assertNoErr(t, err)
	r := &checkedReadCloser{Reader: strings.NewReader(`<b>x</b>`)}
	var output bytes.Buffer
	assertNoErr(t, tpl.Execute(&output, r))
	assertEqual(t, true, r.closed)
	assertEqual(t, `<p>&lt;b&gt;x&lt;/b&gt;</p>`, output.String())
	sentinel := errors.New("reader failed")
	assertErrIs(t, tpl.Execute(io.Discard, brokenReader{sentinel}), sentinel)
	rawTemplate, err := ParsePugTemplate(strings.NewReader(`p {{.raw}}`))
	assertNoErr(t, err)
	assertErrIs(t, rawTemplate.Execute(io.Discard, brokenReader{sentinel}), sentinel)
}

func TestHTMLTemplateEscapesReader(t *testing.T) {
	tpl, err := ParseHTMLTemplate(strings.NewReader(`<p>{{.}}</p>`))
	assertNoErr(t, err)
	var output bytes.Buffer
	assertNoErr(t, tpl.Execute(&output, strings.NewReader(`<img src=x onerror=alert(1)>`)))
	assertEqual(t, `<p>&lt;img src=x onerror=alert(1)&gt;</p>`, output.String())
}

func TestExplicitEscapeFunctions(t *testing.T) {
	cases := []struct {
		name, expression string
		value            any
		want             string
	}{
		{"HTML", `{{._html}}`, `<b>&</b>`, `&lt;b&gt;&amp;&lt;/b&gt;`},
		{"attribute", `{{._htmlAttr}}`, `"x"`, `&#34;x&#34;`},
		{"URL", `{{._url}}`, `javascript:alert(1)`, `#ZgotmplZ`},
		{"JSON", `{{._json}}`, map[string]string{"value": `</script>`}, `{"value":"\u003c/script\u003e"}`},
		{"JavaScript string", `{{._jsstr}}`, `</script>`, `\u003c\/script\u003e`},
		{"CSS", `{{._css}}`, `red; background:url(javascript:alert(1))`, `ZgotmplZ`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Eval(tc.expression, tc.value)
			assertNoErr(t, err)
			assertEqual(t, tc.want, got)
		})
	}
}

func TestPugConditionalAndRangeEscaping(t *testing.T) {
	source := `div
  if show
    a(href="{{url}}") {{label}}
  ul
    range users
      li {{name}}`
	tpl, err := ParsePugTemplate(strings.NewReader(source))
	assertNoErr(t, err)
	data := map[string]any{
		"show": true, "url": "javascript:alert(1)", "label": "<unsafe>",
		"users": []map[string]any{{"name": "<Alice>"}, {"name": "Bob & Carol"}},
	}
	var output bytes.Buffer
	assertNoErr(t, tpl.Execute(&output, data))
	want := `<div><a href="#ZgotmplZ">&lt;unsafe&gt;</a><ul><li>&lt;Alice&gt;</li><li>Bob &amp; Carol</li></ul></div>`
	assertEqual(t, want, output.String())
}

func TestPugEscapingMatchesHTMLTemplate(t *testing.T) {
	cases := []struct {
		name, pug, html string
		value           any
	}{
		{"html", `p {{.}}`, `<p>{{.}}</p>`, `<svg onload=alert(1)>`},
		{"quoted attribute", `p(title="{{.}}") text`, `<p title="{{.}}">text</p>`, `" autofocus onfocus="alert(1)`},
		{"javascript URL", `a(href="{{.}}") link`, `<a href="{{.}}">link</a>`, `javascript:alert(1)`},
		{"query parameter", `a(href="/search?q={{.}}") link`, `<a href="/search?q={{.}}">link</a>`, `foo&redirect=javascript:alert(1)`},
		{"srcset", `img(srcset="{{.}}")`, `<img srcset="{{.}}">`, `javascript:alert(1) 1x, /ok.png 2x`},
		{"event handler string", `button(onclick="show('{{.}}')") click`, `<button onclick="show('{{.}}')">click</button>`, `');alert(1);//`},
		{"javascript data", "script\n  | const data = {{.}};", `<script>const data = {{.}};</script>`, map[string]any{"x": `</script><svg onload=alert(1)>`}},
		{"javascript string", "script\n  | const data = \"{{.}}\";", `<script>const data = "{{.}}";</script>`, `";alert(1);//`},
		{"CSS value", `div(style="color:{{.}}")`, `<div style="color:{{.}}"></div>`, `red; background:url(javascript:alert(1))`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pug, err := ParsePugTemplate(strings.NewReader(tc.pug))
			assertNoErr(t, err)
			standard, err := htmltemplate.New("reference").Parse(tc.html)
			assertNoErr(t, err)
			var got, want bytes.Buffer
			assertNoErr(t, pug.Execute(&got, tc.value))
			assertNoErr(t, standard.Execute(&want, tc.value))
			assertEqual(t, want.String(), got.String())
		})
	}
}

func ExampleRaw() {
	tpl, err := ParsePugTemplate(strings.NewReader(`p {{.}}`))
	if err != nil {
		panic(err)
	}
	var output bytes.Buffer
	if err := tpl.Execute(&output, Raw(`<b>trusted</b>`)); err != nil {
		panic(err)
	}
	fmt.Println(output.String())
	// Output: <p><b>trusted</b></p>
}

func TestPugEscapingCharacterCompatibility(t *testing.T) {
	cases := []struct{ name, pug, html string }{
		{"text", `p {{.}}`, `<p>{{.}}</p>`},
		{"attribute", `p(title="{{.}}")`, `<p title="{{.}}"></p>`},
		{"URL", `a(href="{{.}}")`, `<a href="{{.}}"></a>`},
		{"URL query", `a(href="/?q={{.}}")`, `<a href="/?q={{.}}"></a>`},
		{"srcset", `img(srcset="{{.}}")`, `<img srcset="{{.}}">`},
		{"JS value", "script\n  | let x={{.}};", `<script>let x={{.}};</script>`},
		{"JS string", "script\n  | let x=\"{{.}}\";", `<script>let x="{{.}}";</script>`},
		{"JS template literal", "script\n  | let x=`{{.}}`;", "<script>let x=`{{.}}`;</script>"},
		{"JS regexp", "script\n  | let x=/{{.}}/;", `<script>let x=/{{.}}/;</script>`},
		{"CSS value", `p(style="color:{{.}}")`, `<p style="color:{{.}}"></p>`},
	}
	inputs := []string{
		"", "plain", "http://example.com/a?b=c&d=e", "javascript:alert(1)",
		`</script><img src=x onerror="alert(1)">`, `u\72l(javascript:alert(1))`,
		`\72 ed`, `\65 xpression`, `red; background:url(javascript:alert(1))`,
		`javascript:alert(1) 1x, /image.png 2x`, `data:image/svg+xml,<svg> 1x`,
		"\u2028\u2029", "\x00\t\n\r", "тест😊", string([]byte{0xff, 'x'}),
	}
	for c := 0; c < 128; c++ {
		inputs = append(inputs, string(rune(c)))
	}
	random := rand.New(rand.NewSource(1))
	parts := []string{"a", " ", "<", ">", "&", `"`, "'", `\`, "/", "#", ";", "?", "%", "+", ":", "(", ")", "{", "}", "\x00", "\n", "\u2028", "😊", `\72`, "javascript", "url", string([]byte{0xff})}
	for i := 0; i < 100; i++ {
		var input strings.Builder
		for n := random.Intn(12); n > 0; n-- {
			input.WriteString(parts[random.Intn(len(parts))])
		}
		inputs = append(inputs, input.String())
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pug, err := ParsePugTemplate(strings.NewReader(tc.pug))
			assertNoErr(t, err)
			standard, err := htmltemplate.New("reference").Parse(tc.html)
			assertNoErr(t, err)
			for _, input := range inputs {
				t.Run(fmt.Sprintf("%q", input), func(t *testing.T) {
					var got, want bytes.Buffer
					assertNoErr(t, pug.Execute(&got, input))
					assertNoErr(t, standard.Execute(&want, input))
					assertEqual(t, want.String(), got.String())
				})
			}
		})
	}
}

func TestPugJSValuesMatchHTMLTemplate(t *testing.T) {
	pug, err := ParsePugTemplate(strings.NewReader("script\n  | let value={{.}};"))
	assertNoErr(t, err)
	standard, err := htmltemplate.New("reference").Parse(`<script>let value={{.}};</script>`)
	assertNoErr(t, err)
	values := []any{nil, true, false, 0, -42, 1.25, "</script>", []any{1, "<svg>"}, map[string]any{"x": "</script>"}, json.RawMessage(`{"x":"</script>"}`)}
	for _, value := range values {
		t.Run(fmt.Sprintf("%T_%v", value, value), func(t *testing.T) {
			var got, want bytes.Buffer
			assertNoErr(t, pug.Execute(&got, value))
			assertNoErr(t, standard.Execute(&want, value))
			assertEqual(t, want.String(), got.String())
		})
	}
}
