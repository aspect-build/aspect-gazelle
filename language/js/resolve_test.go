package gazelle

import (
	"testing"

	"github.com/bazelbuild/bazel-gazelle/resolve"
)

func TestToOutDirPath(t *testing.T) {
	eq := func(got, want, msg string) {
		t.Helper()
		if got != want {
			t.Errorf("%s\n\tactual:   %s\n\texpected: %s", msg, got, want)
		}
	}

	t.Run("no outDir", func(t *testing.T) {
		eq(toOutDirPath("", "", "foo.ts"), "foo.ts", "empty config")
		eq(toOutDirPath(".", "", "foo.ts"), "foo.ts", "dot rootDir")
	})

	t.Run("rootDir only strips prefix", func(t *testing.T) {
		eq(toOutDirPath("src", "", "src/foo.ts"), "foo.ts", "in rootDir")
		eq(toOutDirPath("src", "", "src"), "src", "exact rootDir match — no strip")
		eq(toOutDirPath("src", "", "src.ts"), "src.ts", "not in rootDir")
		eq(toOutDirPath("src", "", "src-other/src.ts"), "src-other/src.ts", "similar prefix not stripped")
	})

	t.Run("outDir prepended", func(t *testing.T) {
		eq(toOutDirPath("", "dist", "foo.ts"), "dist/foo.ts", "outDir only")
		eq(toOutDirPath(".", "dist", "foo.ts"), "dist/foo.ts", "dot rootDir + outDir")
	})

	t.Run("rootDir + outDir", func(t *testing.T) {
		eq(toOutDirPath("src", "dist", "src/foo.ts"), "dist/foo.ts", "in rootDir")
		eq(toOutDirPath("src", "dist", "src.ts"), "dist/src.ts", "not in rootDir")
		eq(toOutDirPath("src", "dist", "src-other/src.ts"), "dist/src-other/src.ts", "similar prefix not stripped")
		eq(toOutDirPath("src", "dist", "src"), "dist/src", "exact rootDir match — no strip")
	})

	t.Run("declarationDir root (dot outDir with rootDir)", func(t *testing.T) {
		eq(toOutDirPath("src", ".", "src/foo.ts"), "foo.ts", "in rootDir, dot outDir")
		eq(toOutDirPath("src", ".", "src.ts"), "src.ts", "not in rootDir, dot outDir")
		eq(toOutDirPath("src", ".", "src-other/src.ts"), "src-other/src.ts", "similar prefix, dot outDir")
		eq(toOutDirPath("src", ".", "src"), "src", "exact rootDir match, dot outDir")
	})

	t.Run("dot outDir no-op without rootDir", func(t *testing.T) {
		eq(toOutDirPath(".", ".", "foo.ts"), "foo.ts", "both dot — unchanged")
		eq(toOutDirPath(".", ".", "a/b.ts"), "a/b.ts", "both dot, nested — unchanged")
	})

}

func TestToEmittedExtensionImport(t *testing.T) {
	for _, tc := range []struct {
		importPath, imp string
		expected        string
		mapped          bool
	}{
		{"./lib.ts", "pkg/lib.ts", "pkg/lib.js", true},
		{"../lib.ts", "lib.ts", "lib.js", true},
		{"./lib.tsx", "pkg/lib.tsx", "pkg/lib.js", true},
		{"./lib.jsx", "pkg/lib.jsx", "pkg/lib.js", true},
		{"./lib.mts", "pkg/lib.mts", "pkg/lib.mjs", true},
		{"./lib.cts", "pkg/lib.cts", "pkg/lib.cjs", true},
		{"./a.b.ts", "pkg/a.b.ts", "pkg/a.b.js", true},
		// Not mapped
		{"./lib", "pkg/lib", "", false},
		{"./lib.js", "pkg/lib.js", "", false},
		{"./lib.d.ts", "pkg/lib.d.ts", "", false},
		{"./lib.d.mts", "pkg/lib.d.mts", "", false},
		{"./lib.json", "pkg/lib.json", "", false},
		{"./lib.TS", "pkg/lib.TS", "", false},
		// Only relative specifiers are mapped
		{"lib.ts", "lib.ts", "", false},
		{"@scope/pkg/lib.ts", "@scope/pkg/lib.ts", "", false},
		{"/lib.ts", "lib.ts", "", false},
	} {
		t.Run(tc.importPath, func(t *testing.T) {
			in := ImportStatement{
				ImportSpec: resolve.ImportSpec{Lang: LanguageName, Imp: tc.imp},
				ImportPath: tc.importPath,
			}

			out, mapped := toEmittedExtensionImport(in)
			if mapped != tc.mapped {
				t.Fatalf("toEmittedExtensionImport(%q): mapped = %v, want %v", tc.importPath, mapped, tc.mapped)
			}
			if mapped && out.Imp != tc.expected {
				t.Errorf("toEmittedExtensionImport(%q): Imp = %q, want %q", tc.importPath, out.Imp, tc.expected)
			}
			if out.ImportPath != tc.importPath {
				t.Errorf("toEmittedExtensionImport(%q): ImportPath changed to %q", tc.importPath, out.ImportPath)
			}
		})
	}
}
