package plugin

import (
	common "github.com/aspect-build/aspect-gazelle/common"
)

// A set of queries keyed by name.
type NamedQueries map[string]QueryDefinition

// Intermediate object to hold a query key+result in a single struct.
type QueryProcessorResult struct {
	Result interface{}
	Key    string
}

type QueryType = string

const (
	QueryTypeAst   QueryType = "ast"
	QueryTypeRegex           = "regex"
	QueryTypeJson            = "json"
	QueryTypeYaml            = "yaml"
	QueryTypeRaw             = "raw"
)

// A query to run on source files
type QueryDefinition struct {
	Filter    []string
	QueryType QueryType
	Params    interface{}

	FilterExpr common.GlobExpr
}

func (q QueryDefinition) Match(f string) bool {
	if len(q.Filter) == 0 {
		return true
	}

	return q.FilterExpr(f)
}

// TODO: better naming?  QueryMapping?
type QueryResults map[string]interface{}

// Multiple matches
type QueryMatches []QueryMatch

// The captures of a single query match
type QueryCapture map[string]string

// A single match.
type QueryMatch struct {
	Result   interface{}
	Captures QueryCapture
}

func NewQueryMatch(captures QueryCapture, result interface{}) QueryMatch {
	return QueryMatch{Captures: captures, Result: result}
}

type AstQueryParams struct {
	Grammar string
	Query   string
}

type RegexQueryParams = string

type JsonQueryParams = string

type YamlQueryParams = string
