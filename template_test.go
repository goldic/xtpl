package xtpl

import (
	"bytes"
	"embed"
	"strings"
	"testing"
	"time"

	"github.com/goldic/fsys"
)

//go:embed testdata
var testFS embed.FS

var testDir = fsys.AddVolume(testFS) + "/testdata"

func testFile(name string) string {
	return trimHTML(mustReadFile(testFS, "testdata"+name))
}

var testData = OBJ{
	"ts":          time.Date(2023, 02, 01, 0, 0, 0, 0, time.UTC),
	"lang":        `en`,
	"title":       `Test document`,
	"description": `Test document 'Hello, "World"'`,
	"slogan":      `Hello, "World"`,
	"logoSrc":     `https://test/path?size="i"&b<'a'`,
	"static":      OBJ{"ver": 123},
	"users": []*User{
		{OBJ{"name": "Alice", "age": 20, "online": 1, "data": OBJ{}}},
		{OBJ{"name": "Bob", "age": 22, "online": 0, "data": OBJ{}}},
		{OBJ{"name": "Carol", "age": 18, "online": 1, "data": OBJ{}}},
	},
	"data": OBJ{
		"arr": []any{1, 2, 3},
		"abc": strings.Split("abcedf", ""),
	},
}

func Test_ParseDir(t *testing.T) {
	tpls, err := ParseDir(testDir)
	assertNoErr(t, err)
	require(t, tpls != nil)

	w := bytes.NewBuffer(nil)
	err = tpls.Execute(w, testData)
	assertNoErr(t, err)
	want := strings.Replace(testFile("/results/index.html"), `https://test/path?size=&#34;i&#34;&amp;b&lt;&#39;a&#39;`, `https://test/path?size=%22i%22&amp;b%3c%27a%27`, 1)
	assertEqual(t, want, trimHTML(w.String()))
}

func Test_ParseFS(t *testing.T) {
	tpls, err := ParseFS(must(fsys.Dir(testDir)))
	assertNoErr(t, err)
	require(t, tpls != nil)

	w := bytes.NewBuffer(nil)
	err = tpls.Execute(w, testData)
	assertNoErr(t, err)
	want := strings.Replace(testFile("/results/index.html"), `https://test/path?size=&#34;i&#34;&amp;b&lt;&#39;a&#39;`, `https://test/path?size=%22i%22&amp;b%3c%27a%27`, 1)
	assertEqual(t, want, trimHTML(w.String()))
	// <!DOCTYPE html><html lang=\"en\"><head><meta charset=\"utf-8\"><title>Test document</title><link type=\"text/css\" rel=\"stylesheet\" href=\"/css/main.css?v123\"></head><body><header class=\"main-header\"><h1>Test document</h1><a class=\"logo\" href=\"/\" title=\"Test document\"><img src=\"https://test/path?size=&#34;i&#34;&amp;b&lt;&#39;a&#39;\"><span class=\"slogan\">Hello, &#34;World&#34;</span></a><p class=\"description\">Test document &#39;Hello, &#34;World&#34;&#39;</p></header><div id=\"container\"><h2>Users list</h2><ul><li data-user=\"{&#34;age&#34;:20,&#34;data&#34;:{},&#34;name&#34;:&#34;Alice&#34;,&#34;online&#34;:1}\">(1)&nbsp;<b>Alice</b></li><li data-user=\"{&#34;age&#34;:22,&#34;data&#34;:{},&#34;name&#34;:&#34;Bob&#34;,&#34;online&#34;:0}\">(2)&nbsp;<b>Bob</b></li><li data-user=\"{&#34;age&#34;:18,&#34;data&#34;:{},&#34;name&#34;:&#34;Carol&#34;,&#34;online&#34;:1}\">(3)&nbsp;<b>Carol</b></li></ul></div><script>console.log(\"Page-data: \", {\"abc\":[\"a\",\"b\",\"c\",\"e\",\"d\",\"f\"],\"arr\":[1,2,3]});</script></body></html>
	// <!DOCTYPE html><html lang=\"en\"><head><meta charset=\"utf-8\"><title>Test document</title><link type=\"text/css\" rel=\"stylesheet\" href=\"/css/main.css?v123\"></head><body><header class=\"main-header\"><h1>Test document</h1><a class=\"logo\" href=\"/\" title=\"Test document\"><img src=\"https://test/path?size=&#34;i&#34;&amp;b&lt;&#39;a&#39;\"><span class=\"slogan\">Hello, &#34;World&#34;</span></a><p class=\"description\">Test document &#39;Hello, &#34;World&#34;&#39;</p></header><div id=\"container\"><h2>Users list</h2><ul></ul></div><script>console.log(\"Page-data: \", {\"abc\":[\"a\",\"b\",\"c\",\"e\",\"d\",\"f\"],\"arr\":[1,2,3]});</script></body></html>
}

