package gazelle

import (
	"bytes"
	"regexp"
	"sync"
)

// The signature of regexp.Regexp.Match - used when the underlying matcher could
// be a regex or something else (e.g. a substring match) and regex features such
// as capture groups are not needed.
type BytesMatcher func([]byte) bool

// A cache of parsed regex strings
var regexCache = sync.Map{}

// A matcher cache, may delegate/wrap entries of the regexCache
var matcherCache = sync.Map{}

// Parse a regex and return a regexp.Regexp.
// Use if the full regexp.Regexp API is needed (e.g. capture groups), otherwise
// the ParseMatcher() API is more efficient for simple substring matches.
// Panics if the regex is invalid. Caches parsed regexes for efficiency.
func ParseRegex(regexStr string) *regexp.Regexp {
	re, found := regexCache.Load(regexStr)
	if !found {
		re, _ = regexCache.LoadOrStore(regexStr, regexp.MustCompile(regexStr))
	}

	return re.(*regexp.Regexp)
}

// Parse a regex or simple substring and return a BytesMatcher.
// Panics if the regex is invalid. Caches parsed regexes for efficiency.
func ParseMatcher(matchExpr string) BytesMatcher {
	if m, found := matcherCache.Load(matchExpr); found {
		return m.(BytesMatcher)
	}

	var m BytesMatcher
	if regexp.QuoteMeta(matchExpr) == matchExpr {
		needle := []byte(matchExpr)
		m = func(b []byte) bool {
			return bytes.Contains(b, needle)
		}
	} else {
		m = ParseRegex(matchExpr).Match
	}

	actual, _ := matcherCache.LoadOrStore(matchExpr, m)
	return actual.(BytesMatcher)
}
