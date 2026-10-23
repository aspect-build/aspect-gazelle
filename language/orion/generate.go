package gazelle

import (
	"crypto"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"

	common "github.com/aspect-build/aspect-gazelle/common"
	"github.com/aspect-build/aspect-gazelle/common/cache"
	BazelLog "github.com/aspect-build/aspect-gazelle/common/logger"
	ruleUtils "github.com/aspect-build/aspect-gazelle/common/rule"
	"github.com/aspect-build/aspect-gazelle/language/orion/plugin"
	queryRunner "github.com/aspect-build/aspect-gazelle/language/orion/queries"
	"github.com/bazelbuild/bazel-gazelle/config"
	gazelleLabel "github.com/bazelbuild/bazel-gazelle/label"
	gazelleLanguage "github.com/bazelbuild/bazel-gazelle/language"
	gazelleRule "github.com/bazelbuild/bazel-gazelle/rule"
	bzl "github.com/bazelbuild/buildtools/build"
	"golang.org/x/sync/errgroup"
)

const (
	// TODO: move to common
	MaxWorkerCount = 12
)

const (
	targetAttrValues     = "__target_attr_values"
	targetDeclarationKey = "__target_declaration"
	targetPluginKey      = "__target_plugin"
)

// Gazelle GenerateRules phase - declare:
//   - which rules to delete (GenerateResult.Empty)
//   - which rules to create (or merge with existing) and their associated metadata (GenerateResult.Gen + GenerateResult.Imports)
func (host *GazelleHost) GenerateRules(args gazelleLanguage.GenerateArgs) gazelleLanguage.GenerateResult {
	BazelLog.Tracef("GenerateRules(%s): %s", GazelleLanguageName, args.Rel)

	cfg := getBUILDConfig(args.Config, args.Rel)

	// Mark this BUILDConfig as generated since it is having real rules generated.
	cfg.generated = true

	return host.generateRules(cfg, args)
}