func BenchmarkTemplates_Execute(b *testing.B) {
	tt := must(ParseDir(testDir))
	for i := 0; i < b.N; i++ {
		tt.Execute(bytes.NewBuffer(nil), testData)
	}
}

func TestTemplates_Lookup(t *testing.T) {
	tpls, err := ParseDir(testDir)
	assertNoErr(t, err)
	require(t, tpls != nil)

	require(t, tpls.Lookup("") != nil)
	require(t, tpls.Lookup("/") != nil)
	require(t, tpls.Lookup("/index") != nil)
	require(t, tpls.Lookup("/index.pug") != nil)
	require(t, tpls.Lookup("/pages/users") != nil)
	require(t, tpls.Lookup("/pages/users.pug") != nil)
	require(t, tpls.Lookup("") == tpls.Lookup("/index.pug"))
	require(t, tpls.Lookup("") == tpls.Lookup("/index"))
}

func Test_ParseHTMLTemplate(t *testing.T) {
	tpls, err := ParseDir(testDir)
	assertNoErr(t, err)
	require(t, tpls != nil)
	tpl := tpls.Lookup("/index.html")
	require(t, tpl != nil)

	w := bytes.NewBuffer(nil)
	err = tpl.Execute(w, testData)
	assertNoErr(t, err)
	assertEqual(t, testFile("/results/index.html"), trimHTML(w.String()))
}

func Test_ParseTextTemplate(t *testing.T) {

	tpl, err := ParseTextTemplate(bytes.NewBufferString(`
		Users ({{users.set('$USERS').len}}):
		{{ range users }}
			{{$key}}. {{name}} {{if online}} [{{..onlineSymb}}] {{end}}
		{{ end }}
		(Online: {{$USERS.map('online').filter.len}})
	`))
	assertNoErr(t, err)
	require(t, tpl != nil)

	w := bytes.NewBuffer(nil)
	err = tpl.Execute(w, OBJ{
		"onlineSymb": "*",
		"users": []any{
			OBJ{"name": "Alice", "online": 1},
			OBJ{"name": "Bob", "online": 1},
			OBJ{"name": "Carol", "online": 0},
		},
	})
	assertNoErr(t, err)
	assertEqual(t, trimHTML(`
		Users (3):
			0. Alice [*]
			1. Bob [*]
			2. Carol
		(Online: 2)
	`), trimHTML(w.String()))
}

func Test_GenerateSitemap(t *testing.T) {

	const srcPug = `doctype xml
urlset(xmlns="http://www.sitemaps.org/schemas/sitemap/0.9")
	url()
		loc {{loc}}
		lastmod 2025-01-01
		changefreq daily
		priority 0.9
	`
	args := OBJ{
		"loc":        "https://site.com/",
		"lastmod":    "2025-01-01",
		"changefreq": "daily",
		"priority":   0.9,
	}
	const resultXML = `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
    <url>
        <loc>https://site.com/</loc>
        <lastmod>2025-01-01</lastmod>
        <changefreq>daily</changefreq>
        <priority>0.9</priority>
    </url>
</urlset>`

	tpl, err := ParsePugTemplate(bytes.NewBufferString(srcPug))
	assertNoErr(t, err)
	require(t, tpl != nil)

	w := bytes.NewBuffer(nil)
	err = tpl.Execute(w, args)
	assertNoErr(t, err)
	assertEqual(t, trimHTML(resultXML), trimHTML(w.String()))
}

func trimHTML(s string) string {
	r := strings.NewReplacer("\t", " ", "\n", " ", "  ", " ", "> <", "><")
	for s != r.Replace(s) {
		s = r.Replace(s)
	}
	return strings.TrimSpace(s)
}
