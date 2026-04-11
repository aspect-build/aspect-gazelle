package treesitter

import (
	common "github.com/aspect-build/aspect-gazelle/common"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// An extension of go-tree-sitter predicate filtering.
//
// Examples of standard tree-sitter predicates:
//   - https://tree-sitter.github.io/tree-sitter/using-parsers#predicates
//
// Predicates implemented here:
//   - eq?
//   - match?
func matchesAllPredicates(q *sitterQuery, m *tree_sitter.QueryMatch, input []byte) bool {
	predicates := q.PredicatesForPattern(uint(m.PatternIndex))
	if len(predicates) == 0 {
		return true
	}

	for _, pred := range predicates {
		switch pred.Operator {
		case "eq?", "not-eq?":
			isPositive := pred.Operator == "eq?"

			leftName := q.CaptureNameForId(*pred.Args[0].CaptureId)

			if pred.Args[1].CaptureId != nil {
				// capture vs capture
				rightName := q.CaptureNameForId(*pred.Args[1].CaptureId)
				leftIdx, _ := captureIndexByName(q.captureNames, leftName)
				rightIdx, _ := captureIndexByName(q.captureNames, rightName)
				leftNodes := m.NodesForCaptureIndex(leftIdx)
				rightNodes := m.NodesForCaptureIndex(rightIdx)
				if len(leftNodes) > 0 && len(rightNodes) > 0 {
					if (leftNodes[0].Utf8Text(input) == rightNodes[0].Utf8Text(input)) != isPositive {
						return false
					}
				}
			} else {
				// capture vs literal
				expectedValue := *pred.Args[1].String
				idx, _ := captureIndexByName(q.captureNames, leftName)
				for _, node := range m.NodesForCaptureIndex(idx) {
					if (node.Utf8Text(input) == expectedValue) != isPositive {
						return false
					}
				}
			}

		case "match?", "not-match?":
			isPositive := pred.Operator == "match?"

			captureName := q.CaptureNameForId(*pred.Args[0].CaptureId)
			regex := common.ParseRegex(*pred.Args[1].String)
			idx, _ := captureIndexByName(q.captureNames, captureName)
			for _, node := range m.NodesForCaptureIndex(idx) {
				if regex.MatchString(node.Utf8Text(input)) != isPositive {
					return false
				}
			}
		}
	}

	return true
}

func captureIndexByName(names []string, name string) (uint, bool) {
	for i, n := range names {
		if n == name {
			return uint(i), true
		}
	}
	return 0, false
}
