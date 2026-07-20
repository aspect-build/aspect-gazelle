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

// scopedKind returns the Config.KindMap key of a `# gazelle:map_kind
// ts_project:my_group my_kind //:defs.bzl` directive, restricting the mapping
// to a target group.
func scopedKind(kind, groupName string) string {
	return kind + ":" + groupName
}

// scopedMapKindKey returns the Config.KindMap key a group-scoped map_kind
// directive uses to address the given target group and source kind. The
// group's configured (naming-convention) name is used, so the group is
// addressed the same way as in js_files/js_test_files directives.
func scopedMapKindKey(cfg *JsGazelleConfig, kind, groupName string) string {
	return scopedKind(kind, cfg.MapTargetName(groupName))
}

// scopedMapKind returns the target kind of the group-scoped map_kind
// directive applying to the given target group, or "" when none applies in
// this directory. `ts_project:<group>` is the group key: the mapping applies
// to the entire group, including packages where the group has no transpiled
// sources and its rule is a js_library.
//
// A scoped mapping requires an accompanying `# gazelle:alias_kind <target>
// <wrapped_kind>` directive declaring which kind the target macro wraps —
// ts_project, or the kind a plain map_kind directive maps ts_project to in
// this directory. The standard alias_kind machinery is what merges,
// resolves, and indexes existing rules of the target kind; rules of a scoped
// group are simply generated with the mapped kind (see addProjectRule).
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

	if _, isAliased := c.AliasMap[target]; !isAliased {
		wrapped := TsProjectKind
		if plain, isMapped := c.KindMap[TsProjectKind]; isMapped {
			wrapped = plain.KindName
		}
		ts.reportMapKindMisconfig(c, key,
			"invalid '# gazelle:map_kind %s %s ...': requires '# gazelle:alias_kind %s %s' declaring the kind the %s macro wraps",
			key, target, target, wrapped, target,
		)
		return ""
	}

	return target
}

// reportMapKindMisconfig reports a group-scoped map_kind misconfiguration that
// would produce broken output as a user error (aborting generation), at most
// once per directive key.
func (ts *typeScriptLang) reportMapKindMisconfig(c *config.Config, key, msg string, args ...any) {
	if _, done := ts.mapKindWarned[key]; done {
		return
	}
	ts.mapKindWarned[key] = struct{}{}
	common.MisconfiguredErrorf(c, msg, args...)
}

// recordMapKindScopes tracks, for this directory, which group-scoped map_kind
// keys exist and which are addressed by a real target group, so that keys
// naming no group visited in this run (e.g. a mistyped group name) can be
// reported once all directories visited this run have been configured (see
// DoneGeneratingRules).
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

// DoneGeneratingRules warns about group-scoped map_kind directives whose group
// key matched no target group in any directory visited in this Gazelle run
// (e.g. a mistyped group name). This does not abort generation: the directive
// names no group, so no rule is affected. Only directories visited this run are
// known, so a partial run (e.g. `gazelle <subdir>`) can warn about a group that
// is in fact defined outside the visited set; the message says so. The warning
// goes to stderr so it is visible regardless of log configuration.
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

// unscopeExistingKind reverts an existing rule from a group-scoped map_kind
// target kind back to the kind unscoped source rules have in this directory,
// returning the scoped pseudo-kind of the mapping reverted from ("" if the
// rule was not scoped). The empty-rule machinery requires registered KindInfo
// to delete a rule, so a rule must be unscoped before it can be removed.
func (ts *typeScriptLang) unscopeExistingKind(args language.GenerateArgs, groupName string, existing *rule.Rule) string {
	if existing == nil || existing.ShouldKeep() {
		return ""
	}
	if target := ts.scopedMapKind(args.Config, groupName); target != "" && target == existing.Kind() {
		cfg := args.Config.Exts[LanguageName].(*JsGazelleConfig)
		existing.SetKind(ruleUtils.MapKind(args, TsProjectKind))
		return scopedMapKindKey(cfg, TsProjectKind, groupName)
	}
	return ""
}

// groupSourceRuleKinds returns the kinds this plugin may generate for the
// given target group: the source rule kinds plus any group-scoped map_kind
// target kinds that apply. Passing the result to kind-mapping-aware helpers
// such as ruleUtils.RemoveRule makes them recognize the group's mapped kinds.
func (ts *typeScriptLang) groupSourceRuleKinds(args language.GenerateArgs, groupName string) *treeset.Set[string] {
	kinds := treeset.NewWith(strings.Compare, sourceRuleKinds.Values()...)
	if target := ts.scopedMapKind(args.Config, groupName); target != "" {
		kinds.Add(target)
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
