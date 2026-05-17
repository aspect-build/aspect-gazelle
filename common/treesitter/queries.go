package treesitter

import (
	"fmt"
	"iter"
	"regexp"
	"strconv"
	"strings"
	"sync"

	BazelLog "github.com/aspect-build/aspect-gazelle/common/logger"
	sitter "github.com/odvcencio/gotreesitter"
)

var ErrorsQuery = `(ERROR) @error`

type TreeQuery any

// A cache of parsed queries per language
var queryCache = sync.Map{}
var queryPredicateCache = sync.Map{}

type queryPostPredicateKind int

const (
	queryPredicateEq queryPostPredicateKind = iota
	queryPredicateNotEq
	queryPredicateMatch
	queryPredicateNotMatch
)

type queryPostPredicate struct {
	kind         queryPostPredicateKind
	leftCapture  string
	rightCapture string
	rightLiteral string
	rightRegex   *regexp.Regexp
}

func GetQuery(lang Language, queryStr string) (*sitter.Query, error) {
	tl := lang.(*treeLanguage)
	key := string(tl.grammar) + ":" + queryStr

	q, found := queryCache.Load(key)
	if !found {
		sq, err := sitter.NewQuery(queryStr, tl.lang)
		if err != nil {
			return nil, normalizeQueryError(queryStr, err)
		}
		queryPredicateCache.Store(sq, parseQueryPostPredicates(queryStr))
		q, _ = queryCache.LoadOrStore(key, sq)
	}
	return q.(*sitter.Query), nil
}

func normalizeQueryError(queryStr string, err error) error {
	msg := err.Error()
	const prefix = "query: unknown node type "
	if !strings.HasPrefix(msg, prefix) {
		return err
	}
	name, unquoteErr := strconv.Unquote(strings.TrimPrefix(msg, prefix))
	if unquoteErr != nil || name == "" {
		return err
	}
	line, column := queryPatternPosition(queryStr, name)
	return fmt.Errorf("invalid node type '%s' at line %d column %d", name, line, column)
}

func queryPatternPosition(queryStr, name string) (int, int) {
	idx := strings.Index(queryStr, name)
	if idx < 0 {
		return 0, 0
	}
	if open := strings.LastIndex(queryStr[:idx], "("); open >= 0 {
		idx = open
	}
	line, col := 1, 0
	for _, ch := range queryStr[:idx] {
		if ch == '\n' {
			line++
			col = 0
		} else {
			col++
		}
	}
	return line, col
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
			if !queryMatchPassesPostPredicates(q, m, tree.sourceCode) {
				continue
			}

			captures := tree.mapQueryMatchCaptures(m)
			if !yield(&queryResult{QueryCaptures: captures}) {
				return
			}
		}
	}
}

func parseQueryPostPredicates(queryStr string) []queryPostPredicate {
	var out []queryPostPredicate
	for _, raw := range extractPredicateForms(queryStr) {
		fields := strings.Fields(raw)
		if len(fields) < 3 {
			continue
		}
		kind, ok := parsePredicateKind(fields[0])
		if !ok {
			continue
		}
		left := strings.TrimPrefix(fields[1], "@")
		if left == fields[1] || left == "" {
			continue
		}
		pred := queryPostPredicate{kind: kind, leftCapture: left}
		right := fields[2]
		if strings.HasPrefix(right, "@") {
			pred.rightCapture = strings.TrimPrefix(right, "@")
		} else {
			lit, err := strconv.Unquote(right)
			if err != nil {
				continue
			}
			if kind == queryPredicateMatch || kind == queryPredicateNotMatch {
				re, err := regexp.Compile(lit)
				if err != nil {
					continue
				}
				pred.rightRegex = re
			} else {
				pred.rightLiteral = lit
			}
		}
		out = append(out, pred)
	}
	return out
}

func parsePredicateKind(name string) (queryPostPredicateKind, bool) {
	switch name {
	case "#eq?":
		return queryPredicateEq, true
	case "#not-eq?":
		return queryPredicateNotEq, true
	case "#match?":
		return queryPredicateMatch, true
	case "#not-match?":
		return queryPredicateNotMatch, true
	default:
		return 0, false
	}
}

func extractPredicateForms(queryStr string) []string {
	var forms []string
	for i := 0; i < len(queryStr); i++ {
		if queryStr[i] != '(' || i+1 >= len(queryStr) || queryStr[i+1] != '#' {
			continue
		}
		start := i + 1
		inString := false
		escaped := false
		for j := i + 1; j < len(queryStr); j++ {
			ch := queryStr[j]
			if inString {
				if escaped {
					escaped = false
					continue
				}
				switch ch {
				case '\\':
					escaped = true
				case '"':
					inString = false
				}
				continue
			}
			switch ch {
			case '"':
				inString = true
			case ')':
				forms = append(forms, queryStr[start:j])
				i = j
				goto nextForm
			}
		}
	nextForm:
	}
	return forms
}

func queryMatchPassesPostPredicates(query *sitter.Query, m sitter.QueryMatch, source []byte) bool {
	rawPreds, ok := queryPredicateCache.Load(query)
	if !ok {
		return true
	}
	preds := rawPreds.([]queryPostPredicate)
	if len(preds) == 0 {
		return true
	}
	captures := captureTextValues(m, source)
	for _, pred := range preds {
		left := captures[pred.leftCapture]
		if len(left) == 0 {
			continue
		}
		if !queryPostPredicatePasses(pred, left, captures) {
			return false
		}
	}
	return true
}

func captureTextValues(m sitter.QueryMatch, source []byte) map[string][]string {
	out := make(map[string][]string)
	for _, c := range m.Captures {
		out[c.Name] = append(out[c.Name], c.Text(source))
	}
	return out
}

func queryPostPredicatePasses(pred queryPostPredicate, left []string, captures map[string][]string) bool {
	switch pred.kind {
	case queryPredicateEq, queryPredicateNotEq:
		var right []string
		if pred.rightCapture != "" {
			right = captures[pred.rightCapture]
			if len(right) != len(left) {
				return false
			}
		}
		for i, value := range left {
			cmp := pred.rightLiteral
			if pred.rightCapture != "" {
				cmp = right[i]
			}
			equal := value == cmp
			if pred.kind == queryPredicateEq && !equal {
				return false
			}
			if pred.kind == queryPredicateNotEq && equal {
				return false
			}
		}
		return true
	case queryPredicateMatch, queryPredicateNotMatch:
		if pred.rightRegex == nil {
			return true
		}
		for _, value := range left {
			matched := pred.rightRegex.MatchString(value)
			if pred.kind == queryPredicateMatch && !matched {
				return false
			}
			if pred.kind == queryPredicateNotMatch && matched {
				return false
			}
		}
		return true
	default:
		return true
	}
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
