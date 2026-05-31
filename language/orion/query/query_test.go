package query

import "testing"

func TestQueryGoImports(t *testing.T) {
	src := []byte("package main\nimport \"fmt\"\n")
	results, parseErrs, err := Query(Go, "a.go", src, []string{
		`(import_spec (interpreted_string_literal) @path)`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(parseErrs) != 0 {
		t.Fatalf("unexpected parse errors: %v", parseErrs)
	}
	if len(results) != 1 {
		t.Fatalf("want 1 query result, got %d", len(results))
	}
	if len(results[0]) != 1 {
		t.Fatalf("want 1 match, got %d", len(results[0]))
	}
	if got := results[0][0]["path"]; got != `"fmt"` {
		t.Fatalf("path capture = %q, want %q", got, `"fmt"`)
	}
}

func TestQueryBatchOrderAndEmpty(t *testing.T) {
	src := []byte("package main\nimport \"fmt\"\n")
	results, _, err := Query(Go, "a.go", src, []string{
		`(import_spec (interpreted_string_literal) @path)`,
		`(method_declaration name: (field_identifier) @m)`, // matches nothing
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("want 2 results (one per query), got %d", len(results))
	}
	if len(results[1]) != 0 {
		t.Fatalf("want 0 matches for second query, got %d", len(results[1]))
	}
}

func TestQueryStarlarkPredicate(t *testing.T) {
	// #eq? text predicate must be applied: only the load() call matches.
	src := []byte("load(\"//f:b.bzl\", \"s\")\nother(\"a\", \"b\")\n")
	q := `(module (expression_statement (call
		function: (identifier) @id
		arguments: (argument_list (string) @path (string)))
		(#eq? @id "load")))`
	results, _, err := Query(Starlark, "x.bzl", src, []string{q})
	if err != nil {
		t.Fatal(err)
	}
	if len(results[0]) != 1 {
		t.Fatalf("want 1 load match, got %d", len(results[0]))
	}
	if got := results[0][0]["id"]; got != "load" {
		t.Fatalf("id capture = %q, want \"load\"", got)
	}
}

func TestQueryInvalidQueryIsError(t *testing.T) {
	// A query referencing an unknown node type is a hard error (plugin bug),
	// matching the previous go-tree-sitter message.
	_, _, err := Query(Go, "a.go", []byte("package main\n"), []string{"(import_) @x"})
	if err == nil {
		t.Fatal("expected an error for an invalid query")
	}
	if got := err.Error(); got != "invalid node type 'import_' at line 1 column 1" {
		t.Fatalf("error = %q", got)
	}
}

func TestPathToGrammar(t *testing.T) {
	cases := map[string]Grammar{
		"src/a.ts":  Typescript,
		"src/a.tsx": TypescriptX,
		"x.kt":      Kotlin,
		"BUILD.bzl": Starlark,
		"main.go":   Go,
		"noext":     "",
		"weird.xyz": "",
	}
	for path, want := range cases {
		if got := PathToGrammar(path); got != want {
			t.Errorf("PathToGrammar(%q) = %q, want %q", path, got, want)
		}
	}
}
