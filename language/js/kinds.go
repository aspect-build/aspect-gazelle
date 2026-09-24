package gazelle

import (
	"fmt"
	"os"
	"sort"
	"strings"

	common "github.com/aspect-build/aspect-gazelle/common"
	ruleUtils "github.com/aspect-build/aspect-gazelle/common/rule"
	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bazelbuild/bazel-gazelle/language"
	"github.com/bazelbuild/bazel-gazelle/rule"
	"github.com/emirpasic/gods/v2/sets/treeset"
)

const (
	TsProjectKind         = "ts_project"
	TsProtoLibraryKind    = "ts_proto_library"
	JsLibraryKind         = "js_library"
	JsBinaryKind          = "js_binary"
	JsRunBinaryKind       = "js_run_binary"
	TsConfigKind          = "ts_config"
	NpmPackageKind        = "npm_package"
	NpmLinkAllKind        = "npm_link_all_packages"
	RulesJsModuleName     = "aspect_rules_js"
	RulesJsRepositoryName = RulesJsModuleName
	RulesTsModuleName     = "aspect_rules_ts"
	RulesTsRepositoryName = RulesTsModuleName
	NpmRepositoryName     = "npm"
)

var sourceRuleKinds = treeset.NewWith(strings.Compare, TsProjectKind, JsLibraryKind, TsProtoLibraryKind)

// Kinds returns a map that maps rule names (kinds) and information on how to
// match and merge attributes that may be found in rules of those kinds.
func (*typeScriptLang) Kinds() map[string]rule.KindInfo {
	return tsKinds
}

// scopedKind returns the KindMap key of a `map_kind kind:group ...` directive.
func scopedKind(kind, groupName string) string {
	return kind + ":" + groupName
}

// declarationAppliesTo reports whether a directive declared in declRel applies to rel.
func declarationAppliesTo(declRel, rel string) bool {
	return declRel == "" || declRel == rel || strings.HasPrefix(rel, declRel+"/")
}

// mappingSite is one scoped map_kind declaration; claimed marks whether its
// group exists in any directory the declaration applies to.
type mappingSite struct {
	declRel string
	key     string
	claimed bool
}

// scopedMapKindState is workspace-wide bookkeeping for group-scoped map_kind
// directives, written unsynchronized relying on Gazelle's serial traversal.
type scopedMapKindState struct {
	// Sites per kind:group, the group resolved under the declaring
	// directory's naming convention so mappings survive descendant renames.
	// Unclaimed sites are warned about in DoneGeneratingRules.
	mappingSites map[string][]mappingSite
	// Already-reported errors/warnings, by directive key.
	warned map[string]struct{}
}

func newScopedMapKindState() *scopedMapKindState {
	return &scopedMapKindState{
		mappingSites: make(map[string][]mappingSite),
		warned:       make(map[string]struct{}),
	}
}

// lookup returns the scoped mapping (KindMap key and target kind) applying
// to the group, or empty strings; the nearest declaration wins.
func (s *scopedMapKindState) lookup(c *config.Config, cfg *JsGazelleConfig, kind, groupName string) (string, string) {
	best := -1
	bestKey := ""
	for _, site := range s.mappingSites[scopedKind(kind, groupName)] {
		if declarationAppliesTo(site.declRel, cfg.rel) && len(site.declRel) > best {
			best, bestKey = len(site.declRel), site.key
		}
	}
	if bestKey == "" {
		return "", ""
	}
	return bestKey, c.KindMap[bestKey].KindName
}

// scopedMapKind returns the KindMap key and target kind of the
// `map_kind ts_project:<group>` directive applying to the group, or empty
// strings when none applies or the directive is misconfigured (reported).
func (ts *typeScriptLang) scopedMapKind(c *config.Config, groupName string) (string, string) {
	cfg := c.Exts[LanguageName].(*JsGazelleConfig)

	if jsKey, jsTarget := ts.mapKinds.lookup(c, cfg, JsLibraryKind, groupName); jsTarget != "" {
		_, groupPart, _ := strings.Cut(jsKey, ":")
		ts.mapKinds.report(c, jsKey,
			"invalid '# gazelle:map_kind %s ...': use the group key '%s'; the ts_project group key also covers packages whose group is generated as js_library",
			jsKey, scopedKind(TsProjectKind, groupPart),
		)
		return "", ""
	}

	key, target := ts.mapKinds.lookup(c, cfg, TsProjectKind, groupName)
	if target == "" {
		return "", ""
	}

	if _, isOwnKind := tsKinds[target]; isOwnKind {
		ts.mapKinds.report(c, key,
			"invalid '# gazelle:map_kind %s %s ...': group-scoped map_kind cannot map to the built-in kind %q",
			key, target, target,
		)
		return "", ""
	}

	// The alias is defaulted when unregistered, so a mismatch is a
	// conflicting declaration or a subtree whose plain map_kind changed the
	// wrapped kind; both require an explicit declaration.
	if wrapped, got := wrappedKind(c), c.AliasMap[target]; got != wrapped {
		ts.mapKinds.report(c, key,
			"invalid '# gazelle:map_kind %s %s ...': the %s macro must wrap %s here; declare '# gazelle:alias_kind %s %s'",
			key, target, target, wrapped, target, wrapped,
		)
		return "", ""
	}

	return key, target
}

