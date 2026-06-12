package treesitter

import (
	"bytes"
	"fmt"
	"iter"
	"strings"
	"sync"

	BazelLog "github.com/aspect-build/aspect-gazelle/common/logger"

	sitter "github.com/smacker/go-tree-sitter"
)

var ErrorsQuery = `(ERROR) @error`

// TreeQuery is an opaque compiled query from GetQuery, sealed to this package.
type TreeQuery interface {
	sealedQuery()
}

// A cache of parsed queries per language
var queryCache = sync.Map{}

func GetQuery(lang Language, queryStr string) (TreeQuery, error) {
	key := string(lang.Grammar()) + ":" + queryStr

	q, found := queryCache.Load(key)
	if !found {
		sq, err := newSitterQuery(lang.sitterLang(), queryStr)
		if err != nil {
			return nil, err
		}
		q, _ = queryCache.LoadOrStore(key, sq)
	}
	return q.(*sitterQuery), nil
}

func (tree *treeAst) Query(query TreeQuery) iter.Seq[ASTQueryResult] {
	return func(yield func(ASTQueryResult) bool) {
		// TreeQuery is sealed; *sitterQuery is the only implementation.
		q := query.(*sitterQuery)

		// Execute the query.
		qc := sitter.NewQueryCursor()
		defer qc.Close()
		qc.Exec(q.q, tree.sitterTree.RootNode())

		for {
			m, ok := qc.NextMatch()
			if !ok {
				break
			}

			// Filter the capture results
			if !matchesAllPredicates(q, m, qc, tree.sourceCode) {
				continue
			}

			if !yield(tree.mapQueryMatchCaptures(m, q)) {
				break
			}
		}
	}
}

func (tree *treeAst) mapQueryMatchCaptures(m *sitter.QueryMatch, q *sitterQuery) ASTQueryResult {
	captures := make(map[string]string, len(m.Captures))
	for _, c := range m.Captures {
		name := q.CaptureNameForId(c.Index)
		captures[name] = c.Node.Content(tree.sourceCode)
	}

	return captures
}

// Create an error for each parse error.
func (tree *treeAst) QueryErrors() []error {
	node := tree.sitterTree.RootNode()
	if !node.HasError() {
		return nil
	}

	var errors []error

	query, err := GetQuery(tree.lang, ErrorsQuery)
	if err != nil {
		BazelLog.Fatalf("Failed to create util 'ErrorsQuery': %v", err)
	}
	q := query.(*sitterQuery)

	qc := sitter.NewQueryCursor()
	defer qc.Close()
	qc.Exec(q.q, node)

	for {
		m, ok := qc.NextMatch()
		if !ok {
			break
		}

		// Apply predicates to filter results.
		if !matchesAllPredicates(q, m, qc, tree.sourceCode) {
			continue
		}

		for _, c := range m.Captures {
			at := c.Node.StartPoint()
			errors = append(errors, queryErrorAt(tree.sourceCode, at.Row, at.Column, c.Node.StartByte()))
		}
	}

	return errors
}

// queryErrorAt renders the error's source line with a caret at the error
// position, from plain position data — no AST navigation.
func queryErrorAt(source []byte, row, col, startByte uint32) error {
	// col is the byte offset within the line.
	lineStart := int(startByte - col)
	lineEnd := len(source)
	if i := bytes.IndexByte(source[startByte:], '\n'); i >= 0 {
		lineEnd = int(startByte) + i
	}

	pre := fmt.Sprintf("     %d: ", row+1)
	arw := strings.Repeat(" ", len(pre)+int(col)) + "^"

	return fmt.Errorf("%s%s\n%s", pre, source[lineStart:lineEnd], arw)
}
