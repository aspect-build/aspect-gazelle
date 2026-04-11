package treesitter

import (
	"fmt"
	"iter"
	"strings"
	"sync"

	BazelLog "github.com/aspect-build/aspect-gazelle/common/logger"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

var ErrorsQuery = `(ERROR) @error`

type TreeQuery any

// A cache of parsed queries per language
var queryCache = sync.Map{}

func GetQuery(lang Language, queryStr string) (*sitterQuery, error) {
	grammar := lang.(*treeLanguage).grammar
	treeLang := lang.(*treeLanguage).lang

	key := string(grammar) + ":" + queryStr

	q, found := queryCache.Load(key)
	if !found {
		sq, err := newSitterQuery(treeLang, queryStr)
		if err != nil {
			return nil, err
		}
		q, _ = queryCache.LoadOrStore(key, sq)
	}
	return q.(*sitterQuery), nil
}

type queryResult struct {
	QueryCaptures map[string]string
}

var _ ASTQueryResult = (*queryResult)(nil)

func (qr queryResult) Captures() map[string]string {
	return qr.QueryCaptures
}

func (tree *treeAst) Query(query TreeQuery) iter.Seq[ASTQueryResult] {
	return func(yield func(ASTQueryResult) bool) {
		q := query.(*sitterQuery)

		qc := tree_sitter.NewQueryCursor()
		defer qc.Close()

		matches := qc.Matches(q.q, tree.sitterTree.RootNode(), tree.sourceCode)
		for m := matches.Next(); m != nil; m = matches.Next() {
			if !matchesAllPredicates(q, m, tree.sourceCode) {
				continue
			}
			r := &queryResult{QueryCaptures: tree.mapQueryMatchCaptures(m, q)}
			if !yield(r) {
				break
			}
		}
	}
}

func (tree *treeAst) mapQueryMatchCaptures(m *tree_sitter.QueryMatch, q *sitterQuery) map[string]string {
	captures := make(map[string]string, len(q.captureNames))
	for i, name := range q.captureNames {
		for _, node := range m.NodesForCaptureIndex(uint(i)) {
			captures[name] = node.Utf8Text(tree.sourceCode)
		}
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

	qc := tree_sitter.NewQueryCursor()
	defer qc.Close()

	matches := qc.Matches(query.q, node, tree.sourceCode)
	for m := matches.Next(); m != nil; m = matches.Next() {
		if !matchesAllPredicates(query, m, tree.sourceCode) {
			continue
		}

		for _, at := range m.NodesForCaptureIndex(0) {
			atStart := at.StartPosition()
			show := at

			// Navigate up the AST to include the full source line
			if atStart.Column > 0 {
				for show.StartPosition().Row > 0 && show.StartPosition().Row == atStart.Row {
					parent := show.Parent()
					if parent == nil {
						break
					}
					show = *parent
				}
			}

			// Extract only that line from the parent Node
			lineI := int(atStart.Row - show.StartPosition().Row)
			colI := int(atStart.Column)
			line := strings.Split(show.Utf8Text(tree.sourceCode), "\n")[lineI]

			pre := fmt.Sprintf("     %d: ", atStart.Row+1)
			msg := pre + line
			arw := strings.Repeat(" ", len(pre)+colI) + "^"

			errors = append(errors, fmt.Errorf("%s\n%s", msg, arw))
		}
	}

	return errors
}
