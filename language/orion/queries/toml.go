package queries

import (
	"bytes"

	"github.com/aspect-build/aspect-gazelle/language/orion/plugin"
	"github.com/mikefarah/yq/v4/pkg/yqlib"
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
	for key, q := range queries {
		r, err := runYamlQuery(node, q.(*plugin.TomlQuery).Query)
		if err != nil {
			return nil, err
		}
		results[key] = r
	}
	return results, nil
}
