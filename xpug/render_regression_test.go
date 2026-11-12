package xpug_test

import (
	"encoding/xml"
	"io"
	"strings"
	"testing"

	"github.com/goldic/xtpl/xpug"
)

func TestPrettyRenderingPreservesMixedContent(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		output string
	}{
		{
			name:   "mixed text and inline element",
			input:  "p\n  | Hello\n  strong world\n  | !",
			output: "<p>Hello<strong>world</strong>!</p>\n",
		},
		{
			name:   "preformatted content with a nested element",
			input:  "pre\n  | Hello\n  strong world",
			output: "<pre>Hello<strong>world</strong></pre>\n",
		},
		{
			name:   "textarea content with a nested element",
			input:  "textarea\n  | Hello\n  strong world",
			output: "<textarea>Hello<strong>world</strong></textarea>\n",
		},
		{
			name:   "multiline script remains intact",
			input:  "div\n  script\n    // comment\n    const a = 1\n    const b = 2",
			output: "<div>\n  <script>// comment\nconst a = 1\nconst b = 2</script>\n</div>\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			document, err := xpug.Parse(tt.input)
			if err != nil {
				t.Fatal(err)
			}
			if got := document.PrettyString(); got != tt.output {
				t.Errorf("PrettyString() = %q, want %q", got, tt.output)
			}
		})
	}
}

func TestXMLBooleanAttributesAndEscaping(t *testing.T) {
	input := "doctype xml\nroot(enabled value=\"true\" empty=\"\")\n  text &nbsp; &amp; <tag>"
	document, err := xpug.Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if !document.IsXML() || document.DocType() != "xml" {
		t.Fatalf("unexpected document type %q", document.DocType())
	}
	want := `<?xml version="1.0" encoding="UTF-8"?><root enabled="enabled" value="true" empty=""><text>&amp;nbsp; &amp; &lt;tag&gt;</text></root>`
	got := document.String()
	if got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
	decoder := xml.NewDecoder(strings.NewReader(got))
	for {
		_, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("rendered XML is invalid: %v", err)
		}
	}
}

func TestEntityEscaping(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"named and numeric HTML entities", `p &amp; &nbsp; &#169; &#xA9;`, `<p>&amp; &nbsp; &#169; &#xA9;</p>`},
		{"invalid named entity and text", `p &bogus; & < >`, `<p>&amp;bogus; &amp; &lt; &gt;</p>`},
		{"attribute quotes and ampersands", `p(title='A "quote" & value') text`, `<p title="A &quot;quote&quot; &amp; value">text</p>`},
		{"quotes inside preserved entity text", `p(title='&foo"&amp;')`, `<p title="&foo&quot;&amp;"></p>`},
		{"ampersands without semicolons", `p &&&&&`, `<p>&amp;&amp;&amp;&amp;&amp;</p>`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			document, err := xpug.Parse(tt.input)
			if err != nil {
				t.Fatal(err)
			}
			if got := document.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCustomDirectiveRenderers(t *testing.T) {
	parser := xpug.NewParser().AddDirective("when", func(ctx *xpug.RenderContext, node *xpug.Node) {
		ctx.WriteString("[")
		ctx.WriteString(node.Attr(""))
		ctx.WriteString("]")
		ctx.RenderChildren(node)
	})
	document, err := parser.Parse("when ready: span Ready")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := parser.Render(document, false), "[ready]<span>Ready</span>"; got != want {
		t.Errorf("custom parser output = %q, want %q", got, want)
	}
	if got := document.String(); got != "" {
		t.Errorf("default parser output = %q, want empty output for an unregistered directive", got)
	}
	other := xpug.NewParser()
	if got := other.Render(document, false); got != "" {
		t.Errorf("unregistered directive output = %q, want empty output", got)
	}
	other.AddDirective("when", nil)
	plain, err := other.Parse("when ready")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := other.Render(plain, false), "<when>ready</when>"; got != want {
		t.Errorf("disabled directive output = %q, want %q", got, want)
	}
}
