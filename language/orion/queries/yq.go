package queries

import (
	"bytes"
	"sync"

	BazelLog "github.com/aspect-build/aspect-gazelle/common/logger"
	"github.com/aspect-build/aspect-gazelle/language/orion/plugin"
	"github.com/mikefarah/yq/v4/pkg/yqlib"
	"golang.org/x/sync/errgroup"
)

func runYamlQueries(fileName string, sourceCode []byte, queries plugin.NamedQueries) (plugin.QueryResults, error) {
	decoder := yqlib.NewYamlDecoder(yqlib.ConfiguredYamlPreferences)
	err := decoder.Init(bytes.NewReader(sourceCode))
	if err != nil {
		return nil, err
	}
	node, err := decoder.Decode()
	if err != nil {
		return nil, err
	}

	results := make(plugin.QueryResults, len(queries))
	var mu sync.Mutex

	eg := errgroup.Group{}
	eg.SetLimit(10)

	for key, q := range queries {
		// Capture loop variables for goroutine
		key := key
		q := q
		eg.Go(func() error {
			r, err := runYamlQuery(node, q.(*plugin.YamlQuery).Query)
			if err != nil {
				return err
			}

			mu.Lock()
			results[key] = r
			mu.Unlock()
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}
	return results, nil
}

func runYamlQuery(node *yqlib.CandidateNode, query string) (interface{}, error) {
	var evaluator = yqlib.NewAllAtOnceEvaluator()
	results, err := evaluator.EvaluateNodes(query, node)
	if err != nil {
		return nil, err
	}

	matches := make([]interface{}, 0, results.Len())
	for e := results.Front(); e != nil; e = e.Next() {
		value := convertYqNodeToValue(e.Value.(*yqlib.CandidateNode))
		matches = append(matches, value)
	}

	return matches, nil
}

func convertYqNodeToValue(node *yqlib.CandidateNode) interface{} {
	switch node.Kind {
	case yqlib.MappingNode:
		m := make(map[string]interface{}, len(node.Content)/2)
		for i := 0; i < len(node.Content); i += 2 {
			key := convertYqNodeToValue(node.Content[i])
			value := convertYqNodeToValue(node.Content[i+1])
			m[key.(string)] = value
		}
		return m
	case yqlib.SequenceNode:
		s := make([]interface{}, 0, len(node.Content))
		for _, n := range node.Content {
			s = append(s, convertYqNodeToValue(n))
		}
		return s
	case yqlib.AliasNode:
		return convertYqNodeToValue(node.Alias)
	case yqlib.ScalarNode:
		val, err := node.GetValueRep()
		if err != nil {
			return node.Value
		}
		return val
	default:
		BazelLog.Fatalf("Unknown yq node kind: %v", node.Kind)
		return nil
	}
}
