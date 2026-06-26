package queries

import (
	"encoding/json"
	"sync"

	"github.com/goexlib/jsonc"
	"github.com/itchyny/gojq"

	BazelLog "github.com/aspect-build/aspect-gazelle/common/logger"
	"github.com/aspect-build/aspect-gazelle/language/orion/plugin"
)

func runJsonQueries(fileName string, sourceCode []byte, queries plugin.NamedQueries) (plugin.QueryResults, error) {
	// Strip JSONC (comments, trailing commas); skip a malformed/empty file with
	// no results rather than aborting the run.
	var doc interface{}
	if err := json.Unmarshal(jsonc.Strip(sourceCode), &doc); err != nil {
		BazelLog.Warnf("ignoring unparseable JSON file %q: %v", fileName, err)
	}

	results := make(plugin.QueryResults, len(queries))
	for key, q := range queries {
		if doc == nil {
			results[key] = make([]interface{}, 0)
			continue
		}
		r, err := runJsonQuery(doc, q.(*plugin.JsonQuery).Query)
		if err != nil {
			return nil, err
		}
		results[key] = r
	}
	return results, nil
}

func runJsonQuery(doc interface{}, query string) (interface{}, error) {
	q, err := parseJsonQuery(query)
	if err != nil {
		return nil, err
	}

	matches := make([]interface{}, 0)

	iter := q.Run(doc)
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}

		// See error snippet and notes: https://pkg.go.dev/github.com/itchyny/gojq#readme-usage-as-a-library
		if err, ok := v.(error); ok {
			if err, ok := err.(*gojq.HaltError); ok && err.Value() == nil {
				break
			}
			return nil, err
		}

		matches = append(matches, v)
	}

	return matches, nil
}

var jqQueryCache = sync.Map{}

func parseJsonQuery(query string) (*gojq.Code, error) {
	q, loaded := jqQueryCache.Load(query)
	if !loaded {
		p, err := gojq.Parse(query)
		if err != nil {
			return nil, err
		}
		q, err = gojq.Compile(p)
		if err != nil {
			return nil, err
		}
		q, _ = jqQueryCache.LoadOrStore(query, q)
	}

	return q.(*gojq.Code), nil
}
