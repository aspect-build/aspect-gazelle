package gazelle

import (
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/bmatcuk/doublestar/v4"
)

type GlobExpr func(string) bool

// Expressions that are not even globs
var nonGlobRe = regexp.MustCompile(`^[\w./@-]+$`)

// Expressions that are only a prefix and trailing `/**`
var prefixDirRe = regexp.MustCompile(`^([\w./@-]*)/\*\*$`)

// Doublestar globs that can be simplified to only a prefix and suffix.
// The `**` must be separator-bounded (preceded by `/` or the start, followed by `/`)
// to behave as a real globstar; a glued `**` (e.g. `**.go`) is just a single `*` to doublestar, so it is
// intentionally excluded here and left to the doublestar-backed pre/post paths.
var prePostGlobRe = regexp.MustCompile(`^([\w./@-]*/|)\*\*/(\*?)([\w./@-]+)$`)

// Globs with a prefix or postfix that can be checked before invoking the regex.
// The prefix capture intentionally never ends in `/`: doublestar's zero-length rule
// lets `dir/**` and `dir/**/` match `dir` itself (no trailing slash), so a prefix of
// `dir/` would wrongly reject that case. Trimming the slash keeps the fast-reject sound.
var preGlobRe = regexp.MustCompile(`^([\w./@-]*[\w.@-]).*$`)
var postGlobRe = regexp.MustCompile(`^.*?([\w./@-]+)$`)

var parsedExpCache sync.Map

func ParseGlobExpression(exp string) (GlobExpr, error) {
	loaded, ok := parsedExpCache.Load(exp)
	if ok {
		return loaded.(GlobExpr), nil
	}

	if !doublestar.ValidatePattern(exp) {
		return nil, fmt.Errorf("invalid glob pattern: %s", exp)
	}

	expr := parseGlobExpression(exp)
	loaded, _ = parsedExpCache.LoadOrStore(exp, expr)
	return loaded.(GlobExpr), nil
}

func parseGlobExpression(exp string) GlobExpr {
	if nonGlobRe.MatchString(exp) {
		return func(p string) bool {
			return p == exp
		}
	}

	if tg := prefixDirRe.FindStringSubmatch(exp); len(tg) > 0 {
		// `dir/**` matches the directory itself plus anything beneath it
		dir := tg[1]
		return func(p string) bool {
			return p == dir || (len(p) > len(dir) && p[len(dir)] == '/' && strings.HasPrefix(p, dir))
		}
	}

	if extGlob := prePostGlobRe.FindStringSubmatch(exp); len(extGlob) > 0 {
		// Globs that can be expressed as pre + **/ + ext
		pre, star, ext := extGlob[1], extGlob[2], extGlob[3]
		minLen := len(pre) + len(ext)
		hasStar := star == "*"
		return func(p string) bool {
			if len(p) < minLen || !strings.HasPrefix(p, pre) {
				return false
			}
			return strings.HasSuffix(p, ext) && (hasStar || p == ext || p[len(p)-len(ext)-1] == '/')
		}
	}

	if preGlob := preGlobRe.FindStringSubmatch(exp); len(preGlob) > 0 {
		pre := preGlob[1]
		return func(p string) bool {
			if !strings.HasPrefix(p, pre) {
				return false
			}
			return doublestar.MatchUnvalidated(exp, p)
		}
	}

	if postGlob := postGlobRe.FindStringSubmatch(exp); len(postGlob) > 0 {
		post := postGlob[1]
		return func(p string) bool {
			if !strings.HasSuffix(p, post) {
				return false
			}
			return doublestar.MatchUnvalidated(exp, p)
		}
	}

	return func(p string) bool {
		return doublestar.MatchUnvalidated(exp, p)
	}
}

// ParseGlobExpressionsWithExcludes matches paths matching any include pattern (or all paths if only excludes are given) and no exclude pattern.
func ParseGlobExpressionsWithExcludes(includes, excludes []string) (GlobExpr, error) {
	if len(excludes) == 0 {
		return ParseGlobExpressions(includes)
	}

	excludeMatch, err := ParseGlobExpressions(excludes)
	if err != nil {
		return nil, err
	}

	// Excludes-only: match everything that is not excluded.
	if len(includes) == 0 {
		return func(p string) bool {
			return !excludeMatch(p)
		}, nil
	}

	includeMatch, err := ParseGlobExpressions(includes)
	if err != nil {
		return nil, err
	}

	return func(p string) bool {
		return includeMatch(p) && !excludeMatch(p)
	}, nil
}

