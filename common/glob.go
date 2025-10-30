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

// Doublestar globs that can be simplified to only a prefix and suffix
var prePostGlobRe = regexp.MustCompile(`^([\w./@-]*)\*\*(/\*?)?([\w./@-]+)$`)

// Globs with a prefix or postfix that can be checked before invoking the regex
var preGlobRe = regexp.MustCompile(`^([\w./@-]+).*$`)
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

	if extGlob := prePostGlobRe.FindStringSubmatch(exp); len(extGlob) > 0 {
		// Globs that can be expressed as pre + ** + ext
		pre, slashStar, ext := extGlob[1], extGlob[2], extGlob[3]
		minLen := len(pre) + len(ext)
		hasStar := slashStar == "/*"
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

func ParseGlobExpressions(exps []string) (GlobExpr, error) {
	if len(exps) == 1 {
		return ParseGlobExpression(exps[0])
	}

	key := strings.Join(exps, ",")
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
			// Globs that can be expressed as pre + ** + ext
			pre, slashStar, ext := extGlob[1], extGlob[2], extGlob[3]
			prePosts[pre] = append(prePosts[pre], []string{slashStar, ext})
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
						hasStar := extData[0] == "/*"
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
