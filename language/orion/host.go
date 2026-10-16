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
	"slices"
	"strings"

	"github.com/aspect-build/aspect-gazelle/common/bazel/workspace"
	BazelLog "github.com/aspect-build/aspect-gazelle/common/logger"
	plugin "github.com/aspect-build/aspect-gazelle/language/orion/plugin"
	stareval "github.com/aspect-build/aspect-gazelle/language/orion/starlark"
	starzelle "github.com/aspect-build/aspect-gazelle/language/orion/starzelle"
	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bazelbuild/bazel-gazelle/label"
	gazelleLanguage "github.com/bazelbuild/bazel-gazelle/language"
	"github.com/bazelbuild/bazel-gazelle/rule"
	"github.com/emirpasic/gods/v2/sets/treeset"
)

const GazelleLanguageName = "orion"

// ExitCodeSetupError is the process exit status for a failure in the user's
// gazelle setup, such as an orion extension that does not parse.
//
// It is deliberately not 1. Gazelle already overloads 1: `--mode=diff` and
// `--mode=print` exit 1 when BUILD files are out of date, and the runner
// binaries exit 1 via log.Fatalf when a run itself fails. A CI job that reads
// exit 1 as "BUILD files are out of date, run gazelle" would therefore report
// a typo in an extension as stale BUILD files -- the same misdiagnosis that
// writing diagnostics to stdout used to cause. 2 follows the bazel and POSIX
// convention of 2 = tool or usage error.
//
// The gazelle runner aliases this constant (runner.ExitCodeSetupError) rather
// than picking its own, so every setup-error exit stays on one value and CI
// can keep telling the two apart. Do not collapse it back onto 1.
const ExitCodeSetupError = 2

// A gazelle
type GazelleHost struct {
	database *plugin.Database

	// Hosted plugins
	// TODO: support enabling/disabling/adding in subdirs
	pluginIds []plugin.PluginId
	plugins   map[plugin.PluginId]plugin.Plugin

	// Metadata about rules being generated. May be pre-configured, potentially loaded from *.star etc
	kinds           map[string]plugin.RuleKind
	sourceRuleKinds *treeset.Set[string]

	// Lazy loaded from plugins
	gazelleDirectives []string
	gazelleLoadInfo   []rule.LoadInfo
	gazelleKindInfo   map[string]rule.KindInfo
}

var _ gazelleLanguage.Language = (*GazelleHost)(nil)
var _ gazelleLanguage.ModuleAwareLanguage = (*GazelleHost)(nil)
var _ plugin.PluginHost = (*GazelleHost)(nil)

// NewLanguage builds the orion host, loading `plugins` plus whatever
// ORION_EXTENSIONS/ORION_EXTENSIONS_DIR name.
//
// `gazelle_binary` generates a call to exactly this signature, so this entry
// point has nowhere to return a load failure to and reports it on stderr and
// exits ExitCodeSetupError. Hosts that can do better than killing the
// process — the runner's --watch loop rebuilds the languages on every cycle,
// so an exit there takes the whole watcher down and strands its IBP
// subscription — must call NewLanguageOrError instead.
func NewLanguage(plugins ...string) gazelleLanguage.Language {
	l, err := NewLanguageOrError(plugins...)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(ExitCodeSetupError)
	}
	return l
}

// NewLanguageOrError is NewLanguage for hosts that can handle a failure
// themselves. A returned error means at least one extension did not load; it
// has not been printed, so the caller decides whether to abort and is
// responsible for reporting it on stderr -- never stdout, which carries the
// generated patch under `gazelle -mode=diff`.
func NewLanguageOrError(plugins ...string) (gazelleLanguage.Language, error) {
	l := newGazelleHost()

	if err := l.loadStarzellePlugins(plugins); err != nil {
		return nil, err
	}
	if err := l.loadEnvStarzellePlugins(); err != nil {
		return nil, err
	}

	return l, nil
}

// newGazelleHost builds an empty host seeded with the builtin kinds, which
// plugins may then add to or overwrite.
func newGazelleHost() *GazelleHost {
	h := &GazelleHost{
		plugins:         make(map[string]plugin.Plugin),
		kinds:           make(map[string]plugin.RuleKind),
		sourceRuleKinds: treeset.NewWith(strings.Compare),
		database:        &plugin.Database{},
	}

	for _, k := range builtinKinds {
		h.kinds[k.Name] = k
	}

	return h
}

func (h *GazelleHost) loadStarzellePlugins(plugins []string) error {
	if len(plugins) == 0 {
		return nil
	}

	wd, cwdErr := os.Getwd()
	if cwdErr != nil {
		return fmt.Errorf("Failed to find CWD: %v", cwdErr)
	}

	// Load starzelle plugins configured in the aspect-cli config.yaml
	wr, wrErr := workspace.DefaultFinder.Find(wd)
	if wrErr != nil {
		return fmt.Errorf("Failed to find bazel workspace: %v", wrErr)
	}

	BazelLog.Infof("Loading %v orion plugins from %q: %v", len(plugins), wd, plugins)

	for _, plugin := range plugins {
		if err := h.LoadPlugin(wr, plugin); err != nil {
			return err
		}
	}

	return nil
}

// resolveEnvPluginPath resolves an ORION_EXTENSIONS entry. Plugins in an
// external repository are not present in the source tree, so fall back to the
// runfiles tree for them. Only when the path names nothing in the source tree,
// so a path deliberately pointing outside the workspace keeps resolving there.
func resolveEnvPluginPath(pluginDir, p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	if _, err := os.Stat(filepath.Join(pluginDir, p)); err == nil {
		return p
	}
	if runfile, ok := stareval.ResolveRunfile(p); ok {
		return runfile
	}
	return p
}

