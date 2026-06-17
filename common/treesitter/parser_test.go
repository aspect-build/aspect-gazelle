package treesitter

import (
	"slices"
	"testing"

	golang "github.com/aspect-build/aspect-gazelle/treesitter/golang"
)

// These are general parser-level tests (parse -> query -> close lifecycle,
// language mapping). The exhaustive predicate-filtering matrix (#eq?, #match?,
// quantified captures, etc.) lives in filters_test.go.

var goGrammar = NewLanguage(Go, golang.LanguagePtr())

func parseGo(t *testing.T, src string) AST {
	t.Helper()
	ast, err := ParseSourceCode(goGrammar, "test.go", []byte(src))
	if err != nil {
		t.Fatalf("ParseSourceCode: %v", err)
	}
	t.Cleanup(ast.Close)
	return ast
}

const goSource = `package foo

func Foo() {}
func Bar() {}
func baz() {}
`

// lookupPathLanguage must never panic, including on paths with no extension or
// dotfiles, where path.Ext returns "" or the whole base name respectively.
// Regression test for an `ext[1:]` slice that panicked on an empty extension.
func TestLookupPathLanguage(t *testing.T) {
	tests := []struct {
		path      string
		wantLang  LanguageGrammar
		wantFound bool
	}{
		{"foo.go", Go, true},
		{"a/b/c.tsx", TypescriptX, true},
		{"foo.json", JSON, true},

		// Previously panicked: no extension at all.
		{"Makefile", "", false},
		{"a/b/LICENSE", "", false},
		{"", "", false},

		// Dotfile: path.Ext returns the whole base (".gitignore"), which is not a
		// known extension. Must report not-found, not panic.
		{".gitignore", "", false},
		{"a/b/.env", "", false},

		// Unknown but well-formed extension.
		{"styles.css", "", false},
	}

	for _, tc := range tests {
		gotLang, gotFound := lookupPathLanguage(tc.path)
		if gotLang != tc.wantLang || gotFound != tc.wantFound {
			t.Errorf("lookupPathLanguage(%q) = (%q, %v), want (%q, %v)",
				tc.path, gotLang, gotFound, tc.wantLang, tc.wantFound)
		}
	}
}

// PathToLanguage returns the mapped grammar for a recognized extension.
func TestPathToLanguage_known(t *testing.T) {
	if got := PathToLanguage("a/b/main.go"); got != Go {
		t.Errorf("PathToLanguage = %q, want %q", got, Go)
	}
}

// NewLanguage round-trips the grammar it was constructed with.
func TestNewLanguage_grammarRoundTrip(t *testing.T) {
	if got := goGrammar.Grammar(); got != Go {
		t.Errorf("Grammar() = %q, want %q", got, Go)
	}
}

// A full parse -> query -> capture pass returns the expected captures, and the
// produced AST reports no parse errors for valid source.
func TestParseSourceCode_queryLifecycle(t *testing.T) {
	ast := parseGo(t, goSource)

	if errs := ast.QueryErrors(); len(errs) != 0 {
		t.Fatalf("QueryErrors on valid source: %v", errs)
	}

	q, err := GetQuery(goGrammar, `(function_declaration name: (identifier) @name)`)
	if err != nil {
		t.Fatalf("GetQuery: %v", err)
	}

	var names []string
	for captures := range ast.Query(q) {
		names = append(names, captures["name"])
	}
	if want := []string{"Foo", "Bar", "baz"}; !slices.Equal(names, want) {
		t.Errorf("captured names = %v, want %v", names, want)
	}
}

// A query carrying a filtering predicate only yields matches that pass it,
// exercising the predicate path through Query at the parser level.
func TestParseSourceCode_filteringQuery(t *testing.T) {
	ast := parseGo(t, goSource)

	// #match? keeps only exported (capitalized) function names.
	q, err := GetQuery(goGrammar, `(function_declaration name: (identifier) @name (#match? @name "^[A-Z]"))`)
	if err != nil {
		t.Fatalf("GetQuery: %v", err)
	}

	var names []string
	for captures := range ast.Query(q) {
		names = append(names, captures["name"])
	}
	if want := []string{"Foo", "Bar"}; !slices.Equal(names, want) {
		t.Errorf("filtered names = %v, want %v", names, want)
	}
}

// GetQuery surfaces a compile error for a malformed query string.
func TestGetQuery_invalidQuery(t *testing.T) {
	if _, err := GetQuery(goGrammar, `(function_declaration`); err == nil {
		t.Error("expected an error for a malformed query, got nil")
	}
}

// QueryErrors reports the syntax errors of an invalid parse.
func TestQueryErrors_invalidSource(t *testing.T) {
	ast := parseGo(t, "package foo\n\n)\n")
	if errs := ast.QueryErrors(); len(errs) == 0 {
		t.Error("expected parse errors for invalid source, got none")
	}
}

// Close must be safe to call more than once.
func TestAST_CloseIdempotent(t *testing.T) {
	ast, err := ParseSourceCode(goGrammar, "test.go", []byte(goSource))
	if err != nil {
		t.Fatalf("ParseSourceCode: %v", err)
	}
	ast.Close()
	ast.Close() // must not panic or double-send the tree for deletion
}
