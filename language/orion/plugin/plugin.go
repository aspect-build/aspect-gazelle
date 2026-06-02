package plugin

import (
	"encoding/gob"
	"fmt"
	"slices"
	"strings"

	common "github.com/aspect-build/aspect-gazelle/common"
)

type PluginId = string

type PluginHost interface {
	AddKind(k RuleKind)
	AddPlugin(plugin Plugin)
}

// TODO: change the interface into a factory method (at least in starzelle)
type Plugin interface {
	// Static plugin metadata
	Name() PluginId
	Properties() map[string]Property

	// Prepare for generating targets
	Prepare(ctx PrepareContext) PrepareResult
	Analyze(ctx AnalyzeContext) error
	DeclareTargets(ctx DeclareTargetsContext) DeclareTargetsResult
}

type PropertyType = string

const (
	PropertyType_String  PropertyType = "string"
	PropertyType_Strings PropertyType = "[]string"
	PropertyType_Bool    PropertyType = "bool"
	PropertyType_Number  PropertyType = "number"
)

type RuleKind struct {
	KindInfo
	Name string
	From string

	// RegisteredFrom is the orion plugin file that called gazelle_rule_kind.
	RegisteredFrom string
}

// Subset of the bazel-gazelle rule.KindInfo. See bazel-gazelle for details.
type KindInfo struct {
	// MatchAny is true if a rule of this kind may be matched with any rule
	// of the same kind, regardless of attributes, if exactly one rule is
	// present a build file.
	MatchAny bool

	// MatchAttrs is a list of attributes used in matching. For example,
	// for go_library, this list contains "importpath". Attributes are matched
	// in order.
	MatchAttrs []string

	// NonEmptyAttrs is a set of attributes that, if present, disqualify a rule
	// from being deleted after merge.
	NonEmptyAttrs []string

	// MergeableAttrs is a set of attributes that should be merged before
	// dependency resolution. For example "srcs" are often merged before resolution
	// to compute the full set of sources for a target before resolving dependencies.
	MergeableAttrs []string

	// ResolveAttrs is a set of attributes that should be merged after
	// dependency resolution. For example "deps" are often merged after resolution.
	ResolveAttrs []string
}

// Properties an extension can be configured
type Property struct {
	Name         string // TODO: drop because it's always specified in a map[Name]?
	PropertyType PropertyType
	Default      any
}

type PropertyValues struct {
	// defs are the plugin's property declarations, used to resolve defaults and
	// validate names at read time. Shared and read-only.
	defs map[string]Property

	// values holds only the properties set by a directive (in this directory or
	// inherited); unset properties fall back to their default at read time.
	values map[string]any

	// localKeys holds the names of properties set by a directive in this exact
	// directory's BUILD file (as opposed to inherited from an ancestor). This lets
	// a plugin detect where a marker directive is declared, eg to anchor a scope.
	localKeys map[string]bool
}

func NewPropertyValues(defs map[string]Property) PropertyValues {
	return PropertyValues{defs: defs}
}

// Add records property `name`'s directive-set value. `local` marks it as set by a
// directive in this directory's own BUILD file (as opposed to inherited).
func (pv *PropertyValues) Add(name string, value any, local bool) {
	if pv.values == nil {
		pv.values = make(map[string]any)
	}
	pv.values[name] = value

	if local {
		if pv.localKeys == nil {
			pv.localKeys = make(map[string]bool)
		}
		pv.localKeys[name] = true
	}
}

// IsLocal reports whether `name` was set by a directive in this directory's own
// BUILD file (not inherited from an ancestor).
func (pv *PropertyValues) IsLocal(name string) bool {
	return pv.localKeys[name]
}

// PluginData is a plugin-private, inherited key/value store exposed as ctx.data.
// Values may only be written during the prepare stage (writes are top-down, so
// descendants can read ancestors' values); writes in other stages are an error
// because generation is bottom-up and would not reach descendants. Reads return
// this directory's value or, failing that, the nearest ancestor's (via inherit).
type PluginData struct {
	local    map[string]any
	inherit  func(key string) (any, bool)
	writable bool
}

func NewPluginData(inherit func(key string) (any, bool)) *PluginData {
	return &PluginData{inherit: inherit, writable: true}
}

// Local returns this directory's own written values, for storage/inheritance.
// May be nil when nothing was written; reads of a nil map miss harmlessly.
func (d *PluginData) Local() map[string]any {
	return d.local
}

// Seal makes the store read-only, rejecting further writes. Called after the
// prepare stage so analyze/declare can read but not write.
func (d *PluginData) Seal() {
	d.writable = false
}

func (d *PluginData) lookup(key string) (any, bool) {
	if v, ok := d.local[key]; ok {
		return v, true
	}
	if d.inherit != nil {
		return d.inherit(key)
	}
	return nil, false
}

// The context for an extension to prepare for generating targets.
type PrepareContext struct {
	RepoName   string
	Rel        string
	Properties PropertyValues

	// Data is a plugin-private, inherited key/value store (ctx.data). Writable
	// only during prepare; readable in all stages.
	Data *PluginData

	// HasFile reports whether this directory contains the named file, where name
	// may be a plain filename or a `sub/dir/file` relative path resolved against
	// this directory (ctx.has_file). Set by the host; nil if unavailable.
	HasFile func(name string) bool
}

