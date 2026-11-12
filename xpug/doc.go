// Package xpug parses a simplified Pug-like syntax into an HTML or XML tree.
//
// Its directives are specific to xpug and can be extended with custom
// renderers. xpug is not a complete or compatible implementation of Pug.
// Parse returns a document tree, which can be rendered with Node.String or
// Node.PrettyString. Parser supports custom directive renderers; configure it
// before concurrent use.
//
// Directives can generate expressions for a downstream template engine, but
// xpug does not execute them or apply context-sensitive escaping to their
// values. For Pug templates rendered with contextual escaping, use package
// github.com/goldic/xtpl.
package xpug
