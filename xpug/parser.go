package xpug

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Parser parses templates and holds the directive renderers used for output.
// Configure a parser before using it concurrently; do not call AddDirective
// concurrently with parsing, rendering, or another call to AddDirective.
type Parser struct {
	tabSize    int
	directives map[string]func(*RenderContext, *Node)
}

// NewParser returns a parser with the built-in if, range, and include directives.
func NewParser() *Parser {
	p := &Parser{
		tabSize:    4,
		directives: make(map[string]func(*RenderContext, *Node)),
	}
	p.AddDirective("if", renderBlockDirective)
	p.AddDirective("range", renderBlockDirective)
	p.AddDirective("include", renderInlineDirective)
	return p
}

// AddDirective registers or replaces a directive renderer and returns p.
// A nil renderer disables recognition of the name as a directive.
func (p *Parser) AddDirective(
	name string,
	render func(*RenderContext, *Node),
) *Parser {
	p.directives[name] = render
	return p
}

// Render renders a node using this parser's registered directives.
// Directives without a registered renderer produce no output.
func (p *Parser) Render(node *Node, pretty bool) string {
	if node == nil {
		return ""
	}
	ctx := NewRenderContext(p, pretty, node.IsXML())
	ctx.Render(node)
	return ctx.String()
}

// DefaultParser is used by Parse and Node's rendering methods.
var DefaultParser = NewParser()

// Parse parses input using DefaultParser.
func Parse(input string) (*Node, error) {
	return DefaultParser.Parse(input)
}

// Parse parses input into a document node. Errors include the input line number.
func (p *Parser) Parse(input string) (*Node, error) {
	lines := normalizeLines(input)
	root := &Node{Type: Document}
	type frame struct {
		spaces int
		node   *Node
	}
	stack := []frame{{spaces: -1, node: root}}
	blockIndents := make(map[*Node]int)
	commentIndent := -1
	blankLines := 0
	lineNo := 0
	for raw := range strings.SplitSeq(lines, "\n") {
		lineNo++
		if strings.TrimSpace(raw) == "" {
			if top := stack[len(stack)-1].node; top.blockText || top.isScriptType() {
				blankLines++
			}
			continue
		}
		trimmedRight := strings.TrimRight(raw, " \t")
		spaces, rest, err := p.measureIndent(trimmedRight)
		if err != nil {
			return nil, fmt.Errorf("line %d: %v", lineNo, err)
		}
		top := stack[len(stack)-1]
		if commentIndent >= 0 {
			if spaces > commentIndent {
				continue
			}
			commentIndent = -1
		}
		if spaces < top.spaces {
			found := false
			for _, f := range stack[1:] {
				if f.spaces == spaces {
					found = true
					break
				}
			}
			if !found {
				return nil, fmt.Errorf("line %d: indentation does not match an outer level", lineNo)
			}
		}
		if spaces <= top.spaces {
			for spaces <= top.spaces {
				stack = stack[:len(stack)-1]
				top = stack[len(stack)-1]
			}
		}
		parent := top.node

		if parent.blockText || parent.isScriptType() {
			indent, ok := blockIndents[parent]
			if !ok {
				indent = spaces
				blockIndents[parent] = indent
			}
			if spaces < indent {
				return nil, fmt.Errorf("line %d: inconsistent text block indentation", lineNo)
			}
			for ; blankLines > 0; blankLines-- {
				appendBlockLine(parent, "")
			}
			text := stripIndent(raw, indent, p.tabSize)
			if !parent.blockText && strings.HasPrefix(text, "|") {
				text = strings.TrimPrefix(text[1:], " ")
			}
			appendBlockLine(parent, text)
			continue
		}
		blankLines = 0
		if strings.HasPrefix(rest, "//") {
			commentIndent = spaces
			continue
		}
		if rest == "doctype" || strings.HasPrefix(rest, "doctype ") || strings.HasPrefix(rest, "doctype\t") {
			if spaces != 0 || len(root.Children) != 0 {
				return nil, fmt.Errorf("line %d: doctype must be the first top-level node", lineNo)
			}
			value := ""
			count := 0
			for part := range strings.FieldsSeq(rest) {
				count++
				if count == 2 {
					value = strings.ToLower(part)
				} else if count > 2 {
					return nil, fmt.Errorf("line %d: invalid doctype", lineNo)
				}
			}
			root.appendChild(&Node{Type: Doctype, Data: value})
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(rest), "|") {
			idx := strings.Index(rest, "|")
			text := strings.TrimSpace(rest[idx+1:])
			parent.appendChild(&Node{Type: Text, Data: text})
			continue
		}
		element, inlineText, push, err := p.parseNodeLine(rest)
		if err != nil {
			return nil, fmt.Errorf("line %d: %v", lineNo, err)
		}
		parent.appendChild(element)
		if push != nil {
			stack = append(stack, frame{spaces: spaces, node: push})
		}
		if inlineText != "" {
			element.appendChild(&Node{Type: Text, Data: inlineText})
		}
	}
	return root, nil
}

