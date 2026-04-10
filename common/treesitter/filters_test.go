package treesitter_test

import (
	"maps"
	"slices"
	"testing"

	"github.com/aspect-build/aspect-gazelle/common/treesitter"
	golang "github.com/aspect-build/aspect-gazelle/common/treesitter/grammars/golang"
)

var goLang = golang.NewLanguage()

func mustParseGo(t *testing.T, src string) treesitter.AST {
	t.Helper()
	ast, err := treesitter.ParseSourceCode(goLang, "test.go", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(ast.Close)
	return ast
}

func mustQuery(t *testing.T, queryStr string) treesitter.TreeQuery {
	t.Helper()
	q, err := treesitter.GetQuery(goLang, queryStr)
	if err != nil {
		t.Fatal(err)
	}
	return q
}

// collectCaptures runs the query and returns all values of the named capture.
func collectCaptures(ast treesitter.AST, q treesitter.TreeQuery, capture string) []string {
	var results []string
	for r := range ast.Query(q) {
		if v, ok := r.Captures()[capture]; ok {
			results = append(results, v)
		}
	}
	return results
}

const goFunctions = `package foo

func Foo() {}
func Bar() {}
func baz() {}
`

const goKeyValuePairs = `package foo

var _ = map[string]string{
	"same":  "same",
	"left":  "right",
}
`

func TestNoPredicates(t *testing.T) {
	ast := mustParseGo(t, goFunctions)
	q := mustQuery(t, `(function_declaration name: (identifier) @name)`)

	got := collectCaptures(ast, q, "name")
	want := []string{"Foo", "Bar", "baz"}
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestEqPredicateCaptureVsLiteral_match(t *testing.T) {
	ast := mustParseGo(t, goFunctions)
	q := mustQuery(t, `(function_declaration name: (identifier) @name (#eq? @name "Foo"))`)

	got := collectCaptures(ast, q, "name")
	want := []string{"Foo"}
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestEqPredicateCaptureVsLiteral_noMatch(t *testing.T) {
	ast := mustParseGo(t, goFunctions)
	q := mustQuery(t, `(function_declaration name: (identifier) @name (#eq? @name "Missing"))`)

	got := collectCaptures(ast, q, "name")
	if len(got) != 0 {
		t.Errorf("expected no matches, got %v", got)
	}
}

func TestNotEqPredicateCaptureVsLiteral(t *testing.T) {
	ast := mustParseGo(t, goFunctions)
	q := mustQuery(t, `(function_declaration name: (identifier) @name (#not-eq? @name "Foo"))`)

	got := collectCaptures(ast, q, "name")
	want := []string{"Bar", "baz"}
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestEqPredicateCaptureVsCapture_match(t *testing.T) {
	ast := mustParseGo(t, goKeyValuePairs)
	q := mustQuery(t, `(keyed_element key: (literal_element (interpreted_string_literal) @key) value: (literal_element (interpreted_string_literal) @value) (#eq? @key @value))`)

	got := collectCaptures(ast, q, "key")
	want := []string{`"same"`}
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestNotEqPredicateCaptureVsCapture_match(t *testing.T) {
	ast := mustParseGo(t, goKeyValuePairs)
	q := mustQuery(t, `(keyed_element key: (literal_element (interpreted_string_literal) @key) value: (literal_element (interpreted_string_literal) @value) (#not-eq? @key @value))`)

	got := collectCaptures(ast, q, "key")
	want := []string{`"left"`}
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestMatchPredicate_match(t *testing.T) {
	ast := mustParseGo(t, goFunctions)
	// Only exported (capitalized) function names
	q := mustQuery(t, `(function_declaration name: (identifier) @name (#match? @name "^[A-Z]"))`)

	got := collectCaptures(ast, q, "name")
	want := []string{"Foo", "Bar"}
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestMatchPredicate_noMatch(t *testing.T) {
	ast := mustParseGo(t, goFunctions)
	q := mustQuery(t, `(function_declaration name: (identifier) @name (#match? @name "^[0-9]"))`)

	got := collectCaptures(ast, q, "name")
	if len(got) != 0 {
		t.Errorf("expected no matches, got %v", got)
	}
}

func TestNotMatchPredicate(t *testing.T) {
	ast := mustParseGo(t, goFunctions)
	// Unexported (lowercase) function names only
	q := mustQuery(t, `(function_declaration name: (identifier) @name (#not-match? @name "^[A-Z]"))`)

	got := collectCaptures(ast, q, "name")
	want := []string{"baz"}
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestMultiplePredicates_allMustPass(t *testing.T) {
	ast := mustParseGo(t, goFunctions)
	// eq? AND match? — only "Foo" satisfies both
	q := mustQuery(t, `(function_declaration name: (identifier) @name
		(#match? @name "^[A-Z]")
		(#not-eq? @name "Bar"))`)

	got := collectCaptures(ast, q, "name")
	want := []string{"Foo"}
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestCapturesMap_allCapturesPresent(t *testing.T) {
	ast := mustParseGo(t, goFunctions)
	q := mustQuery(t, `(function_declaration name: (identifier) @name) @func`)

	var got []map[string]string
	for r := range ast.Query(q) {
		got = append(got, maps.Clone(r.Captures()))
	}

	if len(got) != 3 {
		t.Fatalf("expected 3 matches, got %d", len(got))
	}
	for _, m := range got {
		if _, ok := m["name"]; !ok {
			t.Errorf("missing @name capture in %v", m)
		}
		if _, ok := m["func"]; !ok {
			t.Errorf("missing @func capture in %v", m)
		}
	}
}
