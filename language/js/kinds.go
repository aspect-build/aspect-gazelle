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

// scopedMapKindKey is scopedKind keyed by the group's configured target name,
// so groups are addressed the same way as in js_files/js_test_files.
func scopedMapKindKey(cfg *JsGazelleConfig, kind, groupName string) string {
	return scopedKind(kind, cfg.MapTargetName(groupName))
}

// scopedMapKind returns the target kind of the `map_kind ts_project:<group>`
// directive applying to the group, or "" when none applies (or the directive
// is misconfigured, which is reported). A valid mapping requires an
// accompanying alias_kind; that machinery does the merge/resolve/index.
func (ts *typeScriptLang) scopedMapKind(c *config.Config, groupName string) string {
	cfg := c.Exts[LanguageName].(*JsGazelleConfig)

	if jsKey := scopedMapKindKey(cfg, JsLibraryKind, groupName); c.KindMap[jsKey].KindName != "" {
		ts.reportMapKindMisconfig(c, jsKey,
			"invalid '# gazelle:map_kind %s ...': use the group key '%s'; the ts_project group key also covers packages whose group is generated as js_library",
			jsKey, scopedMapKindKey(cfg, TsProjectKind, groupName),
		)
		return ""
	}

	key := scopedMapKindKey(cfg, TsProjectKind, groupName)
	target := c.KindMap[key].KindName
	if target == "" {
		return ""
	}

	if _, isOwnKind := tsKinds[target]; isOwnKind {
		ts.reportMapKindMisconfig(c, key,
			"invalid '# gazelle:map_kind %s %s ...': group-scoped map_kind cannot map to the built-in kind %q",
			key, target, target,
		)
		return ""
	}

	// The alias must declare the wrapped kind gazelle merges/resolves/indexes
	// against: ts_project, or the kind a plain map_kind maps it to here.
	wrapped := TsProjectKind
	if plain, isMapped := c.KindMap[TsProjectKind]; isMapped {
		wrapped = plain.KindName
	}
	if c.AliasMap[target] != wrapped {
		ts.reportMapKindMisconfig(c, key,
			"invalid '# gazelle:map_kind %s %s ...': requires '# gazelle:alias_kind %s %s' declaring the kind the %s macro wraps",
			key, target, target, wrapped, target,
		)
		return ""
	}

	return target
}

// reportMapKindMisconfig reports a scoped map_kind misconfiguration as a user
// error (aborting generation), at most once per directive key.
func (ts *typeScriptLang) reportMapKindMisconfig(c *config.Config, key, msg string, args ...any) {
	if _, done := ts.mapKindWarned[key]; done {
		return
	}
	ts.mapKindWarned[key] = struct{}{}
	common.MisconfiguredErrorf(c, msg, args...)
}

// recordMapKindScopes records which scoped map_kind keys exist and which are
// claimed by a real target group, so DoneGeneratingRules can warn about keys
// naming no group visited this run.
func (ts *typeScriptLang) recordMapKindScopes(c *config.Config, cfg *JsGazelleConfig) {
	for key := range c.KindMap {
		kind, group, isScoped := strings.Cut(key, ":")
		if isScoped && group != "" && (kind == TsProjectKind || kind == JsLibraryKind) {
			ts.mapKindScopeSeen[key] = struct{}{}
		}
	}

	for _, target := range cfg.GetSourceTargets() {
		for _, kind := range []string{TsProjectKind, JsLibraryKind} {
			key := scopedMapKindKey(cfg, kind, target.name)
			if _, ok := c.KindMap[key]; ok {
				ts.mapKindScopeUsed[key] = struct{}{}
			}
		}
	}
}

// DoneGeneratingRules warns (on stderr, non-fatally) about scoped map_kind
// directives whose group matched no target group visited this run — a likely
// typo, though a partial run may not have visited the defining directory, as
// the message notes.
func (ts *typeScriptLang) DoneGeneratingRules() {
	unused := make([]string, 0, len(ts.mapKindScopeSeen))
	for key := range ts.mapKindScopeSeen {
		if _, used := ts.mapKindScopeUsed[key]; !used {
			unused = append(unused, key)
		}
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

// groupSourceRuleKinds returns the kinds this plugin may generate for a group:
// the source rule kinds plus its scoped map_kind target (scopedTarget, "" if
// none). Used by kind-mapping-aware helpers like ruleUtils.RemoveRule.
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
