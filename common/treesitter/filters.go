package treesitter

import (
	common "github.com/aspect-build/aspect-gazelle/common"
	sitter "github.com/smacker/go-tree-sitter"
)

// An extension of the go-tree-sitter QueryCursor.FilterPredicates() to add additional filtering.
//
// Limited implementation of predicates implemented in go-tree-sitter:
//   - https://github.com/smacker/go-tree-sitter/blob/c5d1f3f5f99edffd6f1e2f53de46996717717dd2/bindings.go#L1081
//
// Examples of additional standard tree-sitter predicates:
//   - https://tree-sitter.github.io/tree-sitter/using-parsers#predicates
//
// Spec reference:
//   - https://tree-sitter.github.io/tree-sitter/using-parsers/queries/3-predicates-and-directives.html
//
// Predicates implemented here:
//   - eq?, not-eq?
//   - match?, not-match?
//
// Not implemented: any-eq?, any-not-eq?, any-match?, any-not-match?, any-of?, not-any-of?, is?, is-not?, set!.
func matchesAllPredicates(q *sitterQuery, m *sitter.QueryMatch, qc *sitter.QueryCursor, input []byte) bool {
	predicates := q.PredicatesForPattern(uint32(m.PatternIndex))
	if len(predicates) == 0 {
		return true
	}

	// check each predicate against the match
	for _, steps := range predicates {
		operator := q.StringValueForId(steps[0].ValueId)

		switch operator {
		case "eq?", "not-eq?":
			isPositive := operator == "eq?"

			expectedCaptureNameLeft := q.CaptureNameForId(steps[1].ValueId)

			if steps[2].Type == sitter.QueryPredicateStepTypeCapture {
				expectedCaptureNameRight := q.CaptureNameForId(steps[2].ValueId)

				// Pairwise + equal-length, per tree-sitter reference impl:
				// https://github.com/tree-sitter/tree-sitter/blob/master/lib/binding_rust/lib.rs
				var leftContents, rightContents []string
				for _, c := range m.Captures {
					captureName := q.CaptureNameForId(c.Index)
					if captureName == expectedCaptureNameLeft {
						leftContents = append(leftContents, c.Node.Content(input))
					}
					if captureName == expectedCaptureNameRight {
						rightContents = append(rightContents, c.Node.Content(input))
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
					captureName := q.CaptureNameForId(c.Index)

					if expectedCaptureNameLeft != captureName {
						continue
					}

					if (c.Node.Content(input) == expectedValueRight) != isPositive {
						return false
					}
				}
			}

		case "match?", "not-match?":
			isPositive := operator == "match?"

			expectedCaptureName := q.CaptureNameForId(steps[1].ValueId)
			regex := common.ParseRegex(q.StringValueForId(steps[2].ValueId))

			for _, c := range m.Captures {
				captureName := q.CaptureNameForId(c.Index)
				if expectedCaptureName != captureName {
					continue
				}

				if regex.MatchString(c.Node.Content(input)) != isPositive {
					return false
				}
			}
		}
	}

	return true
}
