package xtpl

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

// BenchmarkExecute measures rendering only; templates are parsed before the timer starts.
func BenchmarkExecute(b *testing.B) {
	users := make([]map[string]any, 20)
	for i := range users {
		users[i] = map[string]any{"name": "Alice & Bob", "profileURL": "https://example.com/profile?id=123"}
	}
	data := map[string]any{
		"name": "Alice & Bob", "title": "Hello, <World>!", "profileURL": "https://example.com/profile?id=123",
		"query": "alice", "count": 42, "users": users,
	}
	cases := []struct {
		name, source string
		parse        func(io.Reader) (*Template, error)
	}{
		{"pug/text", `p {{title}}`, ParsePugTemplate},
		{"pug/attributes", `a(href="{{profileURL}}" title="{{title}}") {{name}}`, ParsePugTemplate},
		{"pug/query", `a(href="/search?q={{query}}") {{name}}`, ParsePugTemplate},
		{"pug/javascript", "script\n  | const count = {{count}};", ParsePugTemplate},
		{"pug/range", "ul\n  range users\n    li(title=\"{{name}}\"): a(href=\"{{profileURL}}\") {{name}}", ParsePugTemplate},
		{"text/range", `<ul>{{range users}}<li>{{name}}</li>{{end}}</ul>`, ParseTextTemplate},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			tpl, err := tc.parse(strings.NewReader(tc.source))
			if err != nil {
				b.Fatal(err)
			}
			var out bytes.Buffer
			if err := tpl.Execute(&out, data); err != nil {
				b.Fatal(err)
			}
			b.SetBytes(int64(out.Len()))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				out.Reset()
				if err := tpl.Execute(&out, data); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
