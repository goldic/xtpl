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
			_, err := ParseTextTemplate(strings.NewReader(source))
			require(t, err != nil, "expected an incomplete template error")
		})
	}
	_, err := ParseTemplate("text", "", "}}", nil)
	require(t, err != nil, "expected an empty delimiter error")
}

func TestParseFSReturnsErrors(t *testing.T) {
	_, err := ParseFS(fstest.MapFS{"bad.html": {Data: []byte(`{{#}}`)}})
	assertErrContains(t, err, "bad.html")
	sentinel := errors.New("open failed")
	_, err = ParseFS(failingFS{err: sentinel})
	assertErrIs(t, err, sentinel)
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
	assertNoErr(t, err)
	sentinel := errors.New("write failed")
	assertErrIs(t, tpls.Execute(failingWriter{sentinel}, nil), sentinel)
	missing, err := ParseTextTemplate(strings.NewReader(`{{.include('absent')}}`))
	assertNoErr(t, err)
	tpls.Add("missing", missing)
	assertErrContains(t, missing.Execute(io.Discard, nil), "absent")
	require(t, NewTemplates().Execute(io.Discard, nil) != nil, "expected a missing index error")
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
		t.Run(tc.source, func(t *testing.T) {
			got, err := Eval(tc.source, tc.value)
			assertNoErr(t, err)
			assertEqual(t, tc.want, got)
		})
	}
}

func TestOptionalChainingRemainsEmpty(t *testing.T) {
	for _, value := range []any{nil, map[string]any{}, map[string]any{"nilObject": (*User)(nil)}} {
		got, err := Eval(`{{nilObject.attr1.attr2}}`, value)
		assertNoErr(t, err)
		assertEqual(t, "", got)
	}
}

func TestParseFSNameCollisionOrder(t *testing.T) {
	tpls, err := ParseFS(fstest.MapFS{
		"index.html": {Data: []byte("html")},
		"index.pug":  {Data: []byte("p pug")},
	})
	assertNoErr(t, err)
	var out bytes.Buffer
	assertNoErr(t, tpls.Execute(&out, nil))
	assertEqual(t, "<p>pug</p>", out.String())
}