// ParseGlobExpressions matches paths matched by any of the given glob patterns.
func ParseGlobExpressions(exps []string) (GlobExpr, error) {
	if len(exps) == 0 {
		return nil, fmt.Errorf("no glob patterns provided")
	}

	if len(exps) == 1 {
		return ParseGlobExpression(exps[0])
	}

	// Join with NUL which cannot appear in glob patterns,
	// so joining on it cannot collide two different pattern lists.
	key := strings.Join(exps, "\x00")
	loaded, ok := parsedExpCache.Load(key)
	if ok {
		return loaded.(GlobExpr), nil
	}

	expr, err := parseGlobExpressions(exps)
	if err != nil {
		return nil, err
	}

	loaded, _ = parsedExpCache.LoadOrStore(key, expr)
	return loaded.(GlobExpr), nil
}

func parseGlobExpressions(exps []string) (GlobExpr, error) {
	exacts := make(map[string]struct{})
	prePosts := make(map[string][][]string)
	prefixDirs := make([]string, 0)
	preGlobs := make(map[string][]string)
	postGlobs := make(map[string][]string)
	globs := make([]string, 0)

	for _, exp := range exps {
		if !doublestar.ValidatePattern(exp) {
			return nil, fmt.Errorf("invalid glob pattern: %s", exp)
		}

		if nonGlobRe.MatchString(exp) {
			exacts[exp] = struct{}{}
		} else if extGlob := prePostGlobRe.FindStringSubmatch(exp); len(extGlob) > 0 {
			// Globs that can be expressed as pre + **/ + ext
			pre, star, ext := extGlob[1], extGlob[2], extGlob[3]
			prePosts[pre] = append(prePosts[pre], []string{star, ext})
		} else if tg := prefixDirRe.FindStringSubmatch(exp); len(tg) > 0 {
			prefixDirs = append(prefixDirs, tg[1])
		} else if preGlob := preGlobRe.FindStringSubmatch(exp); len(preGlob) > 0 {
			pre := preGlob[1]
			preGlobs[pre] = append(preGlobs[pre], exp)
		} else if postGlob := postGlobRe.FindStringSubmatch(exp); len(postGlob) > 0 {
			post := postGlob[1]
			postGlobs[post] = append(postGlobs[post], exp)
		} else {
			globs = append(globs, exp)
		}
	}

	exprFuncs := make([]GlobExpr, 0, 5)

	if len(exacts) > 0 {
		exprFuncs = append(exprFuncs, func(p string) bool {
			_, e := exacts[p]
			return e
		})
	}

	if len(prePosts) > 0 {
		exprFuncs = append(exprFuncs, func(p string) bool {
			lenP := len(p)
			for pre, exts := range prePosts {
				if strings.HasPrefix(p, pre) {
					for _, extData := range exts {
						hasStar := extData[0] == "*"
						ext := extData[1]

						if lenP >= len(pre)+len(ext) && strings.HasSuffix(p, ext) && (hasStar || p == ext || p[lenP-len(ext)-1] == '/') {
							return true
						}
					}
				}
			}
			return false
		})
	}

	if len(prefixDirs) > 0 {
		exprFuncs = append(exprFuncs, func(p string) bool {
			for _, dir := range prefixDirs {
				if p == dir || (len(p) > len(dir) && p[len(dir)] == '/' && strings.HasPrefix(p, dir)) {
					return true
				}
			}
			return false
		})
	}

	if len(preGlobs) > 0 {
		exprFuncs = append(exprFuncs, func(p string) bool {
			for pre, globs := range preGlobs {
				if strings.HasPrefix(p, pre) {
					for _, glob := range globs {
						if doublestar.MatchUnvalidated(glob, p) {
							return true
						}
					}
				}
			}
			return false
		})
	}

	if len(postGlobs) > 0 {
		exprFuncs = append(exprFuncs, func(p string) bool {
			for post, globs := range postGlobs {
				if strings.HasSuffix(p, post) {
					for _, glob := range globs {
						if doublestar.MatchUnvalidated(glob, p) {
							return true
						}
					}
				}
			}
			return false
		})
	}

	if len(globs) > 0 {
		exprFuncs = append(exprFuncs, func(p string) bool {
			for _, glob := range globs {
				if doublestar.MatchUnvalidated(glob, p) {
					return true
				}
			}
			return false
		})
	}

	return func(p string) bool {
		for _, expr := range exprFuncs {
			if expr(p) {
				return true
			}
		}
		return false
	}, nil
}
