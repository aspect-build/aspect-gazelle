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
