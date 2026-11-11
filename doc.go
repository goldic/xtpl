// Package xtpl parses and executes text, HTML, and simplified Pug templates.
//
// HTML escaping is limited to HTML text escaping and is not context-aware.
// Values in URL, JavaScript, CSS, unquoted attribute, and other contexts need
// context-specific handling. Readers and raw values bypass default escaping.
// Templates may call methods on supplied data and must be trusted.
package xtpl
