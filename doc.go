// Package xtpl parses and executes text, HTML, and simplified Pug templates.
//
// Parse a template once and execute it with different data:
//
//	tpl, err := xtpl.ParsePugTemplate(strings.NewReader("p Hello, {{name}}!"))
//	if err != nil {
//		log.Fatal(err)
//	}
//	if err := tpl.Execute(os.Stdout, map[string]any{"name": "Ada & friends"}); err != nil {
//		log.Fatal(err)
//	}
//	// Output: <p>Hello, Ada &amp; friends!</p>
//
// # Expressions and collections
//
// Expressions support chained properties and functions, for example
// {{user.name}} and {{title.toUpper}}. Missing and nil properties produce an
// empty value. Registered functions take precedence over object methods and
// fields with the same name; {{user.get("url")}} accesses a field or map key
// when its name conflicts with a function. A function may be called with or
// without parentheses when it has no additional arguments.
//
// Templates support if, range, variables and include. Range accepts slices,
// arrays, strings, maps, iter.Seq and iter.Seq2. ParseFS and ParseDir load named
// templates into a Templates collection, with /index as the entry point.
//
// # Escaping and trust
//
// By default, ParsePugTemplate selects context-sensitive escaping while parsing
// for supported HTML contexts, including URLs, JavaScript and CSS. Unsupported
// or ambiguous contexts produce an error. SetAutoEscape controls this for Pug
// templates parsed afterward. ParseHTMLTemplate applies HTML text escaping only;
// it does not provide context-sensitive HTML safety. ParseTextTemplate does not
// escape values. Choose the mode appropriate for the output and data.
//
// Raw, the raw function, and explicit context-escaping functions bypass further
// automatic escaping in every context. Use their output only where it is safe.
// Templates and registered functions can call methods and write output, so they
// must be trusted; do not execute untrusted templates with privileged data.
//
// # Concurrency
//
// Register functions and configure global options, templates and collections
// before concurrent use, or synchronize changes externally. After configuration,
// a template can execute concurrently when its data, functions, readers and
// writers are safe for concurrent use.
//
// See the repository README for the expression syntax, Pug examples and a
// detailed description of escaping and its limitations.
package xtpl
