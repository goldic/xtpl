package xtpl

import (
	"errors"
	"io/fs"
	"reflect"
	"strings"
	"testing"
)

func require(t *testing.T, ok bool, message ...any) {
	t.Helper()
	if !ok {
		if len(message) > 0 {
			t.Fatal(message...)
		}
		t.Fatal("requirement failed")
	}
}

func assertEqual(t *testing.T, expected, actual any) {
	t.Helper()
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("expected: %#v\nactual:   %#v", expected, actual)
	}
}

func assertNoErr(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func assertErrIs(t *testing.T, err, expected error) {
	t.Helper()
	if !errors.Is(err, expected) {
		t.Fatalf("expected error %v, got %v", expected, err)
	}
}

func assertErrContains(t *testing.T, err error, expected string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), expected) {
		t.Fatalf("expected error containing %q, got %v", expected, err)
	}
}

func mustReadFile(filesystem fs.FS, name string) string {
	return string(must(fs.ReadFile(filesystem, name)))
}