func (host *GazelleHost) generateRules(cfg *BUILDConfig, args gazelleLanguage.GenerateArgs) gazelleLanguage.GenerateResult {
	queryCache := cache.Get(args.Config)

	// Stage 1:
	// Collect source files indexed for multiple purposes such as:
	//  - iterating over all source files per plugin
	//  - iterating over plugins per source file
	//  - iterating over source files by plugin file group
	pluginSourceFiles, sourceFilePlugins, pluginSourceGroupFiles := host.collectSourceFilesByPlugin(cfg, args.Config, args.RegularFiles)

	// Run queries on source files. Cap concurrency at MaxWorkerCount: more
	// in-flight blocking opens just churn the scheduler spinning up OS threads.
	eg := errgroup.Group{}
	eg.SetLimit(MaxWorkerCount)

	// Stage 2:
	// Parse and query source files and collect results

	sourceFileQueryResults := make(map[string]plugin.QueryResults, len(sourceFilePlugins))
	sourceFileQueryResultsLock := sync.Mutex{}

	// Parse and query source files
	for sourceFile, pluginIds := range sourceFilePlugins {
		// Collect all queries for this source file from all plugins
		// as well as the queriesHash for all plugins with queries for this file.
		queries := make(plugin.NamedQueries)
		pluginHashes := make([]string, 0, len(pluginIds))
		for _, pluginId := range pluginIds {
			prep := cfg.pluginPrepareResults[pluginId]
			hasMatch := false
			for queryId, query := range prep.getQueriesForFile(sourceFile) {
				queries[pluginId+"|"+queryId] = query
				hasMatch = true
			}
			if hasMatch {
				pluginHashes = append(pluginHashes, prep.queriesHash)
			}
		}

		if len(queries) == 0 {
			continue
		}

		// Joined queriesHash for all plugins with queries on this file
		slices.Sort(pluginHashes) // Sorted to ensure deterministic cache keys regardless of plugin order
		queriesHash := strings.Join(pluginHashes, "|")

		// Capture loop variables for goroutine
		sourceFile := sourceFile
		eg.Go(func() error {
			p := joinPkg(args.Rel, sourceFile)
			queryResults, err := host.runSourceQueries(queryCache, queries, queriesHash, args.Config.RepoRoot, p)
			if err != nil {
				return fmt.Errorf("Querying source file %q: %v", p, err)
			}

			sourceFileQueryResultsLock.Lock()
			defer sourceFileQueryResultsLock.Unlock()
			sourceFileQueryResults[sourceFile] = queryResults
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		common.GenerationErrorf(args.Config, "Plugin source query error: %v", err)
		return gazelleLanguage.GenerateResult{}
	}

	// Build the TargetSource for each file for each plugin.
	pluginTargetSources := make(map[plugin.PluginId]map[string]plugin.TargetSource, len(cfg.pluginPrepareResults))
	for pluginId := range cfg.pluginPrepareResults {
		pluginSrcs := pluginSourceFiles[pluginId]

		targetSources := make(map[string]plugin.TargetSource, len(pluginSrcs))
		for _, f := range pluginSrcs {
			targetSources[f] = plugin.TargetSource{
				Path:         f,
				QueryResults: make(plugin.QueryResults),
			}
		}

		pluginTargetSources[pluginId] = targetSources
	}

	// Assign the file query results to the correct plugin TargetSources.QueryResults.
	for f, results := range sourceFileQueryResults {
		for key, queryResult := range results {
			pluginId, queryId, _ := strings.Cut(key, "|")
			pluginTargetSources[pluginId][f].QueryResults[queryId] = queryResult
		}
	}

	// Stage 3:
	// Analyze each plugin source file.
	for pluginId, prep := range cfg.pluginPrepareResults {
		for _, src := range pluginTargetSources[pluginId] {
			// Capture loop variables for goroutine
			pluginId := pluginId
			prep := prep
			src := src
			eg.Go(func() error {
				actx := plugin.NewAnalyzeContext(prep.PrepareContext, &src, host.database)

				if err := host.plugins[pluginId].Analyze(actx); err != nil {
					return fmt.Errorf("analyze failed for %s: %w", pluginId, err)
				}
				return nil
			})
		}
	}

	if err := eg.Wait(); err != nil {
		common.GenerationErrorf(args.Config, "Plugin source analysis error: %v", err)
		return gazelleLanguage.GenerateResult{}
	}

	// Stage 4:
	// Generate target actions for each plugin
	pluginTargetActions := make(map[plugin.PluginId][]plugin.TargetAction, len(cfg.pluginPrepareResults))
	pluginTargetsLock := sync.Mutex{}
	for pluginId, prep := range cfg.pluginPrepareResults {
		// Capture loop variables for goroutine
		pluginId := pluginId
		prep := prep
		eg.Go(func() error {
			// Group the TargetSource's into the source groups for the plugin.
			pluginTargetGroups := plugin.TargetSources{}

			for groupId := range prep.Sources {
				files := pluginSourceGroupFiles[pluginId][groupId]

				// Add the TargetSource for each file in the group, even if empty.
				pluginTargetGroups[groupId] = make([]plugin.TargetSource, 0, len(files))
				for _, f := range files {
					pluginTargetGroups[groupId] = append(pluginTargetGroups[groupId], pluginTargetSources[pluginId][f])
				}
			}

			// If no default group exists create one with all sources.
			if _, hasDefaultGroup := pluginTargetGroups[plugin.DeclareTargetsContextDefaultGroup]; !hasDefaultGroup {
				pluginTargetGroups[plugin.DeclareTargetsContextDefaultGroup] = slices.Collect(maps.Values(pluginTargetSources[pluginId]))
			}

			// Use the collected sources and analysis to generate rules
			actions := host.generateTargets(pluginId, prep, pluginTargetGroups)

			// Lock for the assignment into the cross-thread pluginTargets
			pluginTargetsLock.Lock()
			defer pluginTargetsLock.Unlock()
			pluginTargetActions[pluginId] = actions

			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		common.GenerationErrorf(args.Config, "Plugin target generation error: %v", err)
		return gazelleLanguage.GenerateResult{}
	}

	// Stage 5:
	// Apply plugin actions
	return host.convertPlugActionsToGenerateResult(pluginTargetActions, args)
}

// findRule returns the rule named name in the existing BUILD file, if any.
func findRule(f *gazelleRule.File, name string) *gazelleRule.Rule {
	if f == nil {
		return nil
	}
	for _, r := range f.Rules {
		if r.Name() == name {
			return r
		}
	}
	return nil
}

func applyRemoveAction(args gazelleLanguage.GenerateArgs, result *gazelleLanguage.GenerateResult, rm plugin.RemoveTargetAction) *gazelleRule.Rule {
	r := findRule(args.File, rm.Name)
	if r == nil {
		return nil
	}

	kind := rm.Kind
	if rm.Kind == "" {
		kind = r.Kind() // TODO: need to reverse map_kind?
	}
	empty := gazelleRule.NewRule(kind, r.Name())
	// Gazelle's default merge cannot clear a glob(), which would keep the rule non-empty.
	for _, attr := range r.AttrKeys() {
		if existing, ok := parseGlobAttr(r.Attr(attr)); ok && existing.glob != nil {
			empty.SetAttr(attr, globMerger{})
		}
	}
	result.Empty = append(result.Empty, empty)
	return r
}

func (host *GazelleHost) convertPlugActionsToGenerateResult(pluginActions map[string][]plugin.TargetAction, args gazelleLanguage.GenerateArgs) gazelleLanguage.GenerateResult {
	var result gazelleLanguage.GenerateResult

	// Iterate over the pluginIds[] in a deterministic order
	// instead of iterating over the plugins[] or pluginActions[pluginId] map
	for _, pluginId := range host.pluginIds {
		for _, action := range pluginActions[pluginId] {
			host.applyPluginAction(args, pluginId, action, &result)
		}
	}

	return result
}

func (host *GazelleHost) applyPluginAction(args gazelleLanguage.GenerateArgs, pluginId plugin.PluginId, action plugin.TargetAction, result *gazelleLanguage.GenerateResult) {
	switch a := action.(type) {
	case plugin.RemoveTargetAction:
		// If marked for removal simply add to the empty list and continue
		if removed := applyRemoveAction(args, result, a); removed != nil {
			BazelLog.Debugf("GenerateRules remove target: %s %s(%q)", args.Rel, removed.Kind(), removed.Name())
		}
	case plugin.AddTargetAction:
		// Check for name-collisions with the rule being generated.
		target := a.TargetDeclaration
		colError := ruleUtils.CheckCollisionErrors(target.Name, target.Kind, host.sourceRuleKinds, args)
		if colError != nil {
			common.GenerationErrorf(args.Config, "Source rule generation error: %v", colError)
			return
		}

		// Generate the gazelle Rule to be added/merged into the BUILD file.
		rule, attrs := convertPluginTargetDeclaration(args.Rel, pluginId, target, findRule(args.File, target.Name))

		result.Gen = append(result.Gen, rule)
		result.Imports = append(result.Imports, attrs)
		result.RelsToIndex = append(result.RelsToIndex, targetAttributesToRelsToImport(args.Rel, attrs)...)

		BazelLog.Tracef("GenerateRules(%s) add target: %s %s(%q)", GazelleLanguageName, args.Rel, target.Kind, target.Name)
	default:
		BazelLog.Fatalf("Unknown plugin action type: %T", action)
	}
}

type attributeValue struct {
	singleton bool
	values    []interface{}
	imports   []plugin.TargetImport
	// existing is the attribute value in the BUILD file before merging, if any.
	existing bzl.Expr
}

// globAttr is an existing attribute value made of a glob() and/or a list:
// `[...]`, `glob(...)`, `glob(...) + [...]` or `[...] + glob(...)`.
type globAttr struct {
	glob      *bzl.CallExpr
	list      *bzl.ListExpr
	listFirst bool
}

func parseGlobAttr(e bzl.Expr) (globAttr, bool) {
	if list, isList := e.(*bzl.ListExpr); isList {
		return globAttr{list: list}, true
	}
	if glob, isGlob := asGlobCall(e); isGlob {
		return globAttr{glob: glob}, true
	}
	if sum, isSum := e.(*bzl.BinaryExpr); isSum && sum.Op == "+" {
		if glob, isGlob := asGlobCall(sum.X); isGlob {
			if list, isList := sum.Y.(*bzl.ListExpr); isList {
				return globAttr{glob: glob, list: list}, true
			}
		}
		if glob, isGlob := asGlobCall(sum.Y); isGlob {
			if list, isList := sum.X.(*bzl.ListExpr); isList {
				return globAttr{glob: glob, list: list, listFirst: true}, true
			}
		}
	}
	return globAttr{}, false
}

func asGlobCall(e bzl.Expr) (*bzl.CallExpr, bool) {
	_, isGlob := gazelleRule.ParseGlobExpr(e)
	call, _ := e.(*bzl.CallExpr)
	return call, isGlob
}

// withGlobMerge wraps a generated attribute value in a globMerger when a glob()
// is generated or exists, which gazelle's default merge cannot parse. Other
// values, and existing values globAttr cannot parse such as a select(), are
// left to gazelle, which keeps what it cannot merge and logs it.
func withGlobMerge(value interface{}, existing bzl.Expr) interface{} {
	attr, ok := parseGlobAttr(existing)
	if !ok {
		return value
	}
	switch v := value.(type) {
	case plugin.Glob:
		return globMerger{glob: &v}
	case []interface{}:
		if attr.glob != nil {
			return globMerger{list: v}
		}
	}
	return value
}

// globMerger is a generated glob() and/or list merged into a globAttr. The
// glob() is replaced unless unchanged, and the list merges like any gazelle
// list so "# keep" entries survive. The zero value deletes an existing glob().
type globMerger struct {
	glob *plugin.Glob
	list []interface{}
}

var _ gazelleRule.BzlExprValue = (*globMerger)(nil)
var _ gazelleRule.Merger = (*globMerger)(nil)

func (m globMerger) BzlExpr() bzl.Expr {
	if e := m.Merge(nil); e != nil {
		return e
	}
	return &bzl.ListExpr{}
}

func (m globMerger) Merge(other bzl.Expr) bzl.Expr {
	existing, _ := parseGlobAttr(other)

	var glob bzl.Expr
	if m.glob != nil {
		glob = m.glob.BzlExpr()
		if existing.glob != nil && sameGlob(existing.glob, *m.glob) {
			glob = existing.glob
		}
	}

	var genList bzl.Expr
	if len(m.list) > 0 {
		genList = gazelleRule.ExprFromValue(m.list)
	}
	list := gazelleRule.MergeList(genList, existing.list)

	switch {
	case list == nil:
		return glob
	case glob == nil:
		return list
	case existing.listFirst:
		return &bzl.BinaryExpr{X: list, Op: "+", Y: glob}
	default:
		return &bzl.BinaryExpr{X: glob, Op: "+", Y: list}
	}
}

// sameGlob compares patterns unordered so a hand-ordered glob() is unchanged.
func sameGlob(call *bzl.CallExpr, g plugin.Glob) bool {
	existing, _ := gazelleRule.ParseGlobExpr(call)
	return existing.AllowEmpty == g.AllowEmpty &&
		sameSet(existing.Patterns, g.Include) &&
		sameSet(existing.Excludes, g.Exclude)
}

func sameSet(a, b []string) bool {
	return slices.Equal(slices.Sorted(slices.Values(a)), slices.Sorted(slices.Values(b)))
}

func convertPluginTargetDeclaration(pkg string, pluginId plugin.PluginId, target plugin.TargetDeclaration, existingRule *gazelleRule.Rule) (*gazelleRule.Rule, map[string]*attributeValue) {
	targetRule := gazelleRule.NewRule(target.Kind, target.Name)

	ruleAttrs := make(map[string]*attributeValue, len(target.Attrs))

	targetRule.SetPrivateAttr(targetPluginKey, pluginId)
	targetRule.SetPrivateAttr(targetDeclarationKey, target)
	targetRule.SetPrivateAttr(targetAttrValues, ruleAttrs)

	for attr, val := range target.Attrs {
		attrValue, attrImports, isArray := convertPluginAttribute(pkg, val)

		var existing bzl.Expr
		if existingRule != nil {
			existing = existingRule.Attr(attr)
		}

		// TODO: verify 'attr' is resolveable if len(attrImports) > 0
		ruleAttrs[attr] = &attributeValue{
			singleton: !isArray,
			imports:   attrImports,
			values:    attrValue,
			existing:  existing,
		}

		// Update the attribute if any non-import was specified
		if len(attrValue) > 0 {
			if isArray {
				// An array of values taken as-is
				targetRule.SetAttr(attr, withGlobMerge(attrValue, existing))
			} else if attrValue[0] == nil {
				// A single nil value is the same as deleting
				targetRule.DelAttr(attr)
			} else {
				// Otherwise use the single value
				targetRule.SetAttr(attr, withGlobMerge(attrValue[0], existing))
			}
		}
	}

	return targetRule, ruleAttrs
}

func targetAttributesToRelsToImport(pkg string, attrs map[string]*attributeValue) []string {
	var relToImport []string

	// By default it is assumed all imports are workspace-relative paths.
	// TODO: provide hooks for plugins to override this behavior.

	for _, attrVal := range attrs {
		for _, imp := range attrVal.imports {
			rel := imp.Id
			rel = strings.Trim(rel, "/")

			relToImport = append(relToImport, rel)
		}
	}

	return relToImport
}

func convertPluginAttribute(pkg string, val interface{}) ([]interface{}, []plugin.TargetImport, bool) {
	if a, isArray := val.([]interface{}); isArray {
		// Each element typically yields a single value (imports are rare), so
		// size r to the array up front and leave i nil until something needs it.
		r := make([]interface{}, 0, len(a))
		var i []plugin.TargetImport
		for _, v := range a {
			newR, newI, _ := convertPluginAttribute(pkg, v)
			r = append(r, newR...)
			i = append(i, newI...)
		}
		return r, i, true
	}

	if targetImport, isImport := val.(plugin.TargetImport); isImport {
		return nil, []plugin.TargetImport{targetImport}, false
	}

	// Convert plugin.Label to a gazelle Label
	if l, isLabel := val.(plugin.Label); isLabel {
		val = gazelleLabel.New(l.Repo, l.Pkg, l.Name)
	}

	// Normalize gazelle labels to be relative to the BUILD file
	if l, isLabel := val.(gazelleLabel.Label); isLabel {
		// TODO: also convert the `args.Config.RepoName` repo to relative?
		return []interface{}{l.Rel("", pkg)}, nil, false
	}

	return []interface{}{val}, nil, false
}

func computeQueriesCacheKey(queries plugin.NamedQueries) string {
	cacheDigest := crypto.MD5.New()

	keys := make([]string, 0, len(queries))
	for key := range queries {
		keys = append(keys, key)
	}
	slices.Sort(keys)

	e := gob.NewEncoder(cacheDigest)
	for _, key := range keys {
		if err := e.Encode(key); err != nil {
			BazelLog.Fatalf("Failed to encode query key %q: %v", key, err)
		}
		q := queries[key]
		if err := e.Encode(q.QueryType()); err != nil {
			BazelLog.Fatalf("Failed to encode query type value %q: %v", q, err)
		}
		// Note: gob flattens the pointer and encodes q as its concrete *Query
		// struct (no gob.Register needed: the stream is never decoded and the
		// concrete type is seen at the top level). Func-typed fields such as
		// QueryBase.FilterExpr are ignored by gob like unexported fields; the
		// Filter string patterns are sufficient for cache key purposes.
		if err := e.Encode(q); err != nil {
			BazelLog.Fatalf("Failed to encode query value %q: %v", q, err)
		}
	}

	return hex.EncodeToString(cacheDigest.Sum(nil))
}

func (host *GazelleHost) runSourceQueries(queryCache cache.Cache, queries plugin.NamedQueries, queriesHash, baseDir, f string) (plugin.QueryResults, error) {
	var qr plugin.QueryResults

	r, _, err := queryCache.LoadOrStoreFile(baseDir, f, queriesHash, func(p string, sourceCode []byte) (any, error) {
		return queryRunner.RunQueries(f, sourceCode, queries)
	})

	if r != nil {
		qr = r.(plugin.QueryResults)
	}

	return qr, err
}

// Collect source files managed by this BUILD and batch them by plugins interested in them.
func (host *GazelleHost) collectSourceFilesByPlugin(cfg *BUILDConfig, c *config.Config, files []string) (map[plugin.PluginId][]string, map[string][]plugin.PluginId, map[plugin.PluginId]map[string][]string) {
	pluginSourceFiles := make(map[plugin.PluginId][]string, len(cfg.pluginPrepareResults))
	sourceFilePlugins := make(map[string][]plugin.PluginId)
	pluginSourceGroupFiles := make(map[plugin.PluginId]map[string][]string, len(cfg.pluginPrepareResults))

	// Collect source files managed by this BUILD for each plugin.
	for _, f := range files {
		// Skip BUILD files
		if c.IsValidBuildFileName(f) {
			continue
		}

		for pluginId, p := range cfg.pluginPrepareResults {
			foundGroup := false

			// Collect the groups this file belongs to for this plugin.
			for groupId, groupSrcFilters := range p.Sources {
				for _, srcFilter := range groupSrcFilters {
					if srcFilter.Match(f) {
						foundGroup = true

						if pluginSourceGroupFiles[pluginId] == nil {
							pluginSourceGroupFiles[pluginId] = make(map[string][]string)
						}

						pluginSourceGroupFiles[pluginId][groupId] = append(pluginSourceGroupFiles[pluginId][groupId], f)
						break
					}
				}
			}

			// If the file matched any groups, add it to the file+plugin maps.
			if foundGroup {
				pluginSourceFiles[pluginId] = append(pluginSourceFiles[pluginId], f)
				sourceFilePlugins[f] = append(sourceFilePlugins[f], pluginId)
			}
		}
	}

	return pluginSourceFiles, sourceFilePlugins, pluginSourceGroupFiles
}

// Let plugins declare any targets they want to generate for the target sources.
func (host *GazelleHost) generateTargets(pluginId plugin.PluginId, prep pluginConfig, sources plugin.TargetSources) []plugin.TargetAction {
	ctx := plugin.NewDeclareTargetsContext(
		prep.PrepareContext,
		sources,
		plugin.NewDeclareTargetActions(),
		host.database,
	)

	return host.plugins[pluginId].DeclareTargets(ctx).Actions
}

// path.Join() for cases where the 2 parts are already normalized and simply need concatenation.
func joinPkg(pkg, rel string) string {
	if pkg == "" {
		return rel
	}
	// rel is already workspace-relative and should not need path cleaning.
	return pkg + "/" + rel
}
