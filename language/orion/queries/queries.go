package queries

import (
	BazelLog "github.com/aspect-build/aspect-gazelle/common/logger"
	"github.com/aspect-build/aspect-gazelle/language/orion/plugin"
)

func RunQueries(queryType plugin.QueryType, fileName string, sourceCode []byte, queries plugin.NamedQueries, queryResults chan *plugin.QueryProcessorResult) error {
	// Content gate: a query with a ContentFilter only runs if the source matches
	// it. Queries gated out get an empty result; if that leaves nothing to run,
	// the handler (and its parse) is skipped entirely.
	active := make(plugin.NamedQueries, len(queries))
	for key, q := range queries {
		if !q.MatchContent(sourceCode) {
			queryResults <- &plugin.QueryProcessorResult{
				Key:    key,
				Result: emptyResult(queryType),
			}
			continue
		}
		active[key] = q
	}
	if len(active) == 0 {
		return nil
	}

	switch queryType {
	case plugin.QueryTypeAst:
		return runPluginTreeQueries(fileName, sourceCode, active, queryResults)
	case plugin.QueryTypeRegex:
		return runRegexQueries(sourceCode, active, queryResults)
	case plugin.QueryTypeJson:
		return runJsonQueries(fileName, sourceCode, active, queryResults)
	case plugin.QueryTypeYaml:
		return runYamlQueries(fileName, sourceCode, active, queryResults)
	case plugin.QueryTypeToml:
		return runTomlQueries(fileName, sourceCode, active, queryResults)
	case plugin.QueryTypeRaw:
		return runRawQueries(fileName, sourceCode, active, queryResults)
	default:
		BazelLog.Fatalf("Unknown query type: %v", queryType)
		return nil
	}
}

// emptyResult is the result a handler of the given type emits for no match,
// used for queries skipped by the content gate.
func emptyResult(queryType plugin.QueryType) any {
	switch queryType {
	case plugin.QueryTypeRaw:
		return ""
	case plugin.QueryTypeJson, plugin.QueryTypeYaml, plugin.QueryTypeToml:
		return []interface{}{}
	default: // ast, regex
		return plugin.QueryMatches(nil)
	}
}

func runRawQueries(fileName string, sourceCode []byte, queries plugin.NamedQueries, queryResults chan *plugin.QueryProcessorResult) error {
	sourceCodeStr := string(sourceCode)
	for key := range queries {
		queryResults <- &plugin.QueryProcessorResult{
			Key:    key,
			Result: sourceCodeStr,
		}
	}
	return nil
}
