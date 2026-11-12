// Portions of this file are adapted from Go's html/template package.
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by the BSD-style license in LICENSE-GO.

package xtpl

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"unicode"
	"unicode/utf8"
)

type escapeFunc func(any) any

var htmlEscaper = strings.NewReplacer("\x00", "\ufffd", `"`, "&#34;", "&", "&amp;", "'", "&#39;", "+", "&#43;", "<", "&lt;", ">", "&gt;")

func escapeHTML(v any) any { return htmlEscaper.Replace(toString(v)) }

func escapeURLFilter(v any) any {
	s := toString(v)
	if !safeURL(s) {
		return "#ZgotmplZ"
	}
	return s
}

func safeURL(s string) bool {
	if protocol, _, ok := strings.Cut(s, ":"); ok && !strings.Contains(protocol, "/") {
		return strings.EqualFold(protocol, "http") || strings.EqualFold(protocol, "https") || strings.EqualFold(protocol, "mailto")
	}
	return true
}

func hexByte(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}

func escapeURL(s string, normalize bool) string {
	const hex = "0123456789abcdef"
	var out strings.Builder
	last := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '-', '.', '_', '~':
			continue
		case '!', '#', '$', '&', '*', '+', ',', '/', ':', ';', '=', '?', '@', '[', ']':
			if normalize {
				continue
			}
		case '%':
			if normalize && i+2 < len(s) && hexByte(s[i+1]) && hexByte(s[i+2]) {
				continue
			}
		default:
			if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' {
				continue
			}
		}
		if last == 0 {
			out.Grow(len(s) + 16)
		}
		out.WriteString(s[last:i])
		out.WriteByte('%')
		out.WriteByte(hex[c>>4])
		out.WriteByte(hex[c&15])
		last = i + 1
	}
	if last == 0 {
		return s
	}
	out.WriteString(s[last:])
	return out.String()
}

func escapeURLNormalize(v any) any { return escapeURL(toString(v), true) }
func escapeURLQuery(v any) any     { return escapeURL(toString(v), false) }

