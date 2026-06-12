package node

import (
	"testing"
)

func TestIsNodeImport(t *testing.T) {
	builtins := []string{
		"fs",
		"fs/promises",
		"path",
		"assert",
		"assert/strict",
		"child_process",
		"util",
		// "node:" prefixed imports are always builtins
		"node:fs",
		"node:path",
		"node:test",
	}
	for _, m := range builtins {
		if !IsNodeImport(m) {
			t.Errorf("IsNodeImport(%q) = false, want true", m)
		}
	}

	nonBuiltins := []string{
		"",
		"react",
		"@scope/pkg",
		"./fs",
		"fs2",
		"lodash/fp",
	}
	for _, m := range nonBuiltins {
		if IsNodeImport(m) {
			t.Errorf("IsNodeImport(%q) = true, want false", m)
		}
	}
}
