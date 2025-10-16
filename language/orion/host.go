package gazelle

/**
 * A Gazelle language.Language implementation hosting and delegating to one
 * or more orion starlark extensions.
 */

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/aspect-build/aspect-gazelle/common/bazel/workspace"
	BazelLog "github.com/aspect-build/aspect-gazelle/common/logger"
	plugin "github.com/aspect-build/aspect-gazelle/language/orion/plugin"
	starzelle "github.com/aspect-build/aspect-gazelle/language/orion/starzelle"
	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bazelbuild/bazel-gazelle/label"
	gazelleLanguage "github.com/bazelbuild/bazel-gazelle/language"
	"github.com/bazelbuild/bazel-gazelle/rule"
	"github.com/emirpasic/gods/sets/treeset"
)

const GazelleLanguageName = "orion"

// A gazelle
type GazelleHost struct {
	database *plugin.Database

	// Hosted plugins
	// TODO: support enabling/disabling/adding in subdirs
	pluginIds []plugin.PluginId
	plugins   map[plugin.PluginId]plugin.Plugin

	// Metadata about rules being generated. May be pre-configured, potentially loaded from *.star etc
	kinds           map[string]plugin.RuleKind
	sourceRuleKinds *treeset.Set

	// Lazy loaded from plugins
	gazelleDirectives []string
	gazelleLoadInfo   []rule.LoadInfo
	gazelleKindInfo   map[string]rule.KindInfo
}

var _ gazelleLanguage.Language = (*GazelleHost)(nil)
var _ gazelleLanguage.ModuleAwareLanguage = (*GazelleHost)(nil)
var _ plugin.PluginHost = (*GazelleHost)(nil)

func NewLanguage(plugins ...string) gazelleLanguage.Language {
	l := &GazelleHost{
		plugins:         make(map[string]plugin.Plugin),
		kinds:           make(map[string]plugin.RuleKind),
		sourceRuleKinds: treeset.NewWithStringComparator(),
		database:        &plugin.Database{},
	}

	// Initialize with builtin kinds. Plugins can add/overwrite these.
	for _, k := range builtinKinds {
		l.kinds[k.Name] = k
	}

	l.loadStarzellePlugins(plugins)
	l.loadEnvStarzellePlugins()

	return l
}

func (h *GazelleHost) loadStarzellePlugins(plugins []string) {
	if len(plugins) == 0 {
		return
	}

	wd, cwdErr := os.Getwd()
	if cwdErr != nil {
		BazelLog.Fatalf("Failed to find CWD: %v", cwdErr)
		return
	}

	// Load starzelle plugins configured in the aspect-cli config.yaml
	wr, wrErr := workspace.DefaultFinder.Find(wd)
	if wrErr != nil {
		BazelLog.Fatalf("Failed to find bazel workspace: %v", wrErr)
		return
	}

	BazelLog.Infof("Loading %v orion plugins from %q: %v", len(plugins), wd, plugins)

	for _, plugin := range plugins {
		h.LoadPlugin(wr, plugin)
	}
}

func (h *GazelleHost) loadEnvStarzellePlugins() {
	// Add plugins configured via env
	builtinPluginDir := os.Getenv("ORION_EXTENSIONS")
	builtinPluginSubdir := "."

	if builtinPluginDir == "" {
		// Noop if env is not set and not running tests
		if os.Getenv("BAZEL_TEST") != "1" {
			BazelLog.Tracef("No ORION_EXTENSIONS environment variable set")
			return
		}

		// Load from runfiles + TEST_ORION_EXTENSIONS if running tests
		builtinPluginDir = path.Join(os.Getenv("RUNFILES_DIR"), os.Getenv("TEST_WORKSPACE"))
		builtinPluginSubdir = os.Getenv("TEST_ORION_EXTENSIONS")
	}

	if !filepath.IsAbs(builtinPluginDir) {
		BazelLog.Fatalf("ORION_EXTENSIONS must be an absolute path, got %q", builtinPluginDir)
		return
	}

	builtinPlugins, err := filepath.Glob(path.Join(builtinPluginDir, builtinPluginSubdir, "*.axl"))
	if err != nil {
		BazelLog.Fatalf("Failed to find builtin plugins: %v", err)
		return
	}

	if len(builtinPlugins) == 0 {
		BazelLog.Warnf("No orion plugins found in %q", builtinPluginDir)
		return
	}

	// Sort to ensure a consistent order not dependent on the fs or glob ordering.
	sort.Strings(builtinPlugins)

	// Split the plugin paths to dir + rel for better logging and load API
	// Only relativize if builtinPluginDir is not absolute
	for i, p := range builtinPlugins {
		if relPath, err := filepath.Rel(builtinPluginDir, p); err == nil {
			builtinPlugins[i] = relPath
		} else {
			// Fallback to original path if relativization fails
			builtinPlugins[i] = p
		}
	}

	BazelLog.Infof("Loading %v orion env plugins from %q: %v", len(builtinPlugins), builtinPluginDir, builtinPlugins)

	for _, p := range builtinPlugins {
		h.LoadPlugin(builtinPluginDir, p)
	}
}
func (h *GazelleHost) LoadPlugin(pluginDir, pluginPath string) {
	// Can not add new plugins after configuration/data-collection has started
	if h.gazelleKindInfo != nil || h.gazelleLoadInfo != nil {
		BazelLog.Fatalf("Cannot add plugin %q after configuration has started", pluginPath)
		return
	}

	err := starzelle.LoadProxy(h, pluginDir, pluginPath)
	if err != nil {
		BazelLog.Infof("Failed to load orion plugin %q/%q: %v\n", pluginDir, pluginPath, err)

		// Try to remove the `parentDir` from the error message to align paths
		// with the user's workspace relative paths, and to remove sandbox paths
		// when run in tests.
		errStr := strings.ReplaceAll(err.Error(), pluginDir+"/", "")

		fmt.Printf("Failed to load orion plugin %q: %v\n", pluginPath, errStr)
		return
	}
}

