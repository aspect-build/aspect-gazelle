package treesitter

import (
	"fmt"
	"iter"
	"strings"
	"sync"

	BazelLog "github.com/aspect-build/aspect-gazelle/common/logger"
	sitter "github.com/odvcencio/gotreesitter"
)

var ErrorsQuery = `(ERROR) @error`

type TreeQuery any

// A cache of parsed queries per language
var queryCache = sync.Map{}

func GetQuery(lang Language, queryStr string) (*sitter.Query, error) {
	tl := lang.(*treeLanguage)
	key := string(tl.grammar) + ":" + queryStr

	q, found := queryCache.Load(key)
	if !found {
		sq, err := sitter.NewQuery(queryStr, tl.lang)
		if err != nil {
			return nil, err
		}
		q, _ = queryCache.LoadOrStore(key, sq)
	}
	return q.(*sitter.Query), nil
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
		q := query.(*sitter.Query)

		cursor := q.Exec(tree.sitterTree.RootNode(), tree.lang.lang, tree.sourceCode)

		for {
			m, ok := cursor.NextMatch()
			if !ok {
				break
			}

			// gotreesitter can return a single QueryMatch containing multiple
			// captures with the same name (one per matching child node). The
			// rest of the codebase expects one result per logical match — i.e.
			// one capture per name. Expand such matches into per-row results,
			// pairing each duplicate name's i-th value with the other captures
			// at the same row index (or with non-duplicated captures).
			rows := splitMatchRows(m)
			for _, row := range rows {
				captures := make(map[string]string, len(row))
				for _, c := range row {
					captures[c.Name] = c.Text(tree.sourceCode)
				}
				if !yield(&queryResult{QueryCaptures: captures}) {
					return
				}
			}
		}
	}
}

// splitMatchRows splits a QueryMatch into one or more rows, where each row
// contains at most one capture per name. When all capture names are unique,
// a single row is returned. When duplicates exist, captures are zip-aligned
// across name occurrences.
func splitMatchRows(m sitter.QueryMatch) [][]sitter.QueryCapture {
	if len(m.Captures) == 0 {
		return [][]sitter.QueryCapture{nil}
	}
	maxCount := 1
	counts := make(map[string]int)
	for _, c := range m.Captures {
		counts[c.Name]++
		if counts[c.Name] > maxCount {
			maxCount = counts[c.Name]
		}
	}
	if maxCount == 1 {
		return [][]sitter.QueryCapture{m.Captures}
	}
	rows := make([][]sitter.QueryCapture, maxCount)
	idx := make(map[string]int)
	for _, c := range m.Captures {
		i := idx[c.Name]
		rows[i] = append(rows[i], c)
		idx[c.Name] = i + 1
	}
	return rows
}

func (tree *treeAst) mapQueryMatchCaptures(m sitter.QueryMatch) map[string]string {
	captures := make(map[string]string, len(m.Captures))
	for _, c := range m.Captures {
		captures[c.Name] = c.Text(tree.sourceCode)
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

	cursor := query.Exec(node, tree.lang.lang, tree.sourceCode)

	for {
		m, ok := cursor.NextMatch()
		if !ok {
			break
		}

		for _, c := range m.Captures {
			at := c.Node
			atStart := at.StartPoint()
			show := c.Node

			// Navigate up the AST to include the full source line
			if atStart.Column > 0 {
				for show.StartPoint().Row > 0 && show.StartPoint().Row == atStart.Row && show.Parent() != nil {
					show = show.Parent()
				}
			}

			// Extract only that line from the parent Node
			lineI := int(atStart.Row - show.StartPoint().Row)
			colI := int(atStart.Column)
			line := strings.Split(show.Text(tree.sourceCode), "\n")[lineI]

			pre := fmt.Sprintf("     %d: ", atStart.Row+1)
			msg := pre + line
			arw := strings.Repeat(" ", len(pre)+colI) + "^"

			errors = append(errors, fmt.Errorf("%s\n%s", msg, arw))
		}
	}

	return errors
}
