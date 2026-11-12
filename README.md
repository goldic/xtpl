# xtpl

xtpl is a Go template engine with chained expressions, optional-style access to
missing object properties, template collections, and support for HTML, plain
text, and a simplified Pug-like syntax. It requires Go 1.24 or later.

## Installation

```sh
go get github.com/goldic/xtpl
```

## Quick start

Write a Pug template, parse it once, and execute it with your data:

```go
package main

import (
    "log"
    "os"
    "strings"

    "github.com/goldic/xtpl"
)

func main() {
    source := `h1 Hello, {{name}}!
p Welcome to {{site}}.
a(href="{{profileURL}}") View profile`

    tpl, err := xtpl.ParsePugTemplate(strings.NewReader(source))
    if err != nil {
        log.Fatal(err)
    }

    data := map[string]any{
        "name":       "Ada & friends",
        "site":       "Example Club",
        "profileURL": "/members/ada",
    }
    if err := tpl.Execute(os.Stdout, data); err != nil {
        log.Fatal(err)
    }
}
```

Output:

```html
<h1>Hello, Ada &amp; friends!</h1><p>Welcome to Example Club.</p><a href="/members/ada">View profile</a>
```

Dynamic values are escaped according to their HTML context by default. For
example, `&` in the name becomes `&amp;`. See [automatic escaping](#automatic-escaping-for-pug)
for URL filtering and other contexts.

## Loops and conditions

Pug's `range` iterates over a collection, and `if` renders its contents when a
value is non-empty. Inside the loop, expressions refer to the current item.

Replace `source` and `data` in the program above with:

```go
source := `h1 {{title}}
ul
  range users
    li
      b {{name}}
      if online
        span.online online`

data := map[string]any{
    "title": "Members",
    "users": []map[string]any{
        {"name": "Alice", "online": true},
        {"name": "Bob", "online": false},
    },
}
```

Output:

```html
<h1>Members</h1><ul><li><b>Alice</b><span class="online">online</span></li><li><b>Bob</b></li></ul>
```

## Multiple templates with include

A collection lets one template include another. For example, use this layout:

```text
main.go
templates/
    index.pug
    user.pug
```

`templates/index.pug`:

```pug
doctype html
html
  body
    h1 {{title}}
    ul
      range users
        include("/user.pug")
```

`templates/user.pug`:

```pug
li: a(href="{{profileURL}}") {{name}}
```

The included template receives the current value, so `user.pug` sees the user
from the current iteration. This program embeds the files and renders `index`:

```go
package main

import (
    "embed"
    "io/fs"
    "log"
    "os"

    "github.com/goldic/xtpl"
)

//go:embed templates
var files embed.FS

func main() {
    templateFS, err := fs.Sub(files, "templates")
    if err != nil {
        log.Fatal(err)
    }
    templates, err := xtpl.ParseFS(templateFS)
    if err != nil {
        log.Fatal(err)
    }

    data := map[string]any{
        "title": "Members",
        "users": []map[string]any{
            {"name": "Alice", "profileURL": "/members/alice"},
            {"name": "Bob", "profileURL": "/members/bob"},
        },
    }
    if err := templates.Execute(os.Stdout, data); err != nil {
        log.Fatal(err)
    }
}
```

Output:

```html
<!DOCTYPE html><html><body><h1>Members</h1><ul><li><a href="/members/alice">Alice</a></li><li><a href="/members/bob">Bob</a></li></ul></body></html>
```

`ParseDir` is also available for templates on disk. Call
`xtpl.SetPrettyOutput(true)` before parsing if you want formatted Pug output.

`ParseTextTemplate` renders without default escaping. `ParseHTMLTemplate` uses
HTML text escaping. `ParsePugTemplate` uses [xpug](xpug/README.md) to convert
its simplified Pug-like syntax to a template, then analyzes its HTML context and
applies context-specific escaping. `ParseTemplate` accepts custom delimiters and
an escape function. Input is read in full while parsing.

## Expressions and control flow

- `{{user.name}}` looks up `name` in `user`; a missing or nil object property
  produces an empty value and further property access remains empty.
- `{{title.toUpper}}` calls a registered function with the preceding value as
  its first argument. `{{price.printf("%.2f")}}` supplies another argument.
- `{{if online}}...{{end}}` renders a block for a non-empty value, and
  `{{range users}}...{{end}}` iterates a slice, array, string, map,
  `iter.Seq`, or `iter.Seq2`.
- In a range, `.` is the current item, `$` is the original root, `..` is the
  immediate parent, and `$key` is the iteration key. User variables set outside
  a range are not inherited by its child context. Map order is unspecified.
- For `iter.Seq`, `$key` is a zero-based index; for `iter.Seq2`, `$key` is the
  first value yielded by the iterator and `.` is the second. Sequences are
  consumed as they are rendered, so single-use iterators may be exhausted after
  one execution.
- `{{value.set("$name")}}` stores a variable and returns the value;
  `{{$name}}` or `{{var("$name")}}` reads it in the same context.
- `{{include("/partials/header")}}` renders a named template using the current
  value. Includes need a collection created with `ParseFS` or `ParseDir`.
- `{{/* comment */}}` is a comment. The expression language is not JavaScript
  or Go: strings have no escape sequences, and `else` is not supported.

For example, pass a sequence produced by Go's `slices.Values`:

```go
data := map[string]any{
    "users": slices.Values([]string{"Alice", "Bob"}),
}
```

The program needs `import "slices"`. A Pug template can then render the values
and their indices:

```pug
ul
  range users
    li {{$key}}: {{.}}
```

Output:

```html
<ul><li>0: Alice</li><li>1: Bob</li></ul>
```

Built-in functions include `empty`, `bool`, `str`, `int`, `float`, `sum`,
`mul`, `eq`, `lt`, `gt`, `len`, `substr`, `split`, `join`, `map`, `filter`,
`html`, `htmlAttr`, `url`, `urlQuery`, `json`, `jsstr`, `css`, `raw`, `get`,
`var`, and `include`. `IsEmpty` treats nil and Go zero values as empty; a non-nil empty
slice or map is **not** empty. Registered functions and object methods can be
called from templates. See `exfuncs.go` for the full set and their current
behavior.

Registered functions take precedence over object methods and fields with the same name.
Parentheses do not change this: `{{user.url}}` and `{{user.url()}}` both call
the `url` function with `user` as its argument. Use `{{user.get("url")}}` to
read the `url` field or map key instead. `get` also supports values implementing
the `Getter` interface, and returns an empty value for missing or nil objects.
For Go structs, use the exported field name or its lower-initial equivalent,
such as `get("URL")` or `get("name")` for the `URL` and `Name` fields.

The expression evaluator is intentionally tolerant of missing object properties.
Some invalid function calls and panics are currently suppressed, while some
other errors, such as an out-of-range direct slice access, are returned by
`Execute`. Do not rely on suppression to validate template input.

## Template collections

`ParseFS` accepts `fs.FS`, including `embed.FS`; `ParseDir` also supports the
filesystem paths understood by `github.com/goldic/fsys`. They recursively parse
`.html`, `.xhtml`, `.xtpl`, `.pug`, and `.jade` files. Other files are ignored.
A filesystem or parse error aborts loading and identifies the file.

Each file is available by its path and its path without the extension. `Execute`
uses `/index`, and a path ending in `/` resolves to its `index`. Files are
visited in lexical path order; on extensionless name collisions, the last file
wins. Thus `index.pug` wins over `index.html` for `/index`, while each remains
available by its full name. Explicit `Add` also replaces existing entries.
Missing includes and missing index templates return errors.

## Security and concurrency

### Automatic escaping for Pug

By default, `ParsePugTemplate` analyzes the generated HTML using Go's
`html/template`. During parsing, it attaches the appropriate escaping functions
to each expression. At execution time, those functions process each value in
its output context; the context is not analyzed again.

For example:

```go
package main

import (
    "log"
    "os"
    "strings"

    "github.com/goldic/xtpl"
)

func main() {
    source := `p {{message}}
a(href="{{profileURL}}") Open`
    tpl, err := xtpl.ParsePugTemplate(strings.NewReader(source))
    if err != nil {
        log.Fatal(err)
    }

    data := map[string]any{
        "message":    "<b>Hello</b>",
        "profileURL": "javascript:alert(1)",
    }
    if err := tpl.Execute(os.Stdout, data); err != nil {
        log.Fatal(err)
    }
}
```

Output:

```html
<p>&lt;b&gt;Hello&lt;/b&gt;</p><a href="#ZgotmplZ">Open</a>
```

`#ZgotmplZ` is the replacement for a URL with a disallowed scheme. The
replacement does not mean that the original URL was safe.

Supported contexts include HTML text, quoted attributes, URLs, JavaScript values
and strings, and CSS. Unsupported or ambiguous contexts produce a parse error.
Complete dynamic URLs are filtered for unsafe schemes; a URL composed of several
pieces must have a fixed safe prefix, such as `/`, `?`, `#`, `http://`, or
`https://`. `include` must appear as a standalone expression in HTML text.
An included template retains its own escaping mode.
Dynamic CSS is limited to complete property values; URL-containing CSS,
imports, CSS escapes, and concatenated dynamic CSS are rejected.

Pug auto-escaping is enabled by default. `xtpl.SetAutoEscape(false)` disables
context analysis and automatic escaping for Pug templates parsed afterwards;
`xtpl.SetAutoEscape(true)` re-enables it for templates parsed after that call.
This also applies to Pug files loaded by `ParseFS` and `ParseDir`. Previously
parsed templates keep their original behavior. The setting does not change
`ParseTextTemplate`, `ParseHTMLTemplate`, or `ParseTemplate`.

When disabled, dynamic values are not escaped automatically and the Pug
context validation described above is also skipped. Explicit functions such as
`.html` remain available. Only disable auto-escaping when all rendered content
is trusted or handled explicitly. Configure this global setting before
concurrent parsing, or synchronize access externally.

`SetDefaultEscapeFunc` sets an escape function on a template. Its function is
used for Pug templates parsed with auto-escaping disabled; it does not replace
the context-specific handlers already compiled into a Pug template parsed with
auto-escaping enabled.

### Other template modes and trusted values

`ParseHTMLTemplate` applies only HTML text escaping, not context-aware escaping:
it does not validate URL schemes or safely encode arbitrary JavaScript, CSS,
unquoted attributes, or other HTML contexts. `ParseTextTemplate` does not escape
values. Select the appropriate mode for the output and its data.

Values implementing `io.Reader` are read and escaped like other values, and
readers that implement `io.Closer` are closed. `xtpl.Raw` and the template
function `.raw` explicitly bypass escaping in **every context** and must only
be used with trusted, context-appropriate content. For example:

```go
data := map[string]any{"content": xtpl.Raw("<strong>Trusted</strong>")}
```

The explicit functions `html`, `htmlAttr`, `url`, `json`, `jsstr`, and
`css` return raw, already-encoded output for the selected context. They are
intended for deliberate use in text templates; using the wrong function for a
context can be unsafe. Pug templates select their encoders automatically.
`css` filters a CSS value; it does not sanitize an arbitrary stylesheet.
`urlQuery` encodes a URL query parameter and returns an ordinary string. In a
Pug URL, prefer a plain expression such as `/search?q={{query}}`: automatic
escaping already encodes it, and calling `urlQuery` first would encode it twice.

Templates and registered functions are trusted code: they can call methods on
provided values, and context functions can write directly to output. Never
execute an untrusted template with privileged data.

Configure functions with `AddFunc` or `AddFuncEx` during initialization, before
concurrent parsing or execution. `SetPrettyOutput`, `SetAutoEscape`,
`SetDefaultEscapeFunc`, and collection mutations also require initialization-time
configuration or external synchronization. After configuration, a template can
be executed concurrently if its data, functions, readers, and writers are
themselves safe for concurrent use.

## License

MIT. See [LICENSE](LICENSE).
Parts of the contextual escaping implementation are adapted from Go's standard
library and are also covered by [LICENSE-GO](LICENSE-GO).