func (h *GazelleHost) loadEnvStarzellePlugins() error {
	builtinPlugins := []string{}

	// Load relative to cwd by default
	builtinPluginDir, _ := os.Getwd()

	if os.Getenv("BAZEL_TEST") != "" {
		// Load from runfiles when running in bazel tests
		builtinPluginDir = path.Join(os.Getenv("RUNFILES_DIR"), os.Getenv("TEST_WORKSPACE"))
	} else if workspaceDir := os.Getenv("BUILD_WORKSPACE_DIRECTORY"); workspaceDir != "" {
		// Load from bazel workspace root when run via bazel
		builtinPluginDir = workspaceDir
	}

	// Comma-separated list of plugin paths relative to the workspace root
	if builtinPluginsList := os.Getenv("ORION_EXTENSIONS"); builtinPluginsList != "" {
		builtinPlugins = append(builtinPlugins, strings.Split(builtinPluginsList, ",")...)
	}

	// Load all plugins in the specified subdirectory of the builtinPluginDir
	if builtinPluginSubdir := os.Getenv("ORION_EXTENSIONS_DIR"); builtinPluginSubdir != "" {
		if !path.IsAbs(builtinPluginSubdir) {
			builtinPluginSubdir = path.Join(builtinPluginDir, builtinPluginSubdir)
		}
		builtinDirPlugins, err := filepath.Glob(path.Join(builtinPluginSubdir, "*.axl"))
		if err != nil {
			return fmt.Errorf("Failed to find builtin plugins: %v", err)
		}

		if len(builtinDirPlugins) == 0 {
			BazelLog.Warnf("No orion plugins found in %q", builtinPluginDir)
		}

		// Sort to ensure a consistent order not dependent on the fs or glob ordering.
		slices.Sort(builtinDirPlugins)

		builtinPlugins = append(builtinPlugins, builtinDirPlugins...)
	}

	if len(builtinPlugins) == 0 {
		return nil
	}

	// Split the plugin paths to dir + rel for better logging and load API.
	for i, p := range builtinPlugins {
		p = resolveEnvPluginPath(builtinPluginDir, p)

		if relPath, err := filepath.Rel(builtinPluginDir, p); err == nil {
			builtinPlugins[i] = filepath.ToSlash(relPath)
		} else {
			// Fallback to original path if relativization fails
			builtinPlugins[i] = p
		}
	}

	BazelLog.Infof("Loading %v orion env plugins from %q: %v", len(builtinPlugins), builtinPluginDir, builtinPlugins)

	for _, p := range builtinPlugins {
		if err := h.LoadPlugin(builtinPluginDir, p); err != nil {
			return err
		}
	}

	return nil
}

// LoadPlugin evaluates one orion extension and registers what it declares
// (plugins, rule kinds) on h.
//
// A load failure is returned rather than reported, because only the caller
// knows what to do about it, but it must not be ignored: an extension that did
// not load generates none of its targets, so continuing writes BUILD files
// silently missing whatever it was responsible for. Whoever reports it must do
// so on stderr, never stdout, which carries the generated patch under
// `gazelle -mode=diff`.
//
// The message is also recorded through BazelLog at error level — not info,
// which the default WarnLevel drops — so a run debugged from a log file does
// not simply stop mid-sequence. That mirrors common.MisconfiguredErrorf's
// both-streams behaviour; the helper itself is not reused because it needs a
// *config.Config and languages are constructed before gazelle builds one.
func (h *GazelleHost) LoadPlugin(pluginDir, pluginPath string) error {
	// Can not add new plugins after configuration/data-collection has started
	if h.gazelleKindInfo != nil || h.gazelleLoadInfo != nil {
		return fmt.Errorf("Cannot add orion plugin %q after configuration has started", pluginPath)
	}

	err := starzelle.LoadProxy(h, pluginDir, pluginPath)
	if err != nil {
		// Strip `pluginDir` so paths read as the user's workspace-relative ones,
		// and so sandbox paths do not leak into test output.
		errStr := strings.ReplaceAll(err.Error(), pluginDir+"/", "")

		// Name the plugin: with several extensions configured the error alone
		// does not always say which file to go fix.
		loadErr := fmt.Errorf("Failed to load orion plugin %q: %s", pluginPath, errStr)

		// Skipped when the log is already stderr, where the caller's report
		// lands, so the user is not told the same thing twice.
		if BazelLog.GetOutput() != os.Stderr {
			BazelLog.Errorf("%v", loadErr)
		}

		return loadErr
	}

	return nil
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
	if existing, exists := h.kinds[k.Name]; exists {
		from := k.RegisteredFrom
		if from == "" {
			from = "<builtin>"
		}
		existingFrom := existing.RegisteredFrom
		if existingFrom == "" {
			existingFrom = "<builtin>"
		}
		fmt.Fprintf(os.Stderr, "WARN: gazelle_rule_kind(%q) registered by %q overrides existing registration by %q\n", k.Name, from, existingFrom)
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
				fmt.Fprintf(os.Stderr, "Invalid rule 'From' label %q: %v\n", r.From, err)
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
					After:   []string{},
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

// PluginRegisteredKinds returns plugin-registered kinds keyed by name,
// excluding the seeded builtinKinds.
func (h *GazelleHost) PluginRegisteredKinds() map[string]plugin.RuleKind {
	out := make(map[string]plugin.RuleKind, len(h.kinds))
	for name, k := range h.kinds {
		out[name] = k
	}
	for _, k := range builtinKinds {
		delete(out, k.Name)
	}
	return out
}