func stripIndent(line string, width, tabSize int) string {
	pos, spaces := 0, 0
	for pos < len(line) && spaces < width {
		switch line[pos] {
		case ' ':
			spaces++
		case '\t':
			spaces += tabSize
		default:
			return line[pos:]
		}
		pos++
	}
	if spaces > width {
		return strings.Repeat(" ", spaces-width) + line[pos:]
	}
	return line[pos:]
}

func appendBlockLine(parent *Node, line string) {
	if n := len(parent.Children); n > 0 {
		last := parent.Children[n-1]
		if last.Type == Text || last.Type == RawText {
			last.Data += "\n" + line
			return
		}
	}
	parent.appendChild(&Node{Type: Text, Data: line})
}

func (p *Parser) parseNodeLine(s string) (*Node, string, *Node, error) {
	s = strings.TrimLeft(s, " 	")
	if s == "" {
		return nil, "", nil, fmt.Errorf("empty line")
	}
	if node, inlineText, push, ok, err := p.parseDirectiveLine(s); ok || err != nil {
		return node, inlineText, push, err
	}

	pos := 0
	tag, newPos := parseTagOrImplicitDiv(s, pos)
	pos = newPos

	el := &Node{Type: Element, Data: tag}

	// .class / #id chain
	for pos < len(s) {
		switch s[pos] {
		case '.':
			if pos+1 == len(s) || s[pos+1] == ' ' || s[pos+1] == '\t' {
				goto afterShortcodes
			}
			pos++
			name, np, ok := readIdent(s, pos)
			if !ok || name == "" {
				return nil, "", nil, fmt.Errorf("expected class name after '.'")
			}
			el.addAttr("class", name)
			pos = np
		case '#':
			pos++
			name, np, ok := readIdent(s, pos)
			if !ok || name == "" {
				return nil, "", nil, fmt.Errorf("expected id after '#'")
			}
			el.addAttr("id", name)
			pos = np
		default:
			goto afterShortcodes
		}
	}

afterShortcodes:
	// (attrs)
	if pos < len(s) && s[pos] == '(' {
		attrs, np, err := parseAttrs(s, pos+1)
		if err != nil {
			return nil, "", nil, err
		}
		for _, a := range attrs {
			el.addAttribute(a)
		}
		pos = np
	}

	// skip spaces
	for pos < len(s) && (s[pos] == ' ' || s[pos] == '\t') {
		pos++
	}

	// tag.
	if pos < len(s) && s[pos] == '.' {
		pos++
		for pos < len(s) && (s[pos] == ' ' || s[pos] == '\t') {
			pos++
		}
		el.blockText = true
		if inline := strings.TrimLeft(s[pos:], " \t"); inline != "" {
			el.appendChild(&Node{Type: Text, Data: inline})
		}
		return el, "", el, nil
	}

	// tag: child
	if pos < len(s) && s[pos] == ':' {
		pos++
		for pos < len(s) && (s[pos] == ' ' || s[pos] == '\t') {
			pos++
		}
		rest := strings.TrimLeft(s[pos:], " 	")
		if rest == "" {
			return el, "", el, nil
		}
		if strings.HasPrefix(rest, "|") {
			txt := strings.TrimSpace(rest[1:])
			el.appendChild(&Node{Type: Text, Data: txt})
			return el, "", el, nil
		}
		childEl, childInline, deepest, err := p.parseNodeLine(rest)
		if err != nil {
			return nil, "", nil, err
		}
		if childInline != "" {
			childEl.appendChild(&Node{Type: Text, Data: childInline})
		}
		el.appendChild(childEl)
		if deepest == nil {
			deepest = childEl
		}
		return el, "", deepest, nil
	}

	inlineText := strings.TrimSpace(s[pos:])
	return el, inlineText, el, nil
}

func (p *Parser) parseDirectiveLine(s string) (*Node, string, *Node, bool, error) {
	name, pos, ok := readIdent(s, 0)
	if !ok || p.directives[name] == nil {
		return nil, "", nil, false, nil
	}

	n := len(s)
	for pos < n && (s[pos] == ' ' || s[pos] == '\t') {
		pos++
	}
	argument := ""
	if pos < n && s[pos] == '(' {
		end := strings.IndexByte(s[pos+1:], ')')
		if end < 0 {
			return nil, "", nil, true, fmt.Errorf("unterminated arguments for directive %q", name)
		}
		end += pos + 1
		argument = strings.TrimSpace(s[pos+1 : end])
		pos = end + 1
	} else {
		end := strings.IndexByte(s[pos:], ':')
		if end < 0 {
			argument = strings.TrimSpace(s[pos:])
			pos = n
		} else {
			argument = strings.TrimSpace(s[pos : pos+end])
			pos += end
		}
	}

	node := &Node{Type: Directive, Data: name}
	node.addAttr("", argument)
	for pos < n && (s[pos] == ' ' || s[pos] == '\t') {
		pos++
	}
	if pos < n {
		if s[pos] != ':' {
			return nil, "", nil, true, fmt.Errorf("unexpected content after directive %q", name)
		}
		child, inlineText, push, err := p.parseNodeLine(strings.TrimSpace(s[pos+1:]))
		if err != nil {
			return nil, "", nil, true, err
		}
		if inlineText != "" {
			child.appendChild(&Node{Type: Text, Data: inlineText})
		}
		node.appendChild(child)
		if push == nil {
			push = node
		}
		return node, "", push, true, nil
	}
	return node, "", node, true, nil
}

