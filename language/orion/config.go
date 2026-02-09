package gazelle

import (
	"iter"

	plugin "github.com/aspect-build/aspect-gazelle/language/orion/plugin"
)

type BUILDConfig struct {
	// Shared across all
	repoName string

	// This config
	rel    string
	parent *BUILDConfig

	// If this BUILD has been generated during this execution
	generated bool

	// All directives of this BUILD
	directiveRawValues map[string][]string

	// Plugin specific config
	pluginPrepareResults map[plugin.PluginId]pluginConfig
}

func NewRootConfig(repoName string) *BUILDConfig {
	return &BUILDConfig{
		repoName: repoName,
		rel:      "",

		directiveRawValues: make(map[string][]string),

		pluginPrepareResults: make(map[string]pluginConfig),
	}
}

func (c *BUILDConfig) NewChildConfig(rel string) *BUILDConfig {
	// TODO: freeze the parent config now that a child has copied/inherited it.

	cCopy := *c

	// Child specific
	cCopy.generated = false
	cCopy.rel = rel
	cCopy.parent = c
	cCopy.directiveRawValues = make(map[string][]string)

	// Non-inherited that require cloning
	// TODO: verify these should not be inherited
	cCopy.pluginPrepareResults = make(map[string]pluginConfig)

	return &cCopy
}

func (p *BUILDConfig) appendDirectiveValue(key, value string) {
	values, valueExists := p.directiveRawValues[key]
	if !valueExists {
		p.directiveRawValues[key] = []string{value}
	} else {
		p.directiveRawValues[key] = append(values, value)
	}
}

func (c *BUILDConfig) IsPluginEnabled(pluginId plugin.PluginId) bool {
	if val, exists := c.getRawValue(string(pluginId), true); exists {
		return val[len(val)-1] == "enabled"
	}
	return true
}

func (c *BUILDConfig) getRawValue(key string, inherit bool) ([]string, bool) {
	value, exists := c.directiveRawValues[key]
	if exists {
		return value, true
	}

	if inherit && c.parent != nil {
		return c.parent.getRawValue(key, true)
	}

	return nil, false
}

// An extension of PrepareContext+Result to add internal utils
type pluginConfig struct {
	plugin.PrepareContext
	plugin.PrepareResult

	// Hash of all query definitions configured for this plugin in this context.
	// Precomputed at configure time for potential use as a cache key.
	queriesHash string
}

func (c *pluginConfig) getQueriesForFile(f string) iter.Seq2[string, plugin.QueryDefinition] {
	return func(yield func(string, plugin.QueryDefinition) bool) {
		for n, query := range c.PrepareResult.Queries {
			if query.Match(f) {
				if !yield(n, query) {
					return
				}
			}
		}
	}
}
