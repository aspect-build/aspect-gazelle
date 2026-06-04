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
		matcher, err := ParseMatcher(pattern)
		if err != nil {
			t.Fatalf("ParseMatcher(%q) returned an error: %v", pattern, err)
		}
		re := regexp.MustCompile(pattern)
		for _, input := range inputs {
			if matcher([]byte(input)) != re.Match([]byte(input)) {
				t.Errorf("ParseMatcher(%q)(%q) did not align with regexp", pattern, input)
			}
		}
	}
}

func TestParseMatcherInvalidRegexErrors(t *testing.T) {
	// Has a metacharacter (so it takes the regex path) and is invalid.
	m, err := ParseMatcher("[unclosed")
	if err == nil {
		t.Fatal("expected ParseMatcher to return an error on an invalid regex")
	}
	if m != nil {
		t.Error("expected ParseMatcher to return a nil matcher on an invalid regex")
	}
}

func TestParseRegexCaches(t *testing.T) {
	// ParseRegex returns the cached *regexp.Regexp, so repeated calls for the
	// same expression return the identical pointer.
	a, err := ParseRegex("a.c")
	if err != nil {
		t.Fatalf("ParseRegex returned an error: %v", err)
	}
	b, err := ParseRegex("a.c")
	if err != nil {
		t.Fatalf("ParseRegex returned an error: %v", err)
	}
	if a != b {
		t.Error("ParseRegex did not return a cached instance for the same expression")
	}
}

func TestParseRegexInvalidErrors(t *testing.T) {
	re, err := ParseRegex("[unclosed")
	if err == nil {
		t.Fatal("expected ParseRegex to return an error on an invalid regex")
	}
	if re != nil {
		t.Error("expected ParseRegex to return a nil regexp on an invalid regex")
	}
}
