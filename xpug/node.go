package xpug

import "strings"

// NodeType identifies the kind of syntax represented by a Node.
type NodeType int

const (
	// Text is escaped when rendered.
	Text NodeType = iota + 1
	// Document is the root of a parsed template.
	Document
	// Element is an HTML or XML element.
	Element
	// RawText is emitted without HTML escaping, except in XML mode.
	RawText
	// Doctype selects an HTML doctype or the XML declaration.
	Doctype
	// Directive is rendered by a registered directive function.
	Directive
)

// Node represents a document, element, text, doctype, or directive.
// For directives, Data is the name and Attr("") is the argument.
type Node struct {
	Type     NodeType
	Data     string
	Attrs    []*Attribute
	Children []*Node

	blockText bool
}

// Attribute is an element attribute. Boolean denotes an attribute without a
// value; Value may otherwise be an empty string.
type Attribute struct {
	Key     string
	Value   string
	Boolean bool
}

var voidTags = map[string]bool{
	"area": true, "base": true, "br": true, "col": true, "embed": true,
	"hr": true, "img": true, "input": true, "link": true, "meta": true,
	"param": true, "source": true, "track": true, "wbr": true,
}

// String renders the node without indentation using DefaultParser.
func (node *Node) String() string {
	return node.Render(false)
}

// PrettyString renders the node with indentation using DefaultParser.
func (node *Node) PrettyString() string {
	return node.Render(true)
}

// IsXML reports whether the first child is an XML doctype.
func (node *Node) IsXML() bool {
	return node.DocType() == "xml"
}

// DocType returns the first child's doctype, or "html" if none is present.
func (node *Node) DocType() string {
	if node != nil && len(node.Children) > 0 {
		if ch := node.Children[0]; ch.Type == Doctype && ch.Data != "" {
			return ch.Data
		}
	}
	return "html"
}

func (node *Node) isScriptType() bool {
	return node.Type == Element && (node.Data == "script" || node.Data == "style")
}

// Attr returns an attribute value, or an empty string if it is absent.
func (node *Node) Attr(k string) string {
	if a := node.findAttr(k); a != nil {
		return a.Value
	}
	return ""
}

func (node *Node) findAttr(k string) *Attribute {
	for _, a := range node.Attrs {
		if a.Key == k {
			return a
		}
	}
	return nil
}

func (node *Node) addAttr(k, v string) {
	node.addAttribute(&Attribute{Key: k, Value: v})
}

func (node *Node) addAttribute(attr *Attribute) {
	if attr.Key == "class" && !attr.Boolean {
		if a := node.findAttr(attr.Key); a != nil && !a.Boolean {
			a.Value += " " + attr.Value
			return
		}
	}
	node.Attrs = append(node.Attrs, attr)
}

func (node *Node) appendChild(child *Node) {
	if node.isScriptType() && child.Type == Text {
		child.Type = RawText
	}
	if n := len(node.Children); n > 0 && !node.isScriptType() && child.Type == Text && node.Children[n-1].Type == Text {
		node.Children[n-1].Data += " " + child.Data
		return
	}
	node.Children = append(node.Children, child)
}

func (node *Node) isVoidTag() bool {
	return voidTags[strings.ToLower(node.Data)]
}

// Render renders the node using DefaultParser.
func (node *Node) Render(pretty bool) string {
	return DefaultParser.Render(node, pretty)
}
