package gazelle

import (
	"testing"
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
