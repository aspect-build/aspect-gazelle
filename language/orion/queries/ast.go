package queries

import (
	BazelLog "github.com/aspect-build/aspect-gazelle/common/logger"
	"github.com/aspect-build/aspect-gazelle/language/orion/plugin"
	"github.com/aspect-build/aspect-gazelle/language/orion/query"
)

func runPluginTreeQueries(fileName string, sourceCode []byte, queries plugin.NamedQueries, queryResults chan *plugin.QueryProcessorResult) error {
	grammar := toTreeGrammar(fileName, queries)

	// Collect the query strings in a stable order, remembering each query's key
	// so results (returned by index) can be mapped back.
	keys := make([]string, 0, len(queries))
	queryStrings := make([]string, 0, len(queries))
	for key, q := range queries {
		keys = append(keys, key)
		queryStrings = append(queryStrings, q.(*plugin.AstQuery).Query)
	}

	// Parse the file once and run every query in a single FFI call.
	results, parseErrors, err := query.Query(grammar, fileName, sourceCode, queryStrings)
	if err != nil {
		return err
	}

	// Parse errors. Only log them due to many false positives.
	// TODO: what false positives? See js plugin where this is from
	if BazelLog.IsTraceEnabled() && len(parseErrors) > 0 {
		BazelLog.Tracef("TreeSitter query errors: %v", parseErrors)
	}

	for i, key := range keys {
		matches := plugin.QueryMatches(nil)
		for _, captures := range results[i] {
			matches = append(matches, plugin.NewQueryMatch(captures, nil))
		}

		queryResults <- &plugin.QueryProcessorResult{
			Key:    key,
			Result: matches,
		}
	}

	return nil
}

func toTreeGrammar(fileName string, queries plugin.NamedQueries) query.Grammar {
	// TODO: fail if queries on the same file use different languages?

	for _, q := range queries {
		grammar := q.(*plugin.AstQuery).Grammar
		if grammar != "" {
			return query.Grammar(grammar)
		}
	}

	return query.PathToGrammar(fileName)
}
