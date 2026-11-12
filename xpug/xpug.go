package xpug

import (
	"html"
	"strconv"
	"strings"
	"unicode/utf8"
)

// RenderContext holds the output and settings shared by directive renderers.
type RenderContext struct {
	strings.Builder
	Parser *Parser
	Level  int
	Pretty bool
	XML    bool
}

// NewRenderContext creates a render context. A nil parser selects DefaultParser.
func NewRenderContext(parser *Parser, pretty, isXML bool) *RenderContext {
	if parser == nil {
		parser = DefaultParser
	}
	return &RenderContext{
		Parser: parser,
		Pretty: pretty,
		XML:    isXML,
	}
}

// Render appends the output for node to the context.
func (ctx *RenderContext) Render(node *Node) {
	if node == nil {
		return
	}
	switch node.Type {
	case Document:
		ctx.RenderDocument(node)
	case Doctype:
		ctx.RenderDoctype(node)
	case Text:
		ctx.RenderText(node)
	case RawText:
		ctx.RenderRawText(node)
	case Directive:
		ctx.RenderDirective(node)
	case Element:
		ctx.RenderElement(node)
	}
}

// RenderChildren appends the output for each child of node.
func (ctx *RenderContext) RenderChildren(node *Node) {
	for _, child := range node.Children {
		ctx.Render(child)
	}
}

// Indent writes indentation when pretty output is enabled.
func (ctx *RenderContext) Indent() {
	if ctx.Pretty {
		const spaces = "                                "
		if ctx.Level < 0 {
			ctx.WriteString(strings.Repeat("  ", ctx.Level))
			return
		}
		level := ctx.Level
		for level >= len(spaces)/2 {
			ctx.WriteString(spaces)
			level -= len(spaces) / 2
		}
		ctx.WriteString(spaces[:level*2])
	}
}

// Newline writes a newline when pretty output is enabled.
func (ctx *RenderContext) Newline() {
	if ctx.Pretty {
		ctx.Builder.WriteString("\n")
	}
}

// IsPretty reports whether pretty output is currently enabled.
func (ctx *RenderContext) IsPretty() bool {
	return ctx.Pretty
}

// IsXML reports whether XML output is currently enabled.
func (ctx *RenderContext) IsXML() bool {
	return ctx.XML
}

// RenderDocument renders a document and selects its HTML or XML output mode.
func (ctx *RenderContext) RenderDocument(node *Node) {
	ctx.XML = node.IsXML()
	ctx.RenderChildren(node)
}

// RenderDoctype renders an HTML doctype or XML declaration.
func (ctx *RenderContext) RenderDoctype(node *Node) {
	ctx.Indent()
	if node.Data == "xml" {
		ctx.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	} else {
		ctx.WriteString("<!DOCTYPE ")
		if node.Data == "" {
			ctx.WriteString("html")
		} else {
			ctx.WriteString(node.Data)
		}
		ctx.WriteString(">")
	}
	ctx.Newline()
}

// RenderText renders escaped text.
func (ctx *RenderContext) RenderText(node *Node) {
	ctx.Indent()
	ctx.writeEscaped(node.Data, ctx.XML, false)
	ctx.Newline()
}

// RenderRawText renders unescaped HTML text or escaped XML text.
func (ctx *RenderContext) RenderRawText(node *Node) {
	ctx.Indent()
	if ctx.XML {
		ctx.writeEscaped(node.Data, true, false)
	} else {
		ctx.WriteString(node.Data)
	}
	ctx.Newline()
}

// RenderDirective invokes the registered directive renderer, if any.
func (ctx *RenderContext) RenderDirective(node *Node) {
	ctx.Indent()
	if render := ctx.Parser.directives[node.Data]; render != nil {
		render(ctx, node)
	}
	ctx.Newline()
}

