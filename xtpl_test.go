package xtpl

import (
	"reflect"
	"testing"
)

func assertEqual(t *testing.T, expected, actual any) {
	t.Helper()
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("expected: %#v\nactual:   %#v", expected, actual)
	}
}

func require(t *testing.T, ok bool) {
	t.Helper()
	if !ok {
		t.Fatal("requirement failed")
	}
}
