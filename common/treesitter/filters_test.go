package treesitter_test

import (
	"maps"
	"slices"
	"strings"
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

func countMatches(ast treesitter.AST, q treesitter.TreeQuery) int {
	n := 0
	for range ast.Query(q) {
		n++
	}
	return n
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

// @a=["Outer","Inner"], @b=["Inner"] — mismatched lengths must reject.
func TestEqPredicate_quantifiedCaptureLengthMismatch(t *testing.T) {
	src := `package foo

func Outer() {
	Inner()
	Inner()
}
`
	ast := mustParseGo(t, src)
	q := mustQuery(t, `(function_declaration
		name: (identifier) @a
		body: (block (statement_list
			(expression_statement (call_expression function: (identifier) @a))
			(expression_statement (call_expression function: (identifier) @b))))
		(#eq? @a @b))`)

	got := collectCaptures(ast, q, "a")
	if len(got) != 0 {
		t.Errorf("expected no matches, got %v", got)
	}
}

// #eq? with a quantified capture: all bindings must equal the literal.
func TestEqPredicate_quantifiedVsLiteral(t *testing.T) {
	q := mustQuery(t, `(literal_value
		(keyed_element key: (literal_element (interpreted_string_literal) @k))
		(keyed_element key: (literal_element (interpreted_string_literal) @k))
		(#eq? @k "\"foo\""))`)

	allMatch := mustParseGo(t, `package foo
var _ = map[string]int{"foo": 1, "foo": 2}
`)
	if n := countMatches(allMatch, q); n != 1 {
		t.Errorf("all-match: got %d, want 1", n)
	}

	oneDiffers := mustParseGo(t, `package foo
var _ = map[string]int{"foo": 1, "bar": 2}
`)
	if n := countMatches(oneDiffers, q); n != 0 {
		t.Errorf("one-differs: got %d, want 0", n)
	}
}

// #match? with a quantified capture: all bindings must match the regex.
func TestMatchPredicate_quantifiedVsRegex(t *testing.T) {
	q := mustQuery(t, `(source_file
		(function_declaration name: (identifier) @n)
		(function_declaration name: (identifier) @n)
		(#match? @n "^[A-Z]"))`)

	allMatch := mustParseGo(t, `package foo
func Foo() {}
func Bar() {}
`)
	if n := countMatches(allMatch, q); n != 1 {
		t.Errorf("all-match: got %d, want 1", n)
	}

	oneDiffers := mustParseGo(t, `package foo
func Foo() {}
func bar() {}
`)
	if n := countMatches(oneDiffers, q); n != 0 {
		t.Errorf("one-differs: got %d, want 0", n)
	}
}

// @a=["foo","bar"], @b=["foo","bar"] — aligned pairs match (pairwise, not
// Cartesian: ("foo","bar") across keyed elements would fail).
func TestEqPredicate_quantifiedCapturePairwise(t *testing.T) {
	src := `package foo

var _ = map[string]string{
	"foo": "foo",
	"bar": "bar",
}
`
	ast := mustParseGo(t, src)
	q := mustQuery(t, `(literal_value
		(keyed_element
			key: (literal_element (interpreted_string_literal) @a)
			value: (literal_element (interpreted_string_literal) @b))
		(keyed_element
			key: (literal_element (interpreted_string_literal) @a)
			value: (literal_element (interpreted_string_literal) @b))
		(#eq? @a @b))`)

	var matches int
	for range ast.Query(q) {
		matches++
	}
	if matches != 1 {
		t.Errorf("expected 1 match, got %d", matches)
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

func TestQueryErrors_validSource(t *testing.T) {
	ast := mustParseGo(t, goFunctions)

	errs := ast.QueryErrors()
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestQueryErrors_errorFormat(t *testing.T) {
	ast := mustParseGo(t, `package foo

)
`)
	errs := ast.QueryErrors()
	if len(errs) == 0 {
		t.Fatal("expected parse errors, got none")
	}

	msg := errs[0].Error()
	lines := strings.SplitN(msg, "\n", 2)
	if len(lines) != 2 {
		t.Fatalf("expected two-line error, got: %q", msg)
	}
	if lines[0] != "     3: )" {
		t.Errorf("error line: got %q, want %q", lines[0], "     3: )")
	}
	if lines[1] != "        ^" {
		t.Errorf("caret line: got %q, want %q", lines[1], "        ^")
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

// When a query assigns the same capture name to two nodes in one match,
// mapQueryMatchCaptures keeps the last value (map assignment overwrites).
func TestCapturesMap_duplicateCaptureNameLastWins(t *testing.T) {
	ast := mustParseGo(t, `package foo
func Foo() {}
`)
	// @item captures the function name first, then the parameter list.
	q := mustQuery(t, `(function_declaration name: (identifier) @item parameters: (parameter_list) @item)`)

	var got []map[string]string
	for r := range ast.Query(q) {
		got = append(got, maps.Clone(r.Captures()))
	}

	if len(got) != 1 {
		t.Fatalf("expected 1 match, got %d", len(got))
	}
	if got[0]["item"] != "()" {
		t.Errorf("expected parameter list to win, got %q", got[0]["item"])
	}
}
