package gazelle

import (
	"testing"

	"github.com/bmatcuk/doublestar/v4"
)

func TestParseGlobExpressionVsDoublestar(t *testing.T) {
	// Ensure any shortcuts that ParseGlobExpression takes preserve the same behaviour
	// as running doublestar directly.
	// The results of the expression are not checked, only that any shortcuts ParseGlobExpression
	// adds still match the result of doublestar without those shortcuts.
	tests := map[string][]string{
		// Exact matches
		"file.txt":        {"file.txt", "./file.txt", "file", ".file", "file.", "a/file.txt"},
		"WORKSPACE":       {"WORKSPACE", "WORKSPACE.bazel", "a/WORKSPACE", "WORKSPACE.txt", "a/WORKSPACE.bazel"},
		"WORKSPACE.bazel": {"WORKSPACE", "WORKSPACE.bazel", "a/WORKSPACE", "WORKSPACE.txt", "a/WORKSPACE.bazel"},
		"@foo/bar":        {"@foo/bar/baz", "@foo/bar", "foo/bar", "a/@foo/bar"},
		"@foo/bar@1.2.3":  {"@foo/bar/baz@1.2.3", "@foo/bar@1.2.3", "foo/bar@1.2.3"},
		"@foo/*@1.2.3":    {"@foo/bar/baz@1.2.3", "@foo/bar@1.2.3", "foo/bar@1.2.3", "@foo/baz@1.2.3"},

		// Exact matches with paths
		"path/to/file.txt": {"path/to/file.txt", "a/path/to/file.txt", "path/to/file.txt2"},

		// Doublestar with prefix
		"src/**/*.go":     {"src/main.go", "src/deep/nested/file.go", "src/foo.go", "src/", "src/.go"},
		"src/foo/**/*.go": {"src/main.go", "src/foo/main.go", "src/foo/bar/main.go", "foo/src/main.go", "main.go", "src/foo/src/main.go"},

		// With prefix and suffix that are equal
		"foo/**/foo":          {"foo", "foo/foo", "foo/bar/foo", "foo/bar/NOTfoo", "foo/foo/foo"},
		"src/**/important.ts": {"important.ts", "NOTimportant.ts", "NOT.important.ts", "important.NOT.ts", "src/important.ts", "src/NOTimportant.ts", "src/NOT.important.ts", "src/important.NOT.ts"},

		// Body with doublestars
		"**/foo/**": {"foo/bar", "a/foo/baz", "a/b/c/foo/d/e", "foo", "a/b/c/foo", "foo/a/b/c"},

		// Trailing doublestar: matches the prefix directory itself as well as everything beneath it
		"/**":       {"", "a", "a/b/c"},
		"src/**":    {"src", "src/a", "src/a/b", "srcfoo", "src2", "asrc", "asrc/b", "s", ""},
		"assets/**": {"assets", "assets/x.png", "assets/a/b", "assetsfoo", "asset"},
		"a/b/**":    {"a/b", "a/b/c", "a/b/c/d", "a/bc", "a", "x/a/b"},

		// Trailing-slash globstar: `dir/**/` behaves like `dir/**`
		"foo/**/": {"foo", "foo/a", "foo/a/b", "foobar", "x/foo"},
		"src/**/": {"src", "src/a", "src/a/b", "srcfoo", "asrc"},

		// Glued globstar (no surrounding separator): doublestar treats `**` as a single `*`
		"**bar":    {"bar", "xbar", "a/xbar", "a/bar", "barx", "a/b/bar"},
		"a/**bar":  {"a/bar", "a/xbar", "a/b/bar", "bar", "a/barx", "a/b/xbar"},
		"foo**bar": {"foobar", "fooXbar", "foo/bar", "fooba", "xfoobar", "foo/Xbar"},
		"**.go":    {"foo.go", "a/foo.go", ".go", "main.go", "foogo", "a/.go"},
		"pre**":    {"pre", "prexyz", "pre/x", "prefix", "apre"},

		// Leading single star
		"*foo":  {"foo", "xfoo", "a/foo", "foox", "a/xfoo"},
		"*/foo": {"a/foo", "foo", "a/b/foo", "x/foo", "a/foox"},

		// Double / degenerate globstars
		"**/**":   {"", "a", "a/b", "a/b/c"},
		"a/**/**": {"a", "a/b", "a/b/c", "a/b/c/d/e", "x/a", "ab"},
		"**/a/**": {"a", "a/b", "x/a", "x/a/b", "ba", "a/b/c", "a/b/c/d", "x/y/a/b/c"},

		// Starting doublestars
		"**/WORKSPACE":       {"WORKSPACE", "notWORKSPACE", "notWORKSPACE.bazel", "WORKSPACE.bazel", "a/WORKSPACE", "a/notWORKSPACE", "WORKSPACE.txt", "a/WORKSPACE.bazel", "a/notWORKSPACE.bazel"},
		"**/WORKSPACE.bazel": {"WORKSPACE", "notWORKSPACE", "notWORKSPACE.bazel", "WORKSPACE.bazel", "a/WORKSPACE", "a/notWORKSPACE", "WORKSPACE.txt", "a/WORKSPACE.bazel", "a/notWORKSPACE.bazel"},
		"**/@foo/bar":        {"@foo/bar/baz", "@foo/bar", "foo/bar", "a/@foo/bar"},
		"**/*.go":            {"main.go", "src/main.go", "src/deep/nested/file.go"},
		"**/*_test.go":       {"src/test_file.go", "src/path/test_file.go", "deep/nested/test_file.go"},
		"**/*.pb.go":         {"generated.pb.go", "src/generated.pb.go"},
		"**/*.d.ts":          {"src/types.d.ts", "types.d.ts"},

		// Prefix without doublestars
		"src/*.go":              {"src/main.go", "main.go", "src/a/b/main.go", "foo/src/main.go"},
		"src/*/test_*.go":       {"src/path/test_file.go", "src/a/test_b/c.go", "src/test_file.go"},
		"**/*.test.js":          {"src/test.main.js"},
		"src/**/test_*.spec.ts": {"src/path/test_file.spec.ts", "src/test_foo.spec.ts"},
		"very/long/path/with/many/segments/file.go": {"very/long/path/with/many/segments/file.go"},
		"path/with/unicode/测试文件.txt":                {"path/with/unicode/测试文件.txt"},

		// Odd cases
		"":     {""},
		"**":   {"", "a", "a/b/c"},
		"**/*": {"", "a", "a.b", "a/b/c", "a/b/c.d"},
	}

	for testPattern, testCases := range tests {
		expr := parseGlobExpression(testPattern)
		expr2, err := parseGlobExpressions([]string{testPattern})

		// Verify doublestar agrees on validity
		if (err == nil) != doublestar.ValidatePattern(testPattern) {
			t.Errorf("ParseGlobExpression(%q) returned error %v and doublestar returned the opposite", testPattern, err)
		}

		// Verify matching behaviour
		for _, c := range testCases {
			if expr(c) != doublestar.MatchUnvalidated(testPattern, c) {
				t.Errorf("pattern %q did not align with doublestar with case %q", testPattern, c)
			}

			if expr(c) != expr2(c) {
				t.Errorf("pattern %q did not align between ParseGlobExpression(s) with case %q", testPattern, c)
			}
		}
	}
}

