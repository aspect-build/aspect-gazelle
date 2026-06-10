package treesitter

import (
	"testing"

	"github.com/aspect-build/aspect-gazelle/treesitter/golang"
)

func goLanguage() Language {
	return NewLanguageFromSitter(Go, golang.NewLanguage())
}

func TestQueryBatch(t *testing.T) {
	src := []byte("package main\n\nfunc foo() {}\nfunc bar() {}\n")

	res, err := QueryBatch(goLanguage(), src, map[string]string{
		"fns": `(function_declaration name: (identifier) @fn)`,
	})
	if err != nil {
		t.Fatalf("QueryBatch: %v", err)
	}
	matches := res["fns"]
	if len(matches) != 2 {
		t.Fatalf("got %d matches, want 2", len(matches))
	}
	if matches[0]["fn"] != "foo" || matches[1]["fn"] != "bar" {
		t.Fatalf("got %v, want foo/bar", matches)
	}
}

// Predicate filtering must apply (matchesAllPredicatesBatch parity).
func TestQueryBatchPredicate(t *testing.T) {
	src := []byte("package main\n\nfunc foo() {}\nfunc bar() {}\n")

	res, err := QueryBatch(goLanguage(), src, map[string]string{
		"named_foo": `((function_declaration name: (identifier) @fn) (#eq? @fn "foo"))`,
	})
	if err != nil {
		t.Fatalf("QueryBatch: %v", err)
	}
	matches := res["named_foo"]
	if len(matches) != 1 {
		t.Fatalf("got %d matches, want 1 (only foo)", len(matches))
	}
	if matches[0]["fn"] != "foo" {
		t.Fatalf("got %q, want foo", matches[0]["fn"])
	}
}

func TestQueryBatchMatchPredicate(t *testing.T) {
	src := []byte("package main\n\nfunc Exported() {}\nfunc unexported() {}\n")

	res, err := QueryBatch(goLanguage(), src, map[string]string{
		"exported": `((function_declaration name: (identifier) @fn) (#match? @fn "^[A-Z]"))`,
	})
	if err != nil {
		t.Fatalf("QueryBatch: %v", err)
	}
	if matches := res["exported"]; len(matches) != 1 || matches[0]["fn"] != "Exported" {
		t.Fatalf("got %v, want only Exported", res["exported"])
	}
}
