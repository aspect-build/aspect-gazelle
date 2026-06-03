package gazelle

import (
	"regexp"
	"testing"
)

func TestParseMatcherVsRegexp(t *testing.T) {
	// Ensure the substring shortcut ParseMatcher takes for patterns with no
	// regex metacharacters preserves the same behaviour as running the regex
	// directly. The result of the match is not checked, only that the shortcut
	// still agrees with regexp.Regexp.Match (mirrors glob_test.go).
	tests := map[string][]string{
		// Literal patterns -> substring shortcut.
		"matcher":   {"test-data", "matcher", "a matcher here", "match", "MATCHER", ""},
		"import":    {"import x", "important", "no match here", "", "  import "},
		"test-data": {"test-data", "a test-data b", "testdata", "test-dat", "TEST-DATA"},

		// Patterns with metacharacters -> regex path.
		"a.c":     {"abc", "axc", "ac", "a\nc", "xabcx"},
		"foo|bar": {"foo", "bar", "baz", "foobar", ""},
		"^pkg":    {"pkg x", "x pkg", "pkg", "\npkg"},
		`\d+`:     {"abc123", "abc", "1", ""},

		// Empty pattern matches everything.
		"": {"", "anything"},
	}

	for pattern, inputs := range tests {
		matcher := ParseMatcher(pattern)
		re := regexp.MustCompile(pattern)
		for _, input := range inputs {
			if matcher([]byte(input)) != re.Match([]byte(input)) {
				t.Errorf("ParseMatcher(%q)(%q) did not align with regexp", pattern, input)
			}
		}
	}
}

func TestParseMatcherInvalidRegexPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected ParseMatcher to panic on an invalid regex")
		}
	}()
	// Has a metacharacter (so it takes the regex path) and is invalid.
	ParseMatcher("[unclosed")
}

func TestParseRegexCaches(t *testing.T) {
	// ParseRegex returns the cached *regexp.Regexp, so repeated calls for the
	// same expression return the identical pointer.
	if ParseRegex("a.c") != ParseRegex("a.c") {
		t.Error("ParseRegex did not return a cached instance for the same expression")
	}
}

func TestParseRegexInvalidPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected ParseRegex to panic on an invalid regex")
		}
	}()
	ParseRegex("[unclosed")
}
