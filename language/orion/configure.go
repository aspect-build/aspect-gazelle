package gazelle

import (
	"flag"
	"strconv"
	"sync"

	common "github.com/aspect-build/aspect-gazelle/common"
	BazelLog "github.com/aspect-build/aspect-gazelle/common/logger"
	"github.com/aspect-build/aspect-gazelle/language/orion/plugin"
	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bazelbuild/bazel-gazelle/rule"
	"golang.org/x/sync/errgroup"
)

var _ config.Configurer = (*GazelleHost)(nil)

func (c *GazelleHost) KnownDirectives() []string {
	if c.gazelleDirectives == nil {
		c.gazelleDirectives = []string{}

		// TODO: verify no collisions with other plugins/globals

		for _, plugin := range c.plugins {
			// A directive to enable/disable the plugin
			c.gazelleDirectives = append(c.gazelleDirectives, plugin.Name())

			// Directives defined by the plugin
			for _, dir := range plugin.Properties() {
				c.gazelleDirectives = append(c.gazelleDirectives, dir.Name)
			}
		}
	}

	return c.gazelleDirectives
}

func (configurer *GazelleHost) Configure(c *config.Config, rel string, f *rule.File) {
	BazelLog.Tracef("Configure(%s): %s", GazelleLanguageName, rel)

	// Generate hierarchical configuration.
	var config *BUILDConfig
	if rel == "" {
		config = NewRootConfig(c.RepoName)
	} else {
		config = c.Exts[GazelleLanguageName].(*BUILDConfig).NewChildConfig(rel)
	}
	c.Exts[GazelleLanguageName] = config

	// Record directives from the existing BUILD file.
	if f != nil {
		for _, d := range f.Directives {
			config.appendDirectiveValue(d.Key, d.Value)
		}
	}

	eg := errgroup.Group{}
	eg.SetLimit(10)

	var prepResultMutex sync.Mutex

	// ctx.has_file: lazily report whether this directory contains a file (or a
	// `sub/dir/file` relative path). Backed by the gazelle walk cache, queried
	// only when a plugin actually calls has_file.
	hasFile := func(name string) bool {
		return common.WalkHasPath(rel, name)
	}

	// Prepare the plugins for this configuration.
	for k, p := range configurer.plugins {
		if !config.IsPluginEnabled(k) {
			continue
		}

		// Capture loop variables for goroutine
		k := k
		p := p
		eg.Go(func() error {
			prepContext := configToPrepareContext(p, config)
			prepContext.HasFile = hasFile

			// ctx.data: plugin-private inherited store. Writable during prepare,
			// reads fall through to the nearest ancestor that wrote the key.
			inherit := func(key string) (any, bool) {
				if config.parent != nil {
					return config.parent.getPluginData(k, key)
				}
				return nil, false
			}
			prepContext.Data = plugin.NewPluginData(inherit)

			prepResult := p.Prepare(prepContext)

			// Writes are only allowed during prepare; the same store is reused
			// read-only for the analyze/declare stages.
			prepContext.Data.Seal()

			// Lock while modifying config.pluginPrepareResults / pluginData
			prepResultMutex.Lock()
			defer prepResultMutex.Unlock()

			// Persist any writes from this directory so descendants can inherit them.
			if data := prepContext.Data.Local(); data != nil {
				config.setPluginDataMap(k, data)
			}

			// Index the plugins and their PrepareResult
			config.pluginPrepareResults[k] = pluginConfig{
				PrepareContext: prepContext,
				PrepareResult:  prepResult,

				// Precompute the queriesHash for all of this plugin's queries
				queriesHash: computeQueriesCacheKey(prepResult.Queries),
			}

			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		BazelLog.Errorf("Configure(%s) plugin error: %v", GazelleLanguageName, err)
	}
}

func configToPrepareContext(p plugin.Plugin, cfg *BUILDConfig) plugin.PrepareContext {
	props := p.Properties()
	ctx := plugin.PrepareContext{
		RepoName: cfg.repoName,
		Rel:      cfg.rel,
		// Defaults are resolved lazily at read time; only record directive-set values.
		Properties: plugin.NewPropertyValues(props),
	}

	for k, p := range props {
		// `local` marks a property set by a directive in this directory's own BUILD
		// file (not inherited), letting plugins detect where a marker is declared.
		v, found, local := cfg.getRawValue(p.Name)
		if !found {
			continue
		}

		// A present-but-unparseable directive keeps the default value, but the
		// property still counts as set (and local) at this directory.
		value := p.Default
		if parsedValue, parseErr := parsePropertyValue(p, v); parseErr != nil {
			BazelLog.Warnf("Failed to parse property %q: %v", p.Name, parseErr)
		} else {
			value = parsedValue
		}

		ctx.Properties.Add(k, value, local)
	}

	return ctx
}

func getBUILDConfig(c *config.Config, rel string) *BUILDConfig {
	cfg, ok := c.Exts[GazelleLanguageName].(*BUILDConfig)
	if !ok || cfg == nil {
		BazelLog.Fatalf("Expected BUILDConfig in config.Exts[%q], got %T", GazelleLanguageName, c.Exts[GazelleLanguageName])
	}
	if cfg.rel != rel {
		BazelLog.Fatalf("Mismatched BUILDConfig rel:%q, expected:%q", cfg.rel, rel)
	}
	return cfg
}

func parsePropertyValue(p plugin.Property, values []string) (interface{}, error) {
	switch p.PropertyType {
	case plugin.PropertyType_String:
		return onlyValue(p, values), nil
	case plugin.PropertyType_Strings:
		return values, nil
	case plugin.PropertyType_Bool:
		return onlyValue(p, values) == "true", nil
	case plugin.PropertyType_Number:
		return strconv.ParseInt(onlyValue(p, values), 10, 0)
	}

	panic("unhandled property type: " + p.PropertyType)
}

func onlyValue(p plugin.Property, value []string) string {
	c := len(value)

	if c == 0 {
		BazelLog.Fatalf("expected exactly one value, got none")
		return ""
	} else if c > 1 {
		BazelLog.Warnf("expected exactly one value for %q, got %d", p.Name, c)
	}

	return value[c-1]
}

func (c *GazelleHost) RegisterFlags(fs *flag.FlagSet, cmd string, cfg *config.Config) {
}

func (c *GazelleHost) CheckFlags(fs *flag.FlagSet, cfg *config.Config) error {
	return nil
}
