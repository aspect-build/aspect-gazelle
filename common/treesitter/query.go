package treesitter

import tree_sitter "github.com/tree-sitter/go-tree-sitter"

// Basic wrapper around tree_sitter.Query to cache tree-sitter cgo calls.
type sitterQuery struct {
	q *tree_sitter.Query

	// Pre-computed and cached query data
	captureNames      []string
	predicatePatterns [][]tree_sitter.QueryPredicate
}

var _ TreeQuery = (*sitterQuery)(nil)

func newSitterQuery(lang *tree_sitter.Language, query string) (*sitterQuery, error) {
	q, err := tree_sitter.NewQuery(lang, query)
	if err != nil {
		return nil, err
	}

	captureNames := q.CaptureNames()

	predicatePatterns := make([][]tree_sitter.QueryPredicate, q.PatternCount())
	for i := uint(0); i < q.PatternCount(); i++ {
		predicatePatterns[i] = q.GeneralPredicates(i)
	}

	return &sitterQuery{
		q:                 q,
		captureNames:      captureNames,
		predicatePatterns: predicatePatterns,
	}, nil
}

// Cached query data accessors.

func (q *sitterQuery) CaptureNameForId(id uint) string {
	return q.captureNames[id]
}

func (q *sitterQuery) PredicatesForPattern(patternIndex uint) []tree_sitter.QueryPredicate {
	return q.predicatePatterns[patternIndex]
}
