# xtpl

xtpl is a Go template engine with chained expressions, optional-style access to
missing object properties, template collections, and support for HTML, plain
text, and a simplified Pug-like syntax. It requires Go 1.24 or later.

> **Publication in progress:** `github.com/goldic/xpug` has not been published
> yet. Its `v0.0.0` requirement is a placeholder, so installation will work
> only after xpug is published and this module is updated to a real version.

## Installation

```sh
go get github.com/goldic/xtpl
```

## Example

```go
package main

import (
    "log"
    "os"
    "strings"

    "github.com/goldic/xtpl"
)

func main() {
    source := `<h1>{{title}}</h1>{{range users}}<p>{{name}}</p>{{end}}`
    template, err := xtpl.ParseHTMLTemplate(strings.NewReader(source))
    if err != nil {
        log.Fatal(err)
    }
    data := map[string]any{
        "title": "Team & friends",
        "users": []map[string]any{{"name": "Alice"}, {"name": "Bob"}},
    }
    if err := template.Execute(os.Stdout, data); err != nil {
        log.Fatal(err)
    }
}
```

Output:

```html
<h1>Team &amp; friends</h1><p>Alice</p><p>Bob</p>
```

`ParseTextTemplate` renders without default escaping. `ParseHTMLTemplate` uses
HTML text escaping. `ParsePugTemplate` uses [xpug](https://github.com/goldic/xpug)
to convert its simplified Pug-like syntax to a template. `ParseTemplate` accepts
custom delimiters and an escape function. Input is read in full while parsing.

## Expressions and control flow

- `{{user.name}}` looks up `name` in `user`; a missing or nil object property
  produces an empty value and further property access remains empty.
- `{{title.toUpper}}` calls a registered function with the preceding value as
  its first argument. `{{price.printf("%.2f")}}` supplies another argument.
- `{{if online}}...{{end}}` renders a block for a non-empty value, and
  `{{range users}}...{{end}}` iterates a slice, array, string, or map.
- In a range, `.` is the current item, `$` is the original root, `..` is the
  immediate parent, and `$key` is the iteration key. User variables set outside
  a range are not inherited by its child context. Map order is unspecified.
- `{{value.set("$name")}}` stores a variable and returns the value;
  `{{$name}}` reads it in the same context.
- `{{include("/partials/header")}}` renders a named template using the current
  value. Includes need a collection created with `ParseFS` or `ParseDir`.
- `{{/* comment */}}` is a comment. The expression language is not JavaScript
  or Go: strings have no escape sequences, and `else` is not supported.

Built-in functions include `empty`, `bool`, `str`, `int`, `float`, `sum`,
`mul`, `eq`, `lt`, `gt`, `len`, `substr`, `split`, `join`, `map`, `filter`,
`html`, `json`, `raw`, and `include`. `IsEmpty` treats nil and Go zero values
as empty; a non-nil empty slice or map is **not** empty. Registered functions
and object methods can be called from templates. See `exfuncs.go` for the full
set and their current behavior.

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

**HTML escaping is not context-aware.** `html.EscapeString` protects ordinary
HTML text and appropriately quoted attribute text, but does not validate URL
schemes or safely encode arbitrary JavaScript, CSS, unquoted attributes, or
other HTML contexts. For example, `javascript:...` remains a dangerous URL.
For untrusted values in arbitrary HTML contexts, use Go's `html/template` or
apply context-specific validation and encoding. Values implementing `io.Reader`
and values produced by `raw` or `json` bypass default escaping. Readers that
also implement `io.Closer` are closed after rendering.

Templates are trusted code: they can call registered functions and methods on
provided values. Never execute an untrusted template with privileged data.

Configure functions with `AddFunc` or `AddFuncEx` during initialization, before
concurrent parsing or execution. `SetPrettyOutput`, `SetDefaultEscapeFunc`, and
collection mutations also require initialization-time configuration or external
synchronization. After configuration, a template can be executed concurrently
if its data, functions, readers, and writers are themselves safe for concurrent
use.

## License

MIT. See [LICENSE](LICENSE).