// TestParseGlobExpressionsMulti covers multi-pattern lists spanning the
// different pattern buckets (exact, prefix-dir, pre+post, prefix, postfix,
// generic), cross-checking the combined matcher against doublestar.
func TestParseGlobExpressionsMulti(t *testing.T) {
	tests := []struct {
		name     string
		patterns []string
		paths    []string
	}{
		{
			name:     "exact and prefix-dir",
			patterns: []string{"BUILD", "src/**"},
			paths:    []string{"BUILD", "BUILD.bazel", "src", "src/a.ts", "src/a/b.ts", "other/BUILD"},
		},
		{
			name:     "all buckets",
			patterns: []string{"BUILD", "gen/**", "**/*.ts", "lib/**/*.go", "src/*/main.go", "{src,lib}/file.txt", "**/{a,b}/**"},
			paths: []string{
				"BUILD", "gen", "gen/x", "gen/x/y.go",
				"a.ts", "src/a.ts", "lib/a.go", "lib/x/a.go",
				"src/x/main.go", "src/main.go", "src/x/y/main.go",
				"src/file.txt", "lib/file.txt", "other/file.txt",
				"a/x", "x/b/y", "x/c/y",
			},
		},
		{
			name:     "overlapping patterns",
			patterns: []string{"**/*.spec.ts", "**/*.ts", "src/**"},
			paths:    []string{"a.ts", "a.spec.ts", "src/a.js", "other/a.js"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			expr, err := ParseGlobExpressions(tc.patterns)
			if err != nil {
				t.Fatalf("ParseGlobExpressions(%q) returned error %v", tc.patterns, err)
			}
			for _, p := range tc.paths {
				want := false
				for _, pattern := range tc.patterns {
					if doublestar.MatchUnvalidated(pattern, p) {
						want = true
						break
					}
				}
				if got := expr(p); got != want {
					t.Errorf("patterns %q: match(%q) = %v, doublestar says %v", tc.patterns, p, got, want)
				}
			}
		})
	}
}

