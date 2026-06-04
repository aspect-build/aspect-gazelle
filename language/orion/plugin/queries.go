package plugin

import (
	"regexp"

	common "github.com/aspect-build/aspect-gazelle/common"
)

// A set of queries keyed by name.
type NamedQueries map[string]QueryDefinition

// Intermediate object to hold a query key+result in a single struct.
type QueryProcessorResult struct {
	Result any
	Key    string
}

type QueryType = string

const (
	QueryTypeAst   QueryType = "ast"
	QueryTypeRegex           = "regex"
	QueryTypeJson            = "json"
	QueryTypeYaml            = "yaml"
	QueryTypeToml            = "toml"
	QueryTypeRaw             = "raw"
)

// A query to run on source files.
//
// Implementations are pointers to the *Query structs below, one per QueryType.
type QueryDefinition interface {
	QueryType() QueryType
	MatchPath(f string) bool
	MatchContent(src []byte) bool

	// Seal the interface to pointers to the *Query structs embedding
	// QueryBase. The pointer receiver ensures only the pointer forms satisfy
	// the interface, so query processors can type-assert on them safely.
	isQueryDefinition()
}

// Common properties of all QueryDefinition implementations.
type QueryBase struct {
	// Filter gates the query by file path glob(s); empty means all files.
	// FilterExpr is its compiled matcher.
	Filter     []string
	FilterExpr common.GlobExpr

	// ContentFilter gates the query by file content (substring or regex); empty
	// means always run. ContentFilterExpr is its compiled matcher.
	ContentFilter     string
	ContentFilterExpr common.BytesMatcher
}

func (q QueryBase) MatchPath(f string) bool { return q.FilterExpr(f) }

// MatchContent reports whether the content gate (ContentFilter) matches the
// source. Always true when no ContentFilter was set.
func (q QueryBase) MatchContent(src []byte) bool {
	return q.ContentFilterExpr == nil || q.ContentFilterExpr(src)
}

func (*QueryBase) isQueryDefinition() {}

// A tree-sitter query on the source AST.
type AstQuery struct {
	QueryBase
	Grammar string
	Query   string
}

func (AstQuery) QueryType() QueryType { return QueryTypeAst }

// A regular expression query on the source text.
//
// Create via NewRegexQuery so the expression is parsed (and validated) once.
type RegexQuery struct {
	QueryBase
	Expression string

	// The parsed Expression.
	expressionRe *regexp.Regexp
}

func NewRegexQuery(base QueryBase, expression string) (*RegexQuery, error) {
	re, err := common.ParseRegex(expression)
	if err != nil {
		return nil, err
	}

	return &RegexQuery{
		QueryBase:    base,
		Expression:   expression,
		expressionRe: re,
	}, nil
}

// The parsed Expression, only available when created via NewRegexQuery.
func (q *RegexQuery) ExpressionRe() *regexp.Regexp { return q.expressionRe }

func (RegexQuery) QueryType() QueryType { return QueryTypeRegex }

// A jq query on a JSON document.
type JsonQuery struct {
	QueryBase
	Query string
}

func (JsonQuery) QueryType() QueryType { return QueryTypeJson }

// A yq query on a YAML document.
type YamlQuery struct {
	QueryBase
	Query string
}

func (YamlQuery) QueryType() QueryType { return QueryTypeYaml }

// A yq query on a TOML document.
type TomlQuery struct {
	QueryBase
	Query string
}

func (TomlQuery) QueryType() QueryType { return QueryTypeToml }

// A query returning the raw source text.
type RawQuery struct {
	QueryBase
}

func (RawQuery) QueryType() QueryType { return QueryTypeRaw }

// TODO: better naming?  QueryMapping?
type QueryResults map[string]any

// Multiple matches
type QueryMatches []QueryMatch

// The captures of a single query match
type QueryCapture map[string]string

// A single match.
type QueryMatch struct {
	Result   any
	Captures QueryCapture
}

func NewQueryMatch(captures QueryCapture, result any) QueryMatch {
	return QueryMatch{Captures: captures, Result: result}
}
