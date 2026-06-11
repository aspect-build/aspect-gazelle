package queries

import (
	BazelLog "github.com/aspect-build/aspect-gazelle/common/logger"
	"github.com/aspect-build/aspect-gazelle/language/orion/plugin"
)

func RunQueries(fileName string, sourceCode []byte, queries plugin.NamedQueries) (plugin.QueryResults, error) {
	// Content gate: a query with a ContentFilter only runs if the source matches
	// it. Queries gated out get an empty result; that may leave a type with no
	// active queries, skipping its handler (and its parse) entirely. Surviving
	// queries are grouped by type to run as a single batch per type.
	var gated plugin.QueryResults
	activeByType := make(map[plugin.QueryType]plugin.NamedQueries)
	for key, q := range queries {
		queryType := q.QueryType()
		if !q.MatchContent(sourceCode) {
			if gated == nil {
				gated = make(plugin.QueryResults)
			}
			gated[key] = emptyResult(queryType)
			continue
		}
		if activeByType[queryType] == nil {
			activeByType[queryType] = make(plugin.NamedQueries)
		}
		activeByType[queryType][key] = q
	}

	var results plugin.QueryResults
	for queryType, active := range activeByType {
		batch, err := runQueryBatch(queryType, fileName, sourceCode, active)
		if err != nil {
			return nil, err
		}

		if results == nil {
			results = batch
		} else {
			for key, r := range batch {
				results[key] = r
			}
		}
	}

	// Fold the gated empties in, adopting their map if no batch produced one.
	if results == nil {
		results = gated
	} else {
		for key, r := range gated {
			results[key] = r
		}
	}
	if results == nil {
		results = plugin.QueryResults{}
	}
	return results, nil
}

// runQueryBatch runs the active queries of a single type.
func runQueryBatch(queryType plugin.QueryType, fileName string, sourceCode []byte, active plugin.NamedQueries) (plugin.QueryResults, error) {
	switch queryType {
	case plugin.QueryTypeAst:
		return runPluginTreeQueries(fileName, sourceCode, active)
	case plugin.QueryTypeRegex:
		return runRegexQueries(sourceCode, active)
	case plugin.QueryTypeJson:
		return runJsonQueries(sourceCode, active)
	case plugin.QueryTypeYaml:
		return runYamlQueries(sourceCode, active)
	case plugin.QueryTypeToml:
		return runTomlQueries(sourceCode, active)
	case plugin.QueryTypeRaw:
		return runRawQueries(sourceCode, active)
	default:
		BazelLog.Fatalf("Unknown query type: %v", queryType)
		return nil, nil
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

func runRawQueries(sourceCode []byte, queries plugin.NamedQueries) (plugin.QueryResults, error) {
	sourceCodeStr := string(sourceCode)
	results := make(plugin.QueryResults, len(queries))
	for key := range queries {
		results[key] = sourceCodeStr
	}
	return results, nil
}