// "," is a literal in glob patterns, so pattern lists differing only in where
// a "," falls must not share a cache entry.
func TestParseGlobExpressionsCacheKeyCollision(t *testing.T) {
	m1, err := ParseGlobExpressions([]string{"a,b", "c"})
	if err != nil {
		t.Fatal(err)
	}
	m2, err := ParseGlobExpressions([]string{"a", "b,c"})
	if err != nil {
		t.Fatal(err)
	}

	for path, want := range map[string]bool{"a,b": true, "c": true, "a": false, "b,c": false} {
		if got := m1(path); got != want {
			t.Errorf("m1(%q) = %v, want %v", path, got, want)
		}
	}
	for path, want := range map[string]bool{"a": true, "b,c": true, "a,b": false, "c": false} {
		if got := m2(path); got != want {
			t.Errorf("m2(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestParseGlobExpressionsEmpty(t *testing.T) {
	if _, err := ParseGlobExpressions(nil); err == nil {
		t.Error("ParseGlobExpressions(nil) should return an error")
	}
	if _, err := ParseGlobExpressions([]string{}); err == nil {
		t.Error("ParseGlobExpressions([]) should return an error")
	}
	if _, err := ParseGlobExpressionsWithExcludes(nil, nil); err == nil {
		t.Error("ParseGlobExpressionsWithExcludes(nil, nil) should return an error")
	}
}

// TestParseGlobExpressionsWithExcludes covers include/exclude combination.
// doublestar cannot express negation, so these use an explicit expectation
// table rather than cross-checking against doublestar.
func TestParseGlobExpressionsWithExcludes(t *testing.T) {
	tests := []struct {
		name     string
		includes []string
		excludes []string
		matches  map[string]bool
	}{
		{
			name:     "no excludes behaves like includes only",
			includes: []string{"src/**/*.ts"},
			excludes: nil,
			matches: map[string]bool{
				"src/foo.ts":      true,
				"src/foo.spec.ts": true,
				"other/foo.ts":    false,
			},
		},
		{
			name:     "single exclude",
			includes: []string{"src/**/*.ts"},
			excludes: []string{"src/**/*.spec.ts"},
			matches: map[string]bool{
				"src/foo.ts":           true,
				"src/deep/bar.ts":      true,
				"src/foo.spec.ts":      false,
				"src/deep/bar.spec.ts": false,
				"other/foo.ts":         false, // not in includes
			},
		},
		{
			name:     "multiple excludes",
			includes: []string{"src/**/*.ts"},
			excludes: []string{"**/*.spec.ts", "**/*.d.ts", "src/gen/**"},
			matches: map[string]bool{
				"src/foo.ts":        true,
				"src/foo.spec.ts":   false,
				"src/types.d.ts":    false,
				"src/gen/x.ts":      false,
				"src/gen/deep/y.ts": false,
				"src/keep/z.ts":     true,
			},
		},
		{
			name:     "multiple includes with exclude",
			includes: []string{"src/**/*.ts", "src/**/*.tsx"},
			excludes: []string{"**/*.spec.ts"},
			matches: map[string]bool{
				"src/foo.ts":      true,
				"src/foo.tsx":     true,
				"src/foo.spec.ts": false,
			},
		},
		{
			name:     "excludes only matches everything else",
			includes: nil,
			excludes: []string{"**/*.spec.ts"},
			matches: map[string]bool{
				"foo.ts":          true,
				"a/b/c.go":        true,
				"foo.spec.ts":     false,
				"a/b/foo.spec.ts": false,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			expr, err := ParseGlobExpressionsWithExcludes(tc.includes, tc.excludes)
			if err != nil {
				t.Fatalf("ParseGlobExpressionsWithExcludes(%q, %q) returned error %v", tc.includes, tc.excludes, err)
			}
			for path, want := range tc.matches {
				if got := expr(path); got != want {
					t.Errorf("includes=%q excludes=%q: match(%q) = %v, want %v", tc.includes, tc.excludes, path, got, want)
				}
			}
		})
	}
}