// wrappedKind is the kind a scoped macro wraps here: ts_project, or the kind
// a plain map_kind maps it to.
func wrappedKind(c *config.Config) string {
	if plain, isMapped := c.KindMap[TsProjectKind]; isMapped {
		return plain.KindName
	}
	return TsProjectKind
}

// configure resolves the directory's scoped map_kind declarations and
// defaults the alias_kind of scoped macros the user declared none for.
func (s *scopedMapKindState) configure(c *config.Config, cfg *JsGazelleConfig) {
	s.recordMappings(cfg)
	s.configureAliases(c)
	s.markClaimedMappings(cfg)
}

func (s *scopedMapKindState) recordMappings(cfg *JsGazelleConfig) {
	for _, key := range cfg.scopedMapKindKeys {
		kind, group, _ := strings.Cut(key, ":")
		id := scopedKind(kind, cfg.ReverseMapTargetName(group))
		s.mappingSites[id] = append(s.mappingSites[id], mappingSite{declRel: cfg.rel, key: key})
	}
}

func (s *scopedMapKindState) configureAliases(c *config.Config) {
	wrapped := wrappedKind(c)
	for key, mapped := range c.KindMap {
		kind, group, isScoped := strings.Cut(key, ":")
		if !isScoped || group == "" || kind != TsProjectKind {
			continue
		}

		target := mapped.KindName
		if _, isOwnKind := tsKinds[target]; isOwnKind {
			continue // reported in scopedMapKind
		}
		if _, registered := c.AliasMap[target]; registered {
			continue // matching, or a conflict reported in scopedMapKind
		}

		if c.AliasMap == nil {
			c.AliasMap = make(map[string]string)
		}
		c.AliasMap[target] = wrapped
	}
}

func (s *scopedMapKindState) markClaimedMappings(cfg *JsGazelleConfig) {
	for _, target := range cfg.GetSourceTargets() {
		for _, kind := range []string{TsProjectKind, JsLibraryKind} {
			sites := s.mappingSites[scopedKind(kind, target.name)]
			for i := range sites {
				if declarationAppliesTo(sites[i].declRel, cfg.rel) {
					sites[i].claimed = true
				}
			}
		}
	}
}

// report raises a misconfiguration user error, at most once per key.
func (s *scopedMapKindState) report(c *config.Config, key, msg string, args ...any) {
	if _, done := s.warned[key]; done {
		return
	}
	s.warned[key] = struct{}{}
	common.MisconfiguredErrorf(c, msg, args...)
}

// DoneGeneratingRules warns (on stderr, non-fatally) about scoped map_kind
// directives whose group matched no target group visited this run.
func (ts *typeScriptLang) DoneGeneratingRules() {
	unusedSet := make(map[string]struct{})
	for _, sites := range ts.mapKinds.mappingSites {
		for _, site := range sites {
			if !site.claimed {
				unusedSet[site.key] = struct{}{}
			}
		}
	}
	unused := make([]string, 0, len(unusedSet))
	for key := range unusedSet {
		unused = append(unused, key)
	}
	sort.Strings(unused)

	for _, key := range unused {
		_, group, _ := strings.Cut(key, ":")
		fmt.Fprintf(os.Stderr,
			"gazelle: ignoring '# gazelle:map_kind %s ...': no target group named %q "+
				"in any directory visited in this run "+
				"(target groups are defined by js_files/js_test_files directives; "+
				"when running Gazelle on a subset of directories the group may be defined elsewhere)\n",
			key, group,
		)
	}
}

// groupSourceRuleKinds returns the kinds this plugin may generate for a
// group: the source rule kinds plus its scoped map_kind target, if any.
func (ts *typeScriptLang) groupSourceRuleKinds(scopedTarget string) *treeset.Set[string] {
	kinds := treeset.NewWith(strings.Compare, sourceRuleKinds.Values()...)
	if scopedTarget != "" {
		kinds.Add(scopedTarget)
	}
	return kinds
}

