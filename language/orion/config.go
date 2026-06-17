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

	// Plugin-private inherited data (ctx.data), local to this directory; reads walk the parent chain.
	pluginData map[plugin.PluginId]map[string]any
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

	// Local plugin data; inherited values are reached via the parent chain.
	cCopy.pluginData = nil

	// Non-inherited that require cloning
	// TODO: verify these should not be inherited
	cCopy.pluginPrepareResults = make(map[string]pluginConfig)

	return &cCopy
}

func (p *BUILDConfig) appendDirectiveValue(key, value string) {
	p.directiveRawValues[key] = append(p.directiveRawValues[key], value)
}

func (c *BUILDConfig) IsPluginEnabled(pluginId plugin.PluginId) bool {
	if val, exists, _ := c.getRawValue(string(pluginId)); exists {
		return val[len(val)-1] == "enabled"
	}
	return true
}

// getRawValue returns the directive values for `key`, walking up to ancestors.
// `local` reports whether the match came from this directory's own BUILD file
// rather than being inherited from an ancestor.
func (c *BUILDConfig) getRawValue(key string) (value []string, found bool, local bool) {
	if v, exists := c.directiveRawValues[key]; exists {
		return v, true, true
	}

	if c.parent != nil {
		v, f, _ := c.parent.getRawValue(key)
		return v, f, false
	}

	return nil, false, false
}

// setPluginDataMap records the plugin-private data map written by `pluginId`
// during this directory's prepare stage.
func (c *BUILDConfig) setPluginDataMap(pluginId plugin.PluginId, data map[string]any) {
	if c.pluginData == nil {
		c.pluginData = make(map[plugin.PluginId]map[string]any)
	}
	c.pluginData[pluginId] = data
}

// getPluginData returns the value `pluginId` associated with `key` in this
// directory or, failing that, the nearest ancestor that set it.
func (c *BUILDConfig) getPluginData(pluginId plugin.PluginId, key string) (any, bool) {
	if d, ok := c.pluginData[pluginId]; ok {
		if v, ok := d[key]; ok {
			return v, true
		}
	}

	if c.parent != nil {
		return c.parent.getPluginData(pluginId, key)
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
			if query.MatchPath(f) {
				if !yield(n, query) {
					return
				}
			}
		}
	}
}
