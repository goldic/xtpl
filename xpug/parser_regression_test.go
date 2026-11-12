package xpug_test

import (
	"strings"
	"testing"

	"github.com/goldic/xtpl/xpug"
)

func TestParseAndRender(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		output string
	}{
		{
			name:   "empty document",
			input:  "\ufeff\r\n  \r\n",
			output: "",
		},
		{
			name:   "implicit div and merged classes",
			input:  `.first#main.second(class="third" data-id='42') Hello`,
			output: `<div class="first second third" id="main" data-id="42">Hello</div>`,
		},
		{
			name:   "unicode identifiers",
			input:  "élément.café#東京 Text",
			output: `<élément class="café" id="東京">Text</élément>`,
		},
		{
			name:   "nested shorthand and following child",
			input:  "div: ul: li first\n  span second",
			output: "<div><ul><li>first<span>second</span></li></ul></div>",
		},
		{
			name:   "ordinary text lines are joined",
			input:  "p\n  | first\n  | second",
			output: "<p>first second</p>",
		},
		{
			name:   "text block preserves lines and relative indentation",
			input:  "p.\n  first\n\n    indented\n  | literal pipe\n  last",
			output: "<p>first\n\n  indented\n| literal pipe\nlast</p>",
		},
		{
			name:   "text block with shortcut and inline text",
			input:  "p.note. first\n  second",
			output: "<p class=\"note\">first\nsecond</p>",
		},
		{
			name:   "text block with mixed tab indentation",
			input:  "p.\n  first\n\tsecond",
			output: "<p>first\n  second</p>",
		},
		{
			name:   "script preserves line comments and blank lines",
			input:  "script\n  // comment\n  const a = 1\n\n  const b = 2",
			output: "<script>// comment\nconst a = 1\n\nconst b = 2</script>",
		},
		{
			name:   "style preserves line breaks",
			input:  "style\n  a {\n    color: red;\n  }",
			output: "<style>a {\n  color: red;\n}</style>",
		},
		{
			name:   "nested comments are discarded",
			input:  "div\n  // hidden\n    span hidden\n      b hidden too\n  p visible",
			output: "<div><p>visible</p></div>",
		},
		{
			name:   "line ending normalization",
			input:  "\ufeffdiv\r\n  p first\r  p second\n",
			output: "<div><p>first</p><p>second</p></div>",
		},
		{
			name:   "doctype prefix is an element",
			input:  "doctype-custom example",
			output: "<doctype-custom>example</doctype-custom>",
		},
		{
			name:   "empty doctype defaults to html",
			input:  "doctype\nhtml",
			output: "<!DOCTYPE html><html></html>",
		},
		{
			name:   "attributes distinguish true empty and boolean",
			input:  `input(value="true", title='', disabled, data-id=42)`,
			output: `<input value="true" title="" disabled data-id="42">`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			document, err := xpug.Parse(tt.input)
			if err != nil {
				t.Fatalf("Parse(%q): %v", tt.input, err)
			}
			if got := document.String(); got != tt.output {
				t.Errorf("String() = %q, want %q", got, tt.output)
			}
		})
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		message string
	}{
		{"invalid dedent", "div\n    p first\n   p second", "line 3: indentation does not match an outer level"},
		{"inconsistent text block indentation", "p.\n    first\n  second", "line 3: inconsistent text block indentation"},
		{"nested doctype", "div\n  doctype xml", "line 2: doctype must be the first top-level node"},
		{"late doctype", "p text\ndoctype html", "line 2: doctype must be the first top-level node"},
		{"doctype with extra arguments", "doctype html extra", "line 1: invalid doctype"},
		{"missing class", "p.#main", "line 1: expected class name"},
		{"missing id", "p#", "line 1: expected id"},
		{"unclosed attributes", "div(title='value'", "line 1: unterminated attribute list"},
		{"unclosed quote", `div(title="value)`, "line 1: unterminated quoted value"},
		{"missing attribute value", "div(title=)", "line 1: missing value"},
		{"unclosed directive argument", "if(condition", "line 1: unterminated arguments"},
		{"unexpected directive suffix", "if(condition) extra", "line 1: unexpected content"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			document, err := xpug.Parse(tt.input)
			if err == nil || !strings.Contains(err.Error(), tt.message) {
				t.Fatalf("Parse(%q) error = %v, want an error containing %q", tt.input, err, tt.message)
			}
			if document != nil {
				t.Errorf("Parse(%q) returned a document after an error", tt.input)
			}
		})
	}
}

func TestAttributeNodes(t *testing.T) {
	document, err := xpug.Parse(`input(value="true" title="" disabled)`)
	if err != nil {
		t.Fatal(err)
	}
	attributes := document.Children[0].Attrs
	if len(attributes) != 3 {
		t.Fatalf("got %d attributes, want 3", len(attributes))
	}
	if attributes[0].Value != "true" || attributes[0].Boolean {
		t.Errorf("value attribute = %+v", *attributes[0])
	}
	if attributes[1].Value != "" || attributes[1].Boolean {
		t.Errorf("title attribute = %+v", *attributes[1])
	}
	if !attributes[2].Boolean {
		t.Errorf("disabled attribute = %+v, want boolean", *attributes[2])
	}
}
