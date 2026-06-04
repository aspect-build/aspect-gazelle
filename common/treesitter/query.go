package treesitter

import (
	"fmt"

	common "github.com/aspect-build/aspect-gazelle/common"
	sitter "github.com/smacker/go-tree-sitter"
)

// Basic wrapper around sitter.Query to cache tree-sitter cgo calls.
type sitterQuery struct {
	q *sitter.Query

	// Pre-computed and cached query data
	stringValues      []string
	captureNames      []string
	predicatePatterns [][][]sitter.QueryPredicateStep

	// Pre-parsed match?/not-match? predicate expressions, indexed by the
	// predicate string value id. Nil for string values that are not matchers.
	matchers []common.BytesMatcher
}

var _ TreeQuery = (*sitterQuery)(nil)

func newSitterQuery(lang *sitter.Language, query string) (*sitterQuery, error) {
	q, err := sitter.NewQuery([]byte(query), lang)
	if err != nil {
		return nil, err
	}

	captureNames := make([]string, q.CaptureCount())
	for i := uint32(0); i < q.CaptureCount(); i++ {
		captureNames[i] = q.CaptureNameForId(i)
	}

	stringValues := make([]string, q.StringCount())
	for i := uint32(0); i < q.StringCount(); i++ {
		stringValues[i] = q.StringValueForId(i)
	}

	predicatePatterns := make([][][]sitter.QueryPredicateStep, q.PatternCount())
	matchers := make([]common.BytesMatcher, q.StringCount())
	for i := uint32(0); i < q.PatternCount(); i++ {
		predicatePatterns[i] = q.PredicatesForPattern(i)

		// Parse match? predicate expressions so an invalid expression fails
		// here instead of when the query is run against a match.
		for _, steps := range predicatePatterns[i] {
			switch stringValues[steps[0].ValueId] {
			case "match?", "not-match?":
				exprId := steps[2].ValueId
				m, err := common.ParseMatcher(stringValues[exprId])
				if err != nil {
					return nil, fmt.Errorf("invalid %s predicate expression %q: %w", stringValues[steps[0].ValueId], stringValues[exprId], err)
				}
				matchers[exprId] = m
			}
		}
	}

	return &sitterQuery{
		q:                 q,
		stringValues:      stringValues,
		captureNames:      captureNames,
		predicatePatterns: predicatePatterns,
		matchers:          matchers,
	}, nil
}

// Cached query data accessors mirroring the tree-sitter Query signatures.

func (q *sitterQuery) StringValueForId(id uint32) string {
	return q.stringValues[id]
}

func (q *sitterQuery) CaptureNameForId(id uint32) string {
	return q.captureNames[id]
}

func (q *sitterQuery) PredicatesForPattern(patternIndex uint32) [][]sitter.QueryPredicateStep {
	return q.predicatePatterns[patternIndex]
}

// The pre-parsed matcher for a match? predicate expression string value id.
func (q *sitterQuery) MatcherForId(id uint32) common.BytesMatcher {
	return q.matchers[id]
}
