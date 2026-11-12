package xtpl

import (
	"fmt"
	"html"
	htmltemplate "html/template"
	"io"
	"slices"
	"strconv"
	"strings"
	"text/template/parse"

	"github.com/goldic/xtpl/xpug"
)

func collectPugPipelines(node parse.Node, out map[int]string) error {
	if node == nil {
		return nil
	}
	switch n := node.(type) {
	case *parse.ListNode:
		if n == nil {
			return nil
		}
		for _, child := range n.Nodes {
			if err := collectPugPipelines(child, out); err != nil {
				return err
			}
		}
	case *parse.IfNode:
		if err := collectPugPipelines(n.List, out); err != nil {
			return err
		}
		return collectPugPipelines(n.ElseList, out)
	case *parse.RangeNode:
		if err := collectPugPipelines(n.List, out); err != nil {
			return err
		}
		return collectPugPipelines(n.ElseList, out)
	case *parse.ActionNode:
		if len(n.Pipe.Cmds) == 0 || len(n.Pipe.Cmds[0].Args) != 2 {
			return fmt.Errorf("xtpl: invalid Pug escape action")
		}
		name, ok := n.Pipe.Cmds[0].Args[0].(*parse.IdentifierNode)
		if !ok || name.Ident != "_probe" {
			return fmt.Errorf("xtpl: invalid Pug escape action")
		}
		number, ok := n.Pipe.Cmds[0].Args[1].(*parse.NumberNode)
		if !ok || !number.IsInt {
			return fmt.Errorf("xtpl: invalid Pug escape index")
		}
		var pipeline []string
		for _, command := range n.Pipe.Cmds[1:] {
			if len(command.Args) != 1 {
				return fmt.Errorf("xtpl: unsupported Pug escape pipeline")
			}
			id, ok := command.Args[0].(*parse.IdentifierNode)
			if !ok {
				return fmt.Errorf("xtpl: unsupported Pug escape pipeline")
			}
			pipeline = append(pipeline, id.Ident)
		}
		out[int(number.Int64)] = strings.Join(pipeline, "|")
	}
	return nil
}

func preparePugEscaping(doc *xpug.Node, tpl *Template) error {
	if err := validatePugURLs(doc); err != nil {
		return err
	}
	var source strings.Builder
	var expressions []*Expression
	var build func(*Expression) error
	build = func(e *Expression) error {
		switch e.op {
		case opData:
			source.Write(e.cdata)
		case opRoot:
			for _, child := range e.args {
				if err := build(child); err != nil {
					return err
				}
			}
		case opIf, opRange:
			if hasInclude(e.args[0]) {
				return fmt.Errorf("xtpl: include cannot be used in a condition or range")
			}
			if e.op == opIf {
				source.WriteString("{{if true}}")
			} else {
				source.WriteString("{{range .}}")
			}
			for _, child := range e.args[1:] {
				if err := build(child); err != nil {
					return err
				}
			}
			source.WriteString("{{end}}")
		default:
			i := len(expressions)
			expressions = append(expressions, e)
			source.WriteString("{{_probe ")
			source.WriteString(strconv.Itoa(i))
			source.WriteString("}}")
		}
		return nil
	}
	if err := build(tpl.e); err != nil {
		return err
	}
	parsed, err := htmltemplate.New("pug").Funcs(htmltemplate.FuncMap{"_probe": func(int) string { return "" }}).Parse(source.String())
	if err != nil {
		return fmt.Errorf("xtpl: analyze Pug HTML context: %w", err)
	}
	if err = parsed.Execute(io.Discard, []int{}); err != nil {
		return fmt.Errorf("xtpl: analyze Pug HTML context: %w", err)
	}
	signatures := make(map[int]string)
	if err = collectPugPipelines(parsed.Tree.Root, signatures); err != nil {
		return err
	}
	for i, e := range expressions {
		signature, ok := signatures[i]
		if !ok {
			return fmt.Errorf("xtpl: expression %d has no supported HTML context", i)
		}
		if hasInclude(e) && (e.op != opMethod || e.expr != "include" || signature != "_html_template_htmlescaper") {
			return fmt.Errorf("xtpl: include is only supported as a standalone expression in HTML text")
		}
		var chain []escapeFunc
		for name := range strings.SplitSeq(signature, "|") {
			fn, ok := contextualEscapers[name]
			if !ok {
				return fmt.Errorf("xtpl: unsupported Pug escape context %q", signature)
			}
			chain = append(chain, fn)
		}
		original := *e
		*e = Expression{op: opEscape, args: []*Expression{&original}, escapes: chain}
	}
	return nil
}

