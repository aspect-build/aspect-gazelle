package gazelle

import (
	"testing"

	"github.com/aspect-build/aspect-gazelle/language/orion/plugin"
)

// QueryDefinition is satisfiable only by pointers to the *Query structs,
// allowing query processors to safely type-assert on the pointer forms.
var _ plugin.QueryDefinition = (*plugin.AstQuery)(nil)
var _ plugin.QueryDefinition = (*plugin.RegexQuery)(nil)
var _ plugin.QueryDefinition = (*plugin.JsonQuery)(nil)
var _ plugin.QueryDefinition = (*plugin.YamlQuery)(nil)
var _ plugin.QueryDefinition = (*plugin.TomlQuery)(nil)
var _ plugin.QueryDefinition = (*plugin.RawQuery)(nil)

func queryBase(filter ...string) plugin.QueryBase {
	return plugin.QueryBase{
		Filter:     filter,
		FilterExpr: func(string) bool { return true },
	}
}

func allQueryTypes() plugin.NamedQueries {
	return plugin.NamedQueries{
		"ast":   &plugin.AstQuery{QueryBase: queryBase("*.ts"), Grammar: "typescript", Query: "(import_statement)"},
		"regex": &plugin.RegexQuery{QueryBase: queryBase("*.txt"), Expression: "import (?P<name>.*)"},
		"json":  &plugin.JsonQuery{QueryBase: queryBase("*.json"), Query: ".dependencies"},
		"yaml":  &plugin.YamlQuery{QueryBase: queryBase("*.yaml"), Query: ".jobs"},
		"toml":  &plugin.TomlQuery{QueryBase: queryBase("*.toml"), Query: ".project"},
		"raw":   &plugin.RawQuery{QueryBase: queryBase("*.svg")},
	}
}

func TestComputeQueriesCacheKey(t *testing.T) {
	// Must not fail or panic for any query type, despite the func-valued
	// FilterExpr (ignored by gob) and the lack of gob type registration.
	key := computeQueriesCacheKey(allQueryTypes())
	if key == "" {
		t.Fatal("expected a non-empty cache key")
	}

	t.Run("deterministic", func(t *testing.T) {
		if k2 := computeQueriesCacheKey(allQueryTypes()); k2 != key {
			t.Errorf("cache key not deterministic: %q != %q", k2, key)
		}
	})

	t.Run("ignores FilterExpr identity", func(t *testing.T) {
		queries := allQueryTypes()
		for _, q := range queries {
			switch q := q.(type) {
			case *plugin.RegexQuery:
				q.FilterExpr = func(string) bool { return false }
			}
		}
		if k2 := computeQueriesCacheKey(queries); k2 != key {
			t.Errorf("cache key changed with FilterExpr: %q != %q", k2, key)
		}
	})

	t.Run("sensitive to query content", func(t *testing.T) {
		queries := allQueryTypes()
		queries["regex"] = &plugin.RegexQuery{QueryBase: queryBase("*.txt"), Expression: "require (?P<name>.*)"}
		if k2 := computeQueriesCacheKey(queries); k2 == key {
			t.Error("cache key did not change with query expression")
		}
	})

	t.Run("sensitive to filter patterns", func(t *testing.T) {
		queries := allQueryTypes()
		queries["regex"] = &plugin.RegexQuery{QueryBase: queryBase("*.md"), Expression: "import (?P<name>.*)"}
		if k2 := computeQueriesCacheKey(queries); k2 == key {
			t.Error("cache key did not change with filter patterns")
		}
	})

	t.Run("sensitive to query type", func(t *testing.T) {
		a := computeQueriesCacheKey(plugin.NamedQueries{
			"q": &plugin.JsonQuery{QueryBase: queryBase("*"), Query: ".x"},
		})
		b := computeQueriesCacheKey(plugin.NamedQueries{
			"q": &plugin.YamlQuery{QueryBase: queryBase("*"), Query: ".x"},
		})
		if a == b {
			t.Error("cache key did not distinguish query types with equal content")
		}
	})

	t.Run("sensitive to query name", func(t *testing.T) {
		a := computeQueriesCacheKey(plugin.NamedQueries{
			"q1": &plugin.RawQuery{QueryBase: queryBase("*")},
		})
		b := computeQueriesCacheKey(plugin.NamedQueries{
			"q2": &plugin.RawQuery{QueryBase: queryBase("*")},
		})
		if a == b {
			t.Error("cache key did not distinguish query names")
		}
	})
}
