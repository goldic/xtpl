package xtpl

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"
)

func TestRejectsIncompleteTemplates(t *testing.T) {
	for _, source := range []string{
		`{{if .}}hello`, `{{range .}}hello`, `{{if .}}{{range .}}{{end}}`,
		`{{.`, `{{'abc}}`, `{{.name)}}`, `{{/* comment}}`,
	} {
		t.Run(source, func(t *testing.T) {
			if _, err := ParseTextTemplate(strings.NewReader(source)); err == nil {
				t.Fatalf("accepted incomplete template %q", source)
			}
		})
	}
	if _, err := ParseTemplate("text", "", "}}", nil); err == nil {
		t.Fatal("accepted an empty delimiter")
	}
}

func TestParseFSReturnsErrors(t *testing.T) {
	if _, err := ParseFS(fstest.MapFS{"bad.html": {Data: []byte(`{{#}}`)}}); err == nil || !strings.Contains(err.Error(), "bad.html") {
		t.Fatalf("expected an error naming bad.html, got %v", err)
	}
	sentinel := errors.New("open failed")
	if _, err := ParseFS(failingFS{err: sentinel}); !errors.Is(err, sentinel) {
		t.Fatalf("expected original filesystem error, got %v", err)
	}
}

type failingFS struct{ err error }

func (f failingFS) Open(string) (fs.File, error) { return nil, f.err }

type failingWriter struct{ err error }

func (w failingWriter) Write([]byte) (int, error) { return 0, w.err }

func TestMissingAndFailedIncludes(t *testing.T) {
	tpls, err := ParseFS(fstest.MapFS{
		"index.html": {Data: []byte(`{{.include('child')}}`)},
		"child.html": {Data: []byte(`hello`)},
	})
	if err != nil {
		t.Fatal(err)
	}
	sentinel := errors.New("write failed")
	if err := tpls.Execute(failingWriter{sentinel}, nil); !errors.Is(err, sentinel) {
		t.Fatalf("expected nested writer error, got %v", err)
	}
	missing, err := ParseTextTemplate(strings.NewReader(`{{.include('absent')}}`))
	if err != nil {
		t.Fatal(err)
	}
	tpls.Add("missing", missing)
	if err := missing.Execute(io.Discard, nil); err == nil || !strings.Contains(err.Error(), "absent") {
		t.Fatalf("expected missing include error, got %v", err)
	}
	if err := NewTemplates().Execute(io.Discard, nil); err == nil {
		t.Fatal("expected missing index error")
	}
}

func TestCacheAndSubstringBoundaries(t *testing.T) {
	for _, tc := range []struct {
		source string
		value  any
		want   string
	}{
		{`{{.cache}}ok`, nil, "ok"},
		{`{{.substr(0)}}`, "", ""},
		{`{{.substr(9)}}`, "abc", ""},
		{`{{.substr(1,-1)}}`, "abc", ""},
		{`{{.substr(-2)}}`, "abc", "bc"},
		{`{{.index(4)}}`, []int{1}, ""},
		{`{{.index(-1)}}`, []int{1}, ""},
	} {
		got, err := Eval(tc.source, tc.value)
		if err != nil || got != tc.want {
			t.Errorf("%s: got %q, %v; want %q", tc.source, got, err, tc.want)
		}
	}
}

func TestOptionalChainingRemainsEmpty(t *testing.T) {
	for _, value := range []any{nil, map[string]any{}, map[string]any{"nilObject": (*User)(nil)}} {
		got, err := Eval(`{{nilObject.attr1.attr2}}`, value)
		if err != nil || got != "" {
			t.Fatalf("got %q, %v", got, err)
		}
	}
}

func TestParseFSNameCollisionOrder(t *testing.T) {
	tpls, err := ParseFS(fstest.MapFS{
		"index.html": {Data: []byte("html")},
		"index.pug":  {Data: []byte("p pug")},
	})
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := tpls.Execute(&out, nil); err != nil {
		t.Fatal(err)
	}
	if out.String() != "<p>pug</p>" {
		t.Fatalf("unexpected selected index: %q", out.String())
	}
}
