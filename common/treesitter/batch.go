package treesitter

import (
	"sync"
	"unsafe"

	"github.com/aspect-build/aspect-gazelle/common/treesitter/tsbatch"
	sitter "github.com/smacker/go-tree-sitter"
)

// languagePtr recovers the raw TSLanguage* that smacker's *Language wraps as its
// single unexported field, to hand to the tsbatch cgo bridge. smacker is
// version-pinned and patched in this repo, so the layout is stable.
func languagePtr(l *sitter.Language) unsafe.Pointer {
	return *(*unsafe.Pointer)(unsafe.Pointer(l))
}

// Compiled tsbatch queries, cached per grammar+query string. The compiled query
// is immutable and reused across files and goroutines; ts_query_new is the most
// expensive tree-sitter op and must not run per file.
var batchQueryCache sync.Map

func compiledBatchQuery(grammar LanguageGrammar, langPtr unsafe.Pointer, queryStr string) (*tsbatch.Query, error) {
	key := string(grammar) + ":" + queryStr
	if v, ok := batchQueryCache.Load(key); ok {
		return v.(*tsbatch.Query), nil
	}
	q, err := tsbatch.Compile(langPtr, queryStr)
	if err != nil {
		return nil, err
	}
	actual, _ := batchQueryCache.LoadOrStore(key, q)
	return actual.(*tsbatch.Query), nil
}

// QueryBatch runs each query (keyed by id) against sourceCode through the
// tsbatch cgo bridge. The source is parsed once and each query runs in a single
// cgo crossing that returns capture byte-ranges, versus smacker's
// crossing-per-match walk. Compiled queries, capture names, and predicate
// metadata are all cached. Unlike the treeAst.Query path it touches no shared
// tree node cache, so callers may run it concurrently across files without the
// sequential-per-tree constraint.
func QueryBatch(lang Language, sourceCode []byte, queries map[string]string) (map[string][]ASTQueryResult, error) {
	tl := lang.(*treeLanguage)
	langPtr := languagePtr(tl.lang)

	tree := tsbatch.Parse(langPtr, sourceCode)
	defer tree.Close()

	results := make(map[string][]ASTQueryResult, len(queries))
	for id, queryStr := range queries {
		// Cached smacker compile: capture names + predicate metadata.
		q, err := GetQuery(lang, queryStr)
		if err != nil {
			return nil, err
		}

		// Cached tsbatch compile: the executable query.
		cq, err := compiledBatchQuery(tl.grammar, langPtr, queryStr)
		if err != nil {
			return nil, err
		}

		matches := tsbatch.RunQuery(cq, tree)

		out := make([]ASTQueryResult, 0, len(matches))
		for _, m := range matches {
			if !matchesAllPredicatesBatch(q, m, sourceCode) {
				continue
			}
			captures := make(ASTQueryResult, len(m.Captures))
			for _, c := range m.Captures {
				captures[q.CaptureNameForId(c.ID)] = string(sourceCode[c.StartByte:c.EndByte])
			}
			out = append(out, captures)
		}
		results[id] = out
	}

	return results, nil
}

// matchesAllPredicatesBatch is matchesAllPredicates for tsbatch.Match results:
// capture text is the source slice [StartByte, EndByte) and capture names come
// from the capture id. Keep in sync with matchesAllPredicates (filters.go).
func matchesAllPredicatesBatch(q *sitterQuery, m tsbatch.Match, input []byte) bool {
	predicates := q.PredicatesForPattern(m.Pattern)
	if len(predicates) == 0 {
		return true
	}

	for _, steps := range predicates {
		operator := q.StringValueForId(steps[0].ValueId)

		switch operator {
		case "eq?", "not-eq?":
			isPositive := operator == "eq?"

			expectedCaptureNameLeft := q.CaptureNameForId(steps[1].ValueId)

			if steps[2].Type == sitter.QueryPredicateStepTypeCapture {
				expectedCaptureNameRight := q.CaptureNameForId(steps[2].ValueId)

				var leftContents, rightContents []string
				for _, c := range m.Captures {
					captureName := q.CaptureNameForId(c.ID)
					if captureName == expectedCaptureNameLeft {
						leftContents = append(leftContents, string(input[c.StartByte:c.EndByte]))
					}
					if captureName == expectedCaptureNameRight {
						rightContents = append(rightContents, string(input[c.StartByte:c.EndByte]))
					}
				}

				if len(leftContents) != len(rightContents) {
					return false
				}
				for i := range leftContents {
					if (leftContents[i] == rightContents[i]) != isPositive {
						return false
					}
				}
			} else {
				expectedValueRight := q.StringValueForId(steps[2].ValueId)

				for _, c := range m.Captures {
					if expectedCaptureNameLeft != q.CaptureNameForId(c.ID) {
						continue
					}
					if (string(input[c.StartByte:c.EndByte]) == expectedValueRight) != isPositive {
						return false
					}
				}
			}

		case "match?", "not-match?":
			isPositive := operator == "match?"

			expectedCaptureName := q.CaptureNameForId(steps[1].ValueId)
			matcher := q.MatcherForId(steps[2].ValueId)

			for _, c := range m.Captures {
				if expectedCaptureName != q.CaptureNameForId(c.ID) {
					continue
				}
				if matcher(input[c.StartByte:c.EndByte]) != isPositive {
					return false
				}
			}
		}
	}

	return true
}
