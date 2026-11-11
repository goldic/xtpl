package xtpl

import (
	"fmt"
	"github.com/goldic/fsys"
	"io"
	"io/fs"
	"path/filepath"
	"strings"
)

// Templates is a collection of named templates. Configure it before concurrent use.
type Templates map[string]*Template

// NewTemplates creates an empty collection.
func NewTemplates() Templates {
	return make(Templates)
}

// Add registers t under name and its extensionless alias, replacing existing entries.
func (tpls Templates) Add(name string, t *Template) {
	if t == nil {
		return
	}
	name = normName(name)
	tpls[name] = t
	tpls[trimExt(name)] = t
	t.tpls = tpls
}

// Execute renders the index template or returns an error if it is missing.
func (tpls Templates) Execute(w io.Writer, v any) error {
	tpl := tpls.Lookup("/")
	if tpl == nil {
		return fmt.Errorf("xtpl: template %q not found", "/index")
	}
	return tpl.Execute(w, v)
}

// Lookup finds a named template, using index for names ending in a slash.
func (tpls Templates) Lookup(name string) *Template {
	name = normName(name)
	if strings.HasSuffix(name, "/") {
		name += "index"
	}
	if t := tpls[name]; t != nil {
		return t
	}
	return tpls[trimExt(name)]
}

// ParseDir loads templates recursively from dir using the fsys package.
func ParseDir(dir string) (_ Templates, err error) {
	defer catch(&err)

	return ParseFS(must(fsys.Dir(dir)))
}

// ParseFS loads recognized templates recursively in lexical path order. It
// returns the first filesystem or parsing error, including its path.
func ParseFS(fsys fs.FS) (tpls Templates, err error) {
	defer catch(&err)

	tpls = make(Templates)
	err = fs.WalkDir(fsys, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("xtpl: walk %q: %w", name, err)
		}
		if !d.IsDir() {
			tpl, err := parseFile(fsys, name)
			if err != nil {
				return fmt.Errorf("xtpl: parse %q: %w", name, err)
			}
			tpls.Add(name, tpl)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return
}

func parseFile(fsys fs.FS, path string) (tpl *Template, err error) {
	var parser func(r io.Reader) (tpl *Template, err error)
	switch filepath.Ext(path) {
	case ".pug", ".jade":
		parser = ParsePugTemplate
	case ".html", ".xtpl", ".xhtml":
		parser = ParseHTMLTemplate
	default:
		return
	}
	r, err := fsys.Open(path)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return parser(r)
}

func normName(s string) string {
	if !strings.HasPrefix(s, "/") {
		s = "/" + s
	}
	return s
}

func trimExt(s string) string {
	return strings.TrimSuffix(s, filepath.Ext(s))
}
