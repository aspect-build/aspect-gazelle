package tsbatch

import (
	"fmt"
	"sync"
	"testing"
	"unsafe"

	"github.com/aspect-build/aspect-gazelle/treesitter/golang"
	sitter "github.com/smacker/go-tree-sitter"
)

// sitter.Language is `struct { ptr unsafe.Pointer }`; recover the raw
// TSLanguage* the grammar binding wrapped. (Integration uses the same
// single-field read; smacker is version-pinned and patched in this repo.)
func langPtr(l *sitter.Language) unsafe.Pointer {
	return *(*unsafe.Pointer)(unsafe.Pointer(l))
}

func goLang() unsafe.Pointer { return langPtr(golang.NewLanguage()) }

// run compiles + parses + queries in one go, for tests that don't care about
// reuse.
func run(t *testing.T, src []byte, query string) []Match {
	t.Helper()
	q, err := Compile(goLang(), query)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	tree := Parse(goLang(), src)
	defer tree.Close()
	return RunQuery(q, tree)
}

// text returns the source slice for a capture.
func text(src []byte, c Capture) string { return string(src[c.StartByte:c.EndByte]) }

func TestSingleCapture(t *testing.T) {
	src := []byte("package main\n\nfunc foo() {}\nfunc bar() {}\n")
	matches := run(t, src, `(function_declaration name: (identifier) @fn)`)
	if len(matches) != 2 {
		t.Fatalf("got %d matches, want 2", len(matches))
	}
	if text(src, matches[0].Captures[0]) != "foo" || text(src, matches[1].Captures[0]) != "bar" {
		t.Fatalf("got %v", matches)
	}
}

// Two captures in one pattern must land in the same Match, in order.
func TestMultiCapturePerMatch(t *testing.T) {
	src := []byte("package main\n\nconst x = 1\nconst y = 2\n")
	matches := run(t, src,
		`(const_spec name: (identifier) @name value: (expression_list (int_literal) @val))`)
	if len(matches) != 2 {
		t.Fatalf("got %d matches, want 2", len(matches))
	}
	for _, m := range matches {
		if len(m.Captures) != 2 {
			t.Fatalf("match %+v: got %d captures, want 2", m, len(m.Captures))
		}
	}
	if n, v := text(src, matches[0].Captures[0]), text(src, matches[0].Captures[1]); n != "x" || v != "1" {
		t.Fatalf("match 0: got (%s,%s), want (x,1)", n, v)
	}
}

// Distinct patterns must report distinct pattern indices.
func TestMultiPattern(t *testing.T) {
	src := []byte("package main\n\nfunc foo() {}\ntype T struct{}\n")
	matches := run(t, src,
		`(function_declaration name: (identifier) @fn)
		 (type_spec name: (type_identifier) @ty)`)
	patterns := map[uint32]string{}
	for _, m := range matches {
		patterns[m.Pattern] = text(src, m.Captures[0])
	}
	if len(patterns) != 2 || patterns[0] != "foo" || patterns[1] != "T" {
		t.Fatalf("got %v, want {0:foo, 1:T}", patterns)
	}
}

// Byte offsets must be byte- not rune-based so source slicing is correct.
func TestUTF8ByteOffsets(t *testing.T) {
	src := []byte("package main\n\n// ééé\nfunc café() {}\n")
	matches := run(t, src, `(function_declaration name: (identifier) @fn)`)
	if len(matches) != 1 {
		t.Fatalf("got %d matches, want 1", len(matches))
	}
	if got := text(src, matches[0].Captures[0]); got != "café" {
		t.Fatalf("got %q, want %q", got, "café")
	}
}

func TestNoMatch(t *testing.T) {
	matches := run(t, []byte("package main\n"), `(function_declaration name: (identifier) @fn)`)
	if len(matches) != 0 {
		t.Fatalf("got %d matches, want 0", len(matches))
	}
}

func TestEmptySource(t *testing.T) {
	q, err := Compile(goLang(), `(function_declaration) @f`)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	tree := Parse(goLang(), nil)
	defer tree.Close()
	_ = RunQuery(q, tree)
}

func TestCompileError(t *testing.T) {
	_, err := Compile(goLang(), `(nonsense @`)
	if err == nil {
		t.Fatal("expected a query error")
	}
	if _, ok := err.(*QueryError); !ok {
		t.Fatalf("got %T, want *QueryError", err)
	}
}

// A compiled query is reusable across many trees (the caching contract).
func TestQueryReuseAcrossTrees(t *testing.T) {
	q, err := Compile(goLang(), `(function_declaration name: (identifier) @fn)`)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	for _, src := range [][]byte{
		[]byte("package main\nfunc a() {}\n"),
		[]byte("package main\nfunc b() {}\nfunc c() {}\n"),
	} {
		tree := Parse(goLang(), src)
		got := len(RunQuery(q, tree))
		tree.Close()
		want := 1
		if len(src) > 30 {
			want = 2
		}
		if got != want {
			t.Fatalf("src %q: got %d, want %d", src, got, want)
		}
	}
}

// Concurrent queries sharing one compiled query must be safe: each owns its
// cursor and tree, the query is immutable, no shared node cache.
// Run with --@io_bazel_rules_go//go/config:race.
func TestConcurrent(t *testing.T) {
	src := []byte("package main\n\nfunc foo() {}\nfunc bar() {}\nfunc baz() {}\n")
	q, err := Compile(goLang(), `(function_declaration name: (identifier) @fn)`)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}

	var wg sync.WaitGroup
	errs := make(chan error, 32)
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tree := Parse(goLang(), src)
			defer tree.Close()
			if n := len(RunQuery(q, tree)); n != 3 {
				errs <- fmt.Errorf("got %d matches, want 3", n)
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
}
