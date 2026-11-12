package xpug

import (
	"testing"
)

func TestParseDocument(t *testing.T) {
	const src = `
doctype html
html
    head
        title Мой сайт
        meta(charset="utf-8")
        link(rel="stylesheet" href="/main.css")
    body
		script().
			var a={a:123};
			console.log(a);
        .container#main.col(data-id='42')
            h1 Привет
            p: | Это абзац.
            // This comment is not included in the AST
            p
                | Это абзац, 
                | разбитый на две строки.
			a.button.primary(href="/start" title="Start here", target="_blank" download) Начать
		table:range(expression):tr
			td {{title1}}
			td {{title2}}
		footer (c) Copyright
`
	doc, err := Parse(src)
	require(t, err == nil, err)
	require(t, doc != nil)
	require(t, !doc.IsXML())

	require(t, doc.String() == `<!DOCTYPE html><html><head><title>Мой сайт</title><meta charset="utf-8"><link rel="stylesheet" href="/main.css"></head><body><script>var a={a:123};
console.log(a);</script><div class="container col" id="main" data-id="42"><h1>Привет</h1><p>Это абзац.</p><p>Это абзац, разбитый на две строки.</p><a class="button primary" href="/start" title="Start here" target="_blank" download>Начать</a></div><table>{{ range expression }}<tr><td>{{title1}}</td><td>{{title2}}</td></tr>{{ end }}</table><footer>(c) Copyright</footer></body></html>`)

	require(t, doc.PrettyString() == `<!DOCTYPE html>
<html>
  <head>
    <title>Мой сайт</title>
    <meta charset="utf-8">
    <link rel="stylesheet" href="/main.css">
  </head>
  <body>
    <script>var a={a:123};
console.log(a);</script>
    <div class="container col" id="main" data-id="42">
      <h1>Привет</h1>
      <p>Это абзац.</p>
      <p>Это абзац, разбитый на две строки.</p>
      <a class="button primary" href="/start" title="Start here" target="_blank" download>Начать</a>
    </div>
    <table>
      {{ range expression }}
      <tr>
        <td>{{title1}}</td>
        <td>{{title2}}</td>
      </tr>
      {{ end }}
    </table>
    <footer>(c) Copyright</footer>
  </body>
</html>
`)
}

func TestParseDocumentWithDirectives(t *testing.T) {
	const src = `
doctype html
html
    head
        title {{title}}
        meta(charset="utf-8")
        link(rel="stylesheet", href="/main.css")
    body
        .container#main(data-id='42')
            h1 Hello,
				| {{name}}!
            a.button.primary(href="/start") Start
		table:tr:range(expression)
			th {{key}}:
			td {{value}}
		include(./dir/dir/file)
`
	doc, err := Parse(src)

	require(t, err == nil, err)
	require(t, doc != nil)

	require(t, doc.PrettyString() == `<!DOCTYPE html>
<html>
  <head>
    <title>{{title}}</title>
    <meta charset="utf-8">
    <link rel="stylesheet" href="/main.css">
  </head>
  <body>
    <div class="container" id="main" data-id="42">
      <h1>Hello, {{name}}!</h1>
      <a class="button primary" href="/start">Start</a>
    </div>
    <table>
      <tr>
        {{ range expression }}
        <th>{{key}}:</th>
        <td>{{value}}</td>
        {{ end }}
      </tr>
    </table>
    {{include(./dir/dir/file)}}
  </body>
</html>
`)
}

func TestParseXMLDocument(t *testing.T) {
	const src = `
doctype xml
urlset: url
	loc {{loc}}
	lastmod {{lastmod}}
	changefreq {{changefreq}}
	priority {{priority}}
`

	doc, err := Parse(src)
	require(t, err == nil)
	require(t, doc != nil)
	require(t, doc.IsXML())
	require(t, doc.PrettyString() == `<?xml version="1.0" encoding="UTF-8"?>
<urlset>
  <url>
    <loc>{{loc}}</loc>
    <lastmod>{{lastmod}}</lastmod>
    <changefreq>{{changefreq}}</changefreq>
    <priority>{{priority}}</priority>
  </url>
</urlset>
`)
}

func TestHTMLEscapingAndRawElements(t *testing.T) {
	const src = `
div(title="{{title}} &amp; value") {{text}} &nbsp; &amp; &#169; &#xA9; &#bad; &unknown; <tag>
script
	| if (a < b && c > d) render({{value}});
style
	a > b { content: "&"; width: {{width}}; }
`

	doc, err := Parse(src)
	require(t, err == nil, err)
	expected := `<div title="{{title}} &amp; value">{{text}} &nbsp; &amp; &#169; &#xA9; &amp;#bad; &amp;unknown; &lt;tag&gt;</div><script>if (a < b && c > d) render({{value}});</script><style>a > b { content: "&"; width: {{width}}; }</style>`
	require(t, doc.String() == expected, doc.String())
}

func TestXMLUsesStrictEscapingAndPreservesTemplateExpressions(t *testing.T) {

	const src = `doctype xml
root(value="{{value}} &amp; &nbsp;")
	text {{text}} &amp; &nbsp;
	script if (a < b && c > d) render({{value}});
`
	const expected = `<?xml version="1.0" encoding="UTF-8"?>
<root value="{{value}} &amp; &amp;nbsp;">
  <text>{{text}} &amp; &amp;nbsp;</text>
  <script>if (a &lt; b &amp;&amp; c &gt; d) render({{value}});</script>
</root>
`
	doc, err := Parse(src)
	require(t, err == nil, err)
	require(t, doc.PrettyString() == expected, doc.PrettyString())
}

func TestDirectiveNodes(t *testing.T) {
	const src = `
if user.active
	range users: li {{name}}
include("./footer.pug")
when ready: span Ready
`

	renderWhen := func(ctx *RenderContext, n *Node) {
		ctx.WriteString("{{ when ")
		ctx.WriteString(n.Attr(""))
		ctx.WriteString(" }}")
		ctx.RenderChildren(n)
		ctx.WriteString("{{ end }}")
	}
	parser := NewParser().AddDirective("when", renderWhen)
	doc, err := parser.Parse(src)
	require(t, err == nil, err)
	require(t, len(doc.Children) == 3)

	ifNode := doc.Children[0]
	require(t, ifNode.Type == Directive)
	require(t, ifNode.Data == "if")
	require(t, ifNode.Attr("") == "user.active")
	require(t, len(ifNode.Children) == 1)

	rangeNode := ifNode.Children[0]
	require(t, rangeNode.Type == Directive)
	require(t, rangeNode.Data == "range")
	require(t, rangeNode.Attr("") == "users")
	require(t, len(rangeNode.Children) == 1)
	require(t, rangeNode.Children[0].Type == Element)

	includeNode := doc.Children[1]
	require(t, includeNode.Type == Directive)
	require(t, includeNode.Data == "include")
	require(t, includeNode.Attr("") == `"./footer.pug"`)

	customNode := doc.Children[2]
	require(t, customNode.Type == Directive)
	require(t, customNode.Data == "when")
	require(t, customNode.Attr("") == "ready")
	require(t, parser.Render(customNode, false) == "{{ when ready }}<span>Ready</span>{{ end }}")
}

func require(t *testing.T, ok bool, msg ...any) {
	t.Helper()
	if !ok {
		t.Fatal(msg...)
	}
}
