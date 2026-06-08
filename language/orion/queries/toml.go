package queries

import (
	"bytes"
	"sync"

	"github.com/aspect-build/aspect-gazelle/language/orion/plugin"
	"github.com/mikefarah/yq/v4/pkg/yqlib"
	"golang.org/x/sync/errgroup"
)

func runTomlQueries(fileName string, sourceCode []byte, queries plugin.NamedQueries) (plugin.QueryResults, error) {
	decoder := yqlib.NewTomlDecoder()
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
			r, err := runYamlQuery(node, q.(*plugin.TomlQuery).Query)
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