func (h *GazelleHost) AddPlugin(plugin plugin.Plugin) {
	if _, exists := h.plugins[plugin.Name()]; exists {
		BazelLog.Errorf("Duplicate plugin %q", plugin.Name())
	}

	BazelLog.Infof("Plugin added: %q", plugin.Name())
	h.pluginIds = append(h.pluginIds, plugin.Name())
	h.plugins[plugin.Name()] = plugin
}

func (h *GazelleHost) AddKind(k plugin.RuleKind) {
	if _, exists := h.kinds[k.Name]; exists {
		BazelLog.Errorf("Duplicate rule kind %q", k.Name)
	}

	BazelLog.Infof("Kind added: %q", k.Name)
	h.kinds[k.Name] = k

	// Clear cached plugin.RuleKind => gazelle mapping.
	h.gazelleKindInfo = nil
	h.gazelleLoadInfo = nil
}

func (h *GazelleHost) Kinds() map[string]rule.KindInfo {
	if h.gazelleKindInfo == nil {
		h.gazelleKindInfo = make(map[string]rule.KindInfo, len(h.kinds))

		// Configured by plugins, potentially overriding builtin
		for k, v := range h.kinds {
			h.gazelleKindInfo[k] = rule.KindInfo{
				MatchAny:        v.MatchAny,
				MatchAttrs:      v.MatchAttrs,
				NonEmptyAttrs:   toKeyTrueMap(v.NonEmptyAttrs),
				MergeableAttrs:  toKeyTrueMap(v.MergeableAttrs),
				ResolveAttrs:    toKeyTrueMap(v.ResolveAttrs),
				SubstituteAttrs: make(map[string]bool),
			}
			h.sourceRuleKinds.Add(k)
		}
	}

	return h.gazelleKindInfo
}

func toKeyTrueMap(keys []string) map[string]bool {
	m := make(map[string]bool, len(keys))
	for _, k := range keys {
		m[k] = true
	}
	return m
}

func (h *GazelleHost) Loads() []rule.LoadInfo {
	panic("ApparentLoads should be called instead")
}

func (h *GazelleHost) ApparentLoads(moduleToApparentName func(string) string) []rule.LoadInfo {
	if h.gazelleLoadInfo == nil {
		h.gazelleLoadInfo = make([]rule.LoadInfo, 0, len(h.kinds))

		loads := make(map[string]*rule.LoadInfo)

		for name, r := range h.kinds {
			if r.From == "" {
				continue
			}

			from, err := label.Parse(r.From)
			if err != nil {
				BazelLog.Errorf("Failed to parse label %q: %v", r.From, err)
				fmt.Printf("Invalid rule 'From' label %q: %v", r.From, err)
				continue
			}

			// Map external repo names to apparent names
			if from.Repo != "" {
				apparentName := moduleToApparentName(from.Repo)
				if apparentName != "" {
					from.Repo = apparentName
				}
			}

			fromStr := from.String()

			if loads[fromStr] == nil {
				loads[fromStr] = &rule.LoadInfo{
					Name:    fromStr,
					Symbols: make([]string, 0, 1),
					After:   make([]string, 0),
				}
			}

			loads[fromStr].Symbols = append(loads[fromStr].Symbols, name)
		}

		for _, load := range loads {
			h.gazelleLoadInfo = append(h.gazelleLoadInfo, *load)
		}
	}

	return h.gazelleLoadInfo
}

func (*GazelleHost) Fix(c *config.Config, f *rule.File) {
	// Unsupported
}