func hasInclude(e *Expression) bool {
	if e == nil {
		return false
	}
	if e.op == opMethod && e.expr == "include" {
		return true
	}
	return slices.ContainsFunc(e.args, hasInclude)
}

func validatePugURLs(node *xpug.Node) error {
	if node == nil {
		return nil
	}
	if node.Type == xpug.Element {
		if strings.EqualFold(node.Data, "style") {
			var content strings.Builder
			var appendContent func(*xpug.Node)
			appendContent = func(n *xpug.Node) {
				content.WriteString(n.Data)
				for _, child := range n.Children {
					appendContent(child)
				}
			}
			for _, child := range node.Children {
				appendContent(child)
			}
			if unsafeDynamicCSS(content.String()) {
				return fmt.Errorf("xtpl: dynamic CSS is restricted to complete property values")
			}
		}
		for _, attr := range node.Attrs {
			name := strings.ToLower(attr.Key)
			if name == "style" && unsafeDynamicCSS(attr.Value) {
				return fmt.Errorf("xtpl: dynamic CSS is restricted to complete property values")
			}
			if strings.HasPrefix(name, "data-") {
				name = name[5:]
			} else if prefix, short, ok := strings.Cut(name, ":"); ok {
				if prefix == "xmlns" {
					name = "xmlns"
				} else {
					name = short
				}
			}
			isURL := strings.Contains(" action archive background cite classid codebase data formaction href icon longdesc manifest poster profile src usemap xmlns ", " "+name+" ") || strings.Contains(name, "src") || strings.Contains(name, "uri") || strings.Contains(name, "url")
			if !isURL || !strings.Contains(attr.Value, "{{") {
				continue
			}
			value := html.UnescapeString(attr.Value)
			start := strings.Index(value, "{{")
			end := strings.Index(value[start+2:], "}}")
			if end < 0 {
				return fmt.Errorf("xtpl: unclosed expression in URL attribute %q", attr.Key)
			}
			end += start + 4
			if start == 0 && end == len(value) {
				continue
			}
			if name == "srcset" {
				return fmt.Errorf("xtpl: srcset must use a complete expression")
			}
			prefix := strings.ToLower(value[:start])
			if strings.HasPrefix(prefix, "/") || strings.HasPrefix(prefix, "?") || strings.HasPrefix(prefix, "#") || strings.HasPrefix(prefix, "./") || strings.HasPrefix(prefix, "../") || strings.HasPrefix(prefix, "https://") || strings.HasPrefix(prefix, "http://") {
				continue
			}
			return fmt.Errorf("xtpl: URL attribute %q must use a complete expression or a fixed safe prefix", attr.Key)
		}
	}
	for _, child := range node.Children {
		if err := validatePugURLs(child); err != nil {
			return err
		}
	}
	return nil
}

func unsafeDynamicCSS(s string) bool {
	s = html.UnescapeString(s)
	if !strings.Contains(s, "{{") {
		return false
	}
	lower := strings.ToLower(s)
	if strings.Contains(lower, "url") || strings.Contains(lower, "@import") || strings.Contains(s, `\`) {
		return true
	}
	for offset := 0; ; {
		start := strings.Index(s[offset:], "{{")
		if start < 0 {
			return false
		}
		start += offset
		end := strings.Index(s[start+2:], "}}")
		if end < 0 {
			return true
		}
		end += start + 4
		before := strings.TrimSpace(s[:start])
		after := strings.TrimSpace(s[end:])
		if !strings.HasSuffix(before, ":") || (after != "" && after[0] != ';' && after[0] != '}') {
			return true
		}
		offset = end
	}
}