// The result of an extension preparing for generating targets.
//
// Queries are mapped by file extension and will be executed against all
// matching extensions.
//
// Example:
//
//	 PrepareResult {
//			Extensions: [".java"],
//			Queries: {
//				"imports": {
//					"Type": "string|strings|exists",
//					"Extensions": ["*.java"],
//					"Query": "(import_list)",
//				},
//			},
//	 }
type PrepareResult struct {
	Sources map[string][]SourceFilter
	Queries NamedQueries
}

type SourceFilter interface {
	Match(p string) bool
}

func NewSourceGlobFilter(include, exclude []string) (SourceFilter, error) {
	if len(include) == 0 {
		return nil, fmt.Errorf("at least one include glob pattern is required")
	}

	// Validate the exclude patterns on their own so the error identifies which
	// list the bad pattern came from. Parsed expressions are cached, so the
	// combined parse below does not redo the work.
	if len(exclude) > 0 {
		if _, err := common.ParseGlobExpressions(exclude); err != nil {
			return nil, fmt.Errorf("exclude: %w", err)
		}
	}

	expr, err := common.ParseGlobExpressionsWithExcludes(include, exclude)
	if err != nil {
		return nil, err
	}

	return &SourceGlobFilter{
		Globs:   include,
		Exclude: exclude,
		expr:    expr,
	}, nil
}
func NewSourceExtensionsFilter(exts []string) SourceFilter {
	return &SourceExtensionsFilter{Extensions: exts}
}
func NewSourceFileFilter(files []string) SourceFilter {
	return &SourceFileFilter{Files: files}
}

var _ SourceFilter = (*SourceGlobFilter)(nil)

type SourceGlobFilter struct {
	Globs   []string
	Exclude []string
	expr    common.GlobExpr
}

func (f SourceGlobFilter) Match(p string) bool {
	return f.expr(p)
}

var _ SourceFilter = (*SourceExtensionsFilter)(nil)

type SourceExtensionsFilter struct {
	Extensions []string
}

func (f SourceExtensionsFilter) Match(p string) bool {
	for _, ext := range f.Extensions {
		if strings.HasSuffix(p, ext) {
			return true
		}
	}
	return false
}

var _ SourceFilter = (*SourceFileFilter)(nil)

type SourceFileFilter struct {
	Files []string
}

func (sf SourceFileFilter) Match(p string) bool {
	return slices.Contains(sf.Files, p)
}

type Label struct {
	Repo, Pkg, Name string
}

type AnalyzeContext struct {
	PrepareContext
	Source   *TargetSource
	database *Database
}

func (a AnalyzeContext) AddSymbol(label Label, symbol Symbol) {
	a.database.AddSymbol(label, symbol)
}

func NewAnalyzeContext(prep PrepareContext, source *TargetSource, database *Database) AnalyzeContext {
	return AnalyzeContext{
		PrepareContext: prep,
		Source:         source,
		database:       database,
	}
}

type TargetSources map[string]TargetSourceList
type TargetSourceList []TargetSource

// The context for an extension to generate targets.
//
// Queries results are mapped by file extension, each containing a map of
// query name to result.
type DeclareTargetsContext struct {
	PrepareContext
	Sources  TargetSources
	Targets  DeclareTargetActions
	database *Database
}

func (d DeclareTargetsContext) AddSymbol(label Label, symbol Symbol) {
	d.database.AddSymbol(label, symbol)
}

func NewDeclareTargetsContext(prep PrepareContext, sources TargetSources, targets DeclareTargetActions, database *Database) DeclareTargetsContext {
	return DeclareTargetsContext{
		PrepareContext: prep,
		Sources:        sources,
		Targets:        targets,
		database:       database,
	}
}

type DeclareTargetActions interface {
	Add(target TargetDeclaration)
	Remove(name, kind string)
	Actions() []TargetAction
}

var _ DeclareTargetActions = (*declareTargetActionsImpl)(nil)

type declareTargetActionsImpl struct {
	actions []TargetAction
}

func NewDeclareTargetActions() DeclareTargetActions {
	return &declareTargetActionsImpl{
		actions: []TargetAction{},
	}
}
func (ctx *declareTargetActionsImpl) Actions() []TargetAction {
	return ctx.actions
}
func (ctx *declareTargetActionsImpl) Add(t TargetDeclaration) {
	ctx.actions = append(ctx.actions, AddTargetAction{
		TargetDeclaration: t,
	})
}
func (ctx *declareTargetActionsImpl) Remove(name, kind string) {
	ctx.actions = append(ctx.actions, RemoveTargetAction{
		Name: name,
		Kind: kind,
	})
}

// The result of declaring targets
type DeclareTargetsResult struct {
	Actions []TargetAction
}

type TargetSource struct {
	Path         string
	QueryResults QueryResults
}

func init() {
	// TODO: don't expose 'gob' cache serialization here
	gob.Register(QueryResults{})
	gob.Register(QueryMatches{})
	gob.Register(QueryMatch{})
	gob.Register(QueryCapture{})
	gob.Register(QueryProcessorResult{})
}