func normalizeLines(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = strings.TrimPrefix(s, "\uFEFF")
	return s
}

func (p *Parser) measureIndent(line string) (spaces int, rest string, err error) {
	i := 0
	spaces = 0
	for i < len(line) {
		switch line[i] {
		case ' ':
			spaces++
			i++
		case '\t':
			spaces += p.tabSize
			i++
		default:
			return spaces, line[i:], nil
		}
	}
	return spaces, line[i:], nil
}

func parseTagOrImplicitDiv(s string, pos int) (string, int) {
	if s[pos] == '.' || s[pos] == '#' {
		return "div", pos
	}
	name, np, ok := readIdent(s, pos)
	if !ok || name == "" {
		return "div", pos
	}
	return name, np
}

func readIdent(s string, pos int) (string, int, bool) {
	start := pos
	for pos < len(s) {
		r, size := utf8.DecodeRuneInString(s[pos:])
		if pos == start {
			if !(unicode.IsLetter(r) || r == '_') {
				break
			}
		} else {
			if !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_') {
				break
			}
		}
		pos += size
	}
	if pos == start {
		return "", pos, false
	}
	return s[start:pos], pos, true
}

func parseAttrs(s string, pos int) (attrs []*Attribute, _ int, err error) {
	n := len(s)
	for pos < n {
		for pos < n && (s[pos] == ',' || s[pos] == ' ' || s[pos] == '\t') {
			pos++
		}
		if pos >= n {
			break
		}
		if s[pos] == ')' {
			return attrs, pos + 1, nil
		}

		keyStart := pos
		for pos < n && s[pos] != '=' && s[pos] != ',' && s[pos] != ')' && s[pos] != ' ' && s[pos] != '\t' {
			pos++
		}
		key := s[keyStart:pos]
		if key == "" {
			return nil, pos, fmt.Errorf("expected attribute name")
		}

		for pos < n && (s[pos] == ' ' || s[pos] == '\t') {
			pos++
		}
		if pos >= n || s[pos] != '=' {
			attrs = append(attrs, &Attribute{Key: key, Boolean: true})
			continue
		}

		pos++
		for pos < n && (s[pos] == ' ' || s[pos] == '\t') {
			pos++
		}
		if pos >= n {
			return nil, pos, fmt.Errorf("missing value for attribute %q", key)
		}

		value := ""
		quoted := false
		if s[pos] == '\'' || s[pos] == '"' {
			quoted = true
			quote := s[pos]
			pos++
			valueStart := pos
			for pos < n && s[pos] != quote {
				pos++
			}
			if pos >= n {
				return nil, pos, fmt.Errorf("unterminated quoted value for attribute %q", key)
			}
			value = s[valueStart:pos]
			pos++
		} else {
			valueStart := pos
			for pos < n && s[pos] != ',' && s[pos] != ')' && s[pos] != ' ' && s[pos] != '\t' {
				pos++
			}
			value = s[valueStart:pos]
		}
		if value == "" && !quoted {
			return nil, pos, fmt.Errorf("missing value for attribute %q", key)
		}
		attrs = append(attrs, &Attribute{Key: key, Value: value})
	}
	return nil, pos, fmt.Errorf("unterminated attribute list (missing ')')")
}

func renderBlockDirective(ctx *RenderContext, n *Node) {
	ctx.WriteString("{{ ")
	ctx.WriteString(n.Data)
	if argument := n.Attr(""); argument != "" {
		ctx.WriteString(" ")
		ctx.WriteString(argument)
	}
	ctx.WriteString(" }}")
	ctx.Newline()
	ctx.RenderChildren(n)
	ctx.Indent()
	ctx.WriteString("{{ end }}")
}

func renderInlineDirective(ctx *RenderContext, n *Node) {
	ctx.WriteString("{{")
	ctx.WriteString(n.Data)
	if argument := n.Attr(""); argument != "" {
		ctx.WriteString("(")
		ctx.WriteString(argument)
		ctx.WriteString(")")
	}
	ctx.WriteString("}}")
}
