package queries

import (
	"testing"

	"github.com/aspect-build/aspect-gazelle/language/orion/plugin"
)

// A YAML mapping key is not required to be a string: integers, booleans, and
// even nested mappings/sequences are valid keys. Such a document must not
// crash the gazelle run.
func TestYamlNonStringMappingKey(t *testing.T) {
	tests := map[string]string{
		"integer key": "123: foo\n",
		"boolean key": "true: foo\n",
		"float key":   "1.5: foo\n",
	}

	for name, src := range tests {
		t.Run(name, func(t *testing.T) {
			queries := plugin.NamedQueries{"q": &plugin.YamlQuery{Query: "."}}

			// Before the fix this panicked on an unchecked key.(string).
			results, err := runYamlQueries([]byte(src), queries)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if results["q"] == nil {
				t.Fatalf("expected a result for query %q, got nil", "q")
			}
		})
	}
}
