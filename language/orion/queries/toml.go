package queries

import (
	"bytes"

	"github.com/aspect-build/aspect-gazelle/language/orion/plugin"
	"github.com/mikefarah/yq/v4/pkg/yqlib"
	"golang.org/x/sync/errgroup"
)

func runTomlQueries(fileName string, sourceCode []byte, queries plugin.NamedQueries, queryResults chan *plugin.QueryProcessorResult) error {
	decoder := yqlib.NewTomlDecoder()
	err := decoder.Init(bytes.NewReader(sourceCode))
	if err != nil {
		return err
	}
	node, err := decoder.Decode()
	if err != nil {
		return err
	}

	eg := errgroup.Group{}
	eg.SetLimit(10)

	for key, q := range queries {
		// Capture loop variables for goroutine
		key := key
		q := q
		eg.Go(func() error {
			r, err := runYamlQuery(node, q.Params.(plugin.TomlQueryParams))
			if err != nil {
				return err
			}

			queryResults <- &plugin.QueryProcessorResult{
				Key:    key,
				Result: r,
			}
			return nil
		})
	}

	return eg.Wait()
}