// isManagedKind reports whether kind is one of the given generated kinds or a
// map_kind replacement of one.
func isManagedKind(args language.GenerateArgs, generatedKinds *treeset.Set[string], kind string) bool {
	for it := generatedKinds.Iterator(); it.Next(); {
		if ruleUtils.MapKind(args, it.Value()) == kind {
			return true
		}
	}
	return false
}

var tsKinds = map[string]rule.KindInfo{
	TsProjectKind: {
		MatchAny: false,
		NonEmptyAttrs: map[string]bool{
			"srcs": true,
		},
		SubstituteAttrs: map[string]bool{
			"tsconfig": true,
		},
		MergeableAttrs: map[string]bool{
			"srcs":   true,
			"assets": true,

			// Generated based on project config.
			"isolated_typecheck": true,

			// Attributes reflecting tsconfig when tsconfig generation is enabled.
			"tsconfig":              true,
			"allow_js":              true,
			"composite":             true,
			"declaration":           true,
			"declaration_dir":       true,
			"declaration_map":       true,
			"emit_declaration_only": true,
			"source_map":            true,
			"incremental":           true,
			"ts_build_info_file":    true,
			"resolve_json_module":   true,
			"preserve_jsx":          true,
			"out_dir":               true,
			"root_dir":              true,
		},
		ResolveAttrs: map[string]bool{
			"deps": true,
		},
	},
	JsLibraryKind: {
		MatchAny: false,
		NonEmptyAttrs: map[string]bool{
			"srcs": true,
		},
		SubstituteAttrs: map[string]bool{},
		MergeableAttrs: map[string]bool{
			"srcs": true,
		},
		ResolveAttrs: map[string]bool{
			"deps": true,
		},
	},
	JsBinaryKind: {
		MatchAny: false,
		NonEmptyAttrs: map[string]bool{
			"entry_point": true,
		},
		SubstituteAttrs: map[string]bool{},
		MergeableAttrs:  map[string]bool{},
		ResolveAttrs: map[string]bool{
			"data": true,
		},
	},
	JsRunBinaryKind: {
		MatchAny: false,
		NonEmptyAttrs: map[string]bool{
			"tool": true,
		},
		SubstituteAttrs: map[string]bool{},
		MergeableAttrs:  map[string]bool{},
		ResolveAttrs: map[string]bool{
			"srcs": true,
			"tool": true,
		},
	},
	TsConfigKind: {
		MatchAttrs: []string{"src"},
		NonEmptyAttrs: map[string]bool{
			"src": true,
		},
		SubstituteAttrs: map[string]bool{},
		MergeableAttrs:  map[string]bool{},
		ResolveAttrs: map[string]bool{
			"deps": true,
		},
	},
	TsProtoLibraryKind: {
		MatchAny: false,
		NonEmptyAttrs: map[string]bool{
			"proto": true,
		},
		ResolveAttrs: map[string]bool{
			"deps":  true,
			"proto": true,
		},
	},
	NpmLinkAllKind: {
		MatchAny: true,
	},
	NpmPackageKind: {
		MatchAny: false,
		NonEmptyAttrs: map[string]bool{
			"srcs": true,
		},
		SubstituteAttrs: map[string]bool{},
		MergeableAttrs: map[string]bool{
			"srcs": true,
		},
		ResolveAttrs: map[string]bool{
			"srcs": true,
		},
	},
}

// Loads returns .bzl files and symbols they define. Every rule generated by
// GenerateRules, now or in the past, should be loadable from one of these
// files.
func (ts *typeScriptLang) Loads() []rule.LoadInfo {
	panic("ApparentLoads should be called instead")
}

func (h *typeScriptLang) ApparentLoads(moduleToApparentName func(string) string) []rule.LoadInfo {
	tsModName := moduleToApparentName(RulesTsModuleName)
	if tsModName == "" {
		tsModName = RulesTsRepositoryName
	}

	jsModName := moduleToApparentName(RulesJsModuleName)
	if jsModName == "" {
		jsModName = RulesJsRepositoryName
	}

	return []rule.LoadInfo{
		{
			Name: "@" + tsModName + "//ts:defs.bzl",
			Symbols: []string{
				TsProjectKind,
				TsConfigKind,
			},
		},

		{
			Name: "@" + tsModName + "//ts:proto.bzl",
			Symbols: []string{
				TsProtoLibraryKind,
			},
		},

		{
			Name: "@" + jsModName + "//npm:defs.bzl",
			Symbols: []string{
				NpmPackageKind,
			},
		},

		{
			Name: "@" + jsModName + "//js:defs.bzl",
			Symbols: []string{
				JsLibraryKind, JsBinaryKind, JsRunBinaryKind,
			},
		},

		{
			Name: "@" + NpmRepositoryName + "//:defs.bzl",
			Symbols: []string{
				NpmLinkAllKind,
			},
		},
	}
}