func isHTMLSpace(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' }
func isASCIIAlphaNum(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}

func escapeSrcset(v any) any {
	s := toString(v)
	var b strings.Builder
	// An unsafe URL can be much longer than its filtered output.
	b.Grow(min(len(s), 256))
	first := true
	for part := range strings.SplitSeq(s, ",") {
		if !first {
			b.WriteByte(',')
		}
		first = false
		start := 0
		for start < len(part) && isHTMLSpace(part[start]) {
			start++
		}
		end := start
		for end < len(part) && !isHTMLSpace(part[end]) {
			end++
		}
		valid := safeURL(part[start:end])
		for j := end; j < len(part) && valid; j++ {
			valid = isHTMLSpace(part[j]) || isASCIIAlphaNum(part[j])
		}
		if !valid {
			b.WriteString("#ZgotmplZ")
			continue
		}
		b.WriteString(part[:start])
		b.WriteString(escapeURL(part[start:end], true))
		b.WriteString(part[end:])
	}
	return b.String()
}

func jsIdentPart(r rune) bool {
	return r == '$' || r == '_' || r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'
}

var jsonMarshalerType = reflect.TypeFor[json.Marshaler]()

func escapeJSValue(v any) any {
	if IsNil(v) {
		v = nil
	}
	if v != nil {
		rv := reflect.ValueOf(v)
		for rv.Kind() == reflect.Pointer && !rv.IsNil() && !rv.Type().Implements(jsonMarshalerType) {
			rv = rv.Elem()
		}
		v = rv.Interface()
		if _, ok := v.(json.Marshaler); !ok {
			if s, ok := v.(fmt.Stringer); ok {
				v = s.String()
			}
		}
	}
	b, err := json.Marshal(v)
	if err != nil || len(b) == 0 {
		return " null "
	}
	first, _ := utf8.DecodeRune(b)
	last, _ := utf8.DecodeLastRune(b)
	if jsIdentPart(first) || jsIdentPart(last) {
		return " " + string(b) + " "
	}
	return string(b)
}

func escapeJSString(v any) any   { return escapeJS(toString(v), false, false) }
func escapeJSTemplate(v any) any { return escapeJS(toString(v), true, false) }
func escapeJSRegexp(v any) any {
	s := escapeJS(toString(v), false, true)
	if s == "" {
		return "(?:)"
	}
	return s
}

func escapeJS(s string, templateLiteral, regexp bool) string {
	var out strings.Builder
	last := 0
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		replacement := ""
		switch {
		case r < 0x20:
			switch r {
			case '\t':
				replacement = `\t`
			case '\n':
				replacement = `\n`
			case '\f':
				replacement = `\f`
			case '\r':
				replacement = `\r`
			default:
				replacement = fmt.Sprintf(`\u%04x`, r)
			}
		case r == '\u2028':
			replacement = `\u2028`
		case r == '\u2029':
			replacement = `\u2029`
		case r == '"':
			replacement = `\u0022`
		case r == '`' && !regexp:
			replacement = `\u0060`
		case r == '&':
			replacement = `\u0026`
		case r == '\'':
			replacement = `\u0027`
		case r == '+':
			replacement = `\u002b`
		case r == '/':
			replacement = `\/`
		case r == '<':
			replacement = `\u003c`
		case r == '>':
			replacement = `\u003e`
		case r == '\\':
			replacement = `\\`
		case templateLiteral && r == '$':
			replacement = `\u0024`
		case templateLiteral && r == '{':
			replacement = `\u007b`
		case templateLiteral && r == '}':
			replacement = `\u007d`
		case regexp && strings.ContainsRune("$()*-.?[]^{}|", r):
			replacement = `\` + string(r)
		}
		if replacement != "" {
			if last == 0 {
				out.Grow(len(s))
			}
			out.WriteString(s[last:i])
			out.WriteString(replacement)
			last = i + size
		}
		i += size
	}
	if last == 0 {
		return s
	}
	out.WriteString(s[last:])
	return out.String()
}

func cssSpace(b byte) bool { return b == '\t' || b == '\n' || b == '\f' || b == '\r' || b == ' ' }
func cssNameChar(r rune) bool {
	return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r >= 0x80 && r <= 0xd7ff || r >= 0xe000 && r <= 0xfffd || r >= 0x10000 && r <= 0x10ffff
}

func decodeCSS(s []byte) []byte {
	if !bytes.Contains(s, []byte{'\\'}) {
		return s
	}
	out := make([]byte, 0, len(s))
	for len(s) != 0 {
		i := bytes.IndexByte(s, '\\')
		if i < 0 {
			return append(out, s...)
		}
		out, s = append(out, s[:i]...), s[i:]
		if len(s) < 2 {
			break
		}
		if !hexByte(s[1]) {
			_, n := utf8.DecodeRune(s[1:])
			out, s = append(out, s[1:1+n]...), s[1+n:]
			continue
		}
		j := 2
		for j < len(s) && j < 7 && hexByte(s[j]) {
			j++
		}
		var r rune
		for _, c := range s[1:j] {
			r <<= 4
			switch {
			case c >= '0' && c <= '9':
				r += rune(c - '0')
			case c >= 'a' && c <= 'f':
				r += rune(c - 'a' + 10)
			default:
				r += rune(c - 'A' + 10)
			}
		}
		if r > unicode.MaxRune {
			r, j = r/16, j-1
		}
		out = utf8.AppendRune(out, r)
		s = s[j:]
		if len(s) > 0 && cssSpace(s[0]) {
			if s[0] == '\r' && len(s) > 1 && s[1] == '\n' {
				s = s[2:]
			} else {
				s = s[1:]
			}
		}
	}
	return out
}

func escapeCSSValue(v any) any {
	b := decodeCSS([]byte(toString(v)))
	id := make([]byte, 0, len(b))
	for i, c := range b {
		switch c {
		case 0, '"', '\'', '(', ')', '/', ';', '@', '[', '\\', ']', '`', '{', '}', '<', '>':
			return "ZgotmplZ"
		case '-':
			if i != 0 && b[i-1] == '-' {
				return "ZgotmplZ"
			}
		default:
			if c < utf8.RuneSelf && cssNameChar(rune(c)) {
				id = append(id, c)
			}
		}
	}
	id = bytes.ToLower(id)
	if bytes.Contains(id, []byte("expression")) || bytes.Contains(id, []byte("mozbinding")) {
		return "ZgotmplZ"
	}
	return string(b)
}

var cssReplacements = [128]string{0: `\0`, '\t': `\9`, '\n': `\a`, '\f': `\c`, '\r': `\d`, '"': `\22`, '&': `\26`, '\'': `\27`, '(': `\28`, ')': `\29`, '+': `\2b`, '/': `\2f`, ':': `\3a`, ';': `\3b`, '<': `\3c`, '>': `\3e`, '\\': `\\`, '{': `\7b`, '}': `\7d`}

func escapeCSSString(v any) any {
	s := toString(v)
	var out strings.Builder
	last := 0
	for i := 0; i < len(s); {
		r, n := utf8.DecodeRuneInString(s[i:])
		replacement := ""
		if int(r) < len(cssReplacements) {
			replacement = cssReplacements[r]
		}
		if replacement != "" {
			out.WriteString(s[last:i])
			out.WriteString(replacement)
			last = i + n
			if replacement != `\\` && (last == len(s) || hexByte(s[last]) || cssSpace(s[last])) {
				out.WriteByte(' ')
			}
		}
		i += n
	}
	if last == 0 {
		return s
	}
	out.WriteString(s[last:])
	return out.String()
}

var contextualEscapers = map[string]escapeFunc{
	"_html_template_htmlescaper":      escapeHTML,
	"_html_template_attrescaper":      escapeHTML,
	"_html_template_rcdataescaper":    escapeHTML,
	"_html_template_urlfilter":        escapeURLFilter,
	"_html_template_urlnormalizer":    escapeURLNormalize,
	"_html_template_urlescaper":       escapeURLQuery,
	"_html_template_srcsetescaper":    escapeSrcset,
	"_html_template_jsvalescaper":     escapeJSValue,
	"_html_template_jsstrescaper":     escapeJSString,
	"_html_template_jstmpllitescaper": escapeJSTemplate,
	"_html_template_jsregexpescaper":  escapeJSRegexp,
	"_html_template_cssvaluefilter":   escapeCSSValue,
	"_html_template_cssescaper":       escapeCSSString,
}
