// Package xtpl parses and executes text, HTML, and simplified Pug templates.
//
// Pug templates receive context-sensitive escaping for supported HTML contexts
// by default. SetAutoEscape controls this for subsequently parsed Pug templates.
// HTML templates use only HTML text escaping; text templates are not escaped.
// Raw explicitly bypasses escaping and must contain trusted content. Templates
// and registered functions can call methods and write output and must be trusted.
package xtpl
