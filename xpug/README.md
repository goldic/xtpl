# xpug

xpug parses a **simplified Pug-like format** into a small syntax tree and renders it as HTML or XML. It is part of the xtpl module and requires Go 1.24 or later.

This is not a complete implementation of Pug and does not aim for full Pug compatibility. In particular, xpug's directives have their own syntax and behavior. They generate text for a subsequent template engine; they do not evaluate expressions or load files. You can register additional directive renderers.

## Installation

```sh
go get github.com/goldic/xtpl/xpug
```

## Example

```go
package main

import (
    "fmt"
    "log"

    "github.com/goldic/xtpl/xpug"
)

func main() {
    source := `doctype html
html
  body
    h1 Hello
    a.button(href="/start") Get started`

    document, err := xpug.Parse(source)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Print(document.PrettyString())
}
```

Output:

```html
<!DOCTYPE html>
<html>
  <body>
    <h1>Hello</h1>
    <a class="button" href="/start">Get started</a>
  </body>
</html>
```

Use `document.String()` for compact output. The parsed tree is also available through `Node.Children` and `Node.Attrs`.

## Supported syntax

- Indented elements, inline text, `.class` and `#id` shortcuts, and implicit `div` for a line starting with a class or ID.
- Attributes in parentheses, with comma or whitespace separators. Values may be quoted or unquoted; attributes without a value are boolean. Escapes inside quoted values and multiline attribute lists are not supported.
- `parent: child` for nested elements on one line.
- `| text` for text lines and a trailing dot, as in `p.`, for an indented text block. Adjacent ordinary text lines are joined with a space; text blocks retain line breaks and relative indentation.
- `script` and `style` content is rendered as raw HTML text, with line breaks preserved. In XML mode it is escaped.
- Lines beginning with `//` and their indented children are discarded.
- `doctype html` and `doctype xml` at the beginning of the document. XML mode emits an XML declaration and uses XML-style escaping.
- Built-in `if`, `range`, and `include` directives and registered custom directives.

Indentation determines nesting; a dedent must match an existing level. Tabs count as four spaces. Empty lines outside text blocks are ignored. Output formatting can add whitespace between elements; use compact output where whitespace is significant.

The XML mode is a convenience renderer, not a complete XML validator. It is the caller's responsibility to supply valid element and attribute names and XML characters.

## Directives

The built-in directives generate template expressions without evaluating them:

```pug
if user.active
  p Welcome
range users: li {{name}}
include("footer.pug")
```

Compact output:

```text
{{ if user.active }}<p>Welcome</p>{{ end }}{{ range users }}<li>{{name}}</li>{{ end }}{{include("footer.pug")}}
```

These expressions are intended for a compatible downstream template engine. `include` does not read a file. xpug does not implement Pug's JavaScript expressions, interpolation, mixins, inheritance, or the full Pug directive language.

Template expressions written in element text or attribute values go through the normal escaping rules. For example, `<` in such an expression becomes `&lt;`; use a downstream syntax that accounts for this. Built-in directive arguments are emitted directly.

Register a custom directive on a parser before parsing and render the resulting tree with that same parser:

```go
parser := xpug.NewParser().AddDirective("when", func(ctx *xpug.RenderContext, node *xpug.Node) {
    ctx.WriteString("{{ when ")
    ctx.WriteString(node.Attr(""))
    ctx.WriteString(" }}")
    ctx.RenderChildren(node)
    ctx.WriteString("{{ end }}")
})

document, err := parser.Parse("when ready: span Ready")
if err != nil {
    log.Fatal(err)
}
fmt.Print(parser.Render(document, false))
```

`node.Data` holds the directive name; `node.Attr("")` holds its argument. The convenience methods `document.String()`, `document.PrettyString()`, and `document.Render()` always use `xpug.DefaultParser`. A directive with no renderer in the parser used for rendering produces no output. An unregistered directive name in the input is parsed as an element.

Configure a parser before sharing it between goroutines. Parsing and rendering can run concurrently once its configuration and the tree are no longer being modified; `AddDirective` must not run concurrently with either operation.

## Escaping

Text and attribute values are HTML-escaped, or XML-escaped in XML mode. Recognized entities are preserved. Raw `script` and `style` content, directive output, and template expressions are not evaluated or sanitized by xpug. In particular, xpug is not a sanitizer for untrusted templates or arbitrary HTML, JavaScript, CSS, or URLs. A downstream template engine must handle dynamic values safely in their output context.

When using `xtpl.ParsePugTemplate`, see the [xtpl documentation](../README.md#automatic-escaping-for-pug) for how dynamic expressions are escaped. Calling xpug directly does not execute or contextually escape those expressions.

## License

MIT. See [LICENSE](../LICENSE).