// RenderElement renders an HTML or XML element and its children.
func (ctx *RenderContext) RenderElement(node *Node) {
	ctx.Indent()
	ctx.WriteString("<")
	ctx.WriteString(node.Data)
	for _, attr := range node.Attrs {
		ctx.WriteString(" ")
		ctx.WriteString(attr.Key)
		if attr.Boolean && ctx.XML {
			ctx.WriteString(`="`)
			ctx.writeEscaped(attr.Key, true, true)
			ctx.WriteString(`"`)
		} else if !attr.Boolean {
			ctx.WriteString(`="`)
			ctx.writeEscaped(attr.Value, ctx.XML, true)
			ctx.WriteString(`"`)
		}
	}
	if !ctx.XML && node.isVoidTag() && len(node.Children) == 0 {
		ctx.WriteString(">")
		ctx.Newline()
		return
	}
	ctx.WriteString(">")

	inline := len(node.Children) == 0 || node.Data == "pre" || node.Data == "textarea" || node.isScriptType()
	for _, child := range node.Children {
		if child.Type == Text || child.Type == RawText {
			inline = true
			break
		}
	}
	if !inline {
		ctx.Newline()
	}

	level, pretty := ctx.Level, ctx.Pretty
	ctx.Level++
	ctx.Pretty = ctx.Pretty && !inline
	ctx.RenderChildren(node)
	ctx.Level, ctx.Pretty = level, pretty

	if !inline {
		ctx.Indent()
	}
	ctx.WriteString("</")
	ctx.WriteString(node.Data)
	ctx.WriteString(">")
	ctx.Newline()
}

func (ctx *RenderContext) writeEscaped(s string, xml, attribute bool) {
	// Reuse the next semicolon when multiple ampersands precede it.
	nextSemicolon := -1
	start := 0
	for pos := 0; pos < len(s); {
		if s[pos] != '&' && s[pos] != '<' && s[pos] != '>' && !(attribute && s[pos] == '"') {
			pos++
			continue
		}
		ctx.WriteString(s[start:pos])
		switch s[pos] {
		case '&':
			if nextSemicolon < pos {
				nextSemicolon = strings.IndexByte(s[pos:], ';')
				if nextSemicolon < 0 {
					nextSemicolon = len(s)
				} else {
					nextSemicolon += pos
				}
			}
			if nextSemicolon > pos+1 && nextSemicolon < len(s) && isEntity(s[pos:nextSemicolon+1], xml) {
				entity := s[pos : nextSemicolon+1]
				if attribute {
					ctx.writeAttributeEntity(entity)
				} else {
					ctx.WriteString(entity)
				}
				pos = nextSemicolon + 1
				start = pos
				continue
			}
			ctx.WriteString("&amp;")
		case '<':
			ctx.WriteString("&lt;")
		case '>':
			ctx.WriteString("&gt;")
		case '"':
			ctx.WriteString("&quot;")
		}
		pos++
		start = pos
	}
	ctx.WriteString(s[start:])
}

func (ctx *RenderContext) writeAttributeEntity(entity string) {
	// HTML entity recognition can accept a candidate containing quotes.
	for {
		pos := strings.IndexByte(entity, '"')
		if pos < 0 {
			ctx.WriteString(entity)
			return
		}
		ctx.WriteString(entity[:pos])
		ctx.WriteString("&quot;")
		entity = entity[pos+1:]
	}
}

func isEntity(entity string, xml bool) bool {
	switch entity {
	case "&amp;", "&lt;", "&gt;", "&quot;", "&apos;":
		return true
	case "&nbsp;":
		return !xml
	}
	if entity[1] == '#' && validNumericEntity(entity) {
		return true
	}
	if !xml && html.UnescapeString(entity) != entity {
		return true
	}
	return false
}

func validNumericEntity(entity string) bool {
	value := entity[2 : len(entity)-1]
	base := 10
	if strings.HasPrefix(value, "x") || strings.HasPrefix(value, "X") {
		base = 16
		value = value[1:]
	}
	code, err := strconv.ParseInt(value, base, 32)
	return err == nil && code > 0 && utf8.ValidRune(rune(code))
}
