/*
Symbol-db resolution: choosing among the same-id symbols plugins register with
ctx.add_symbol(), and resolving deferred symbols, ones registered with an
aspect.Import() label instead of a concrete one.

A deferred symbol is provided by whatever its Import resolves to. The Import is
resolved in the scope of the package that declared it: with that package's config
(so its `# gazelle:resolve` overrides apply) and from that package (for nearness
ranking). The result therefore does not depend on the importer, so each deferred
symbol is resolved once and cached for every importer. Only self-imports are
filtered per importer.
*/
package gazelle

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"sync"

	plugin "github.com/aspect-build/aspect-gazelle/language/orion/plugin"
	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bazelbuild/bazel-gazelle/label"
	"github.com/bazelbuild/bazel-gazelle/resolve"
)

// resolveState is the state of resolving one top-level import.
type resolveState struct {
	// The deferred symbols being resolved, outermost first, to detect add_symbol() Import cycles.
	chain []plugin.TargetSymbol

	// Unknown-dependency errors of deferred Imports, reported alongside other imports' errors
	// instead of aborting the attribute's resolution.
	unresolved []error
}

// symbolResult is the resolution of one symbol-db entry for one importer.
type symbolResult struct {
	labels     []label.Label
	unresolved []error

	// The entry resolved only to the importer itself.
	selfOnly bool
}

// deferredResult is the importer-independent resolution of a deferred symbol.
type deferredResult struct {
	labels     []label.Label
	unresolved []error
	err        error
}

// deferredSymbols caches deferredResults by the entry's Import, which is unique per add_symbol() call.
type deferredSymbols struct {
	mu      sync.Mutex
	results map[*plugin.TargetImport]*deferredResult
}

func (d *deferredSymbols) get(imp *plugin.TargetImport) (*deferredResult, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	r, ok := d.results[imp]
	return r, ok
}

func (d *deferredSymbols) put(imp *plugin.TargetImport, r *deferredResult) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.results == nil {
		d.results = make(map[*plugin.TargetImport]*deferredResult)
	}
	d.results[imp] = r
}

// symbolCycleError reports a chain of deferred symbols whose Imports lead back to the first.
type symbolCycleError struct {
	chain []plugin.TargetSymbol
}

func (e *symbolCycleError) Error() string {
	steps := make([]string, len(e.chain))
	for i, s := range e.chain {
		steps[i] = fmt.Sprintf("%q (provider %q) in //%s", s.Id, s.Provider, s.Label.Pkg)
	}
	return "add_symbol() Imports form a cycle: " + strings.Join(steps, " -> ")
}

// resolveSingleSymbol resolves a non-multiple import to one of the same-id symbols `matched`.
// Candidates are tried nearest first; a deferred candidate that resolves to nothing falls
// through to the next, and the unresolved errors are kept only if no candidate resolves.
func (host *GazelleHost) resolveSingleSymbol(
	c *config.Config,
	ix *resolve.RuleIndex,
	pluginId plugin.PluginId,
	impt plugin.TargetImport,
	matched []plugin.TargetSymbol,
	from label.Label,
	st *resolveState,
) (ResolutionType, []label.Label, error) {
	var unresolved []error
	for _, s := range rankSymbols(matched, from, c.RepoName) {
		res, err := host.symbolLabels(c, ix, pluginId, s, from, st.chain)
		if err != nil {
			return Resolution_Error, nil, err
		}
		if res.selfOnly {
			return Resolution_None, nil, nil
		}
		labels := uniqueLabels(res.labels, c.RepoName)
		switch len(labels) {
		case 0:
			unresolved = append(unresolved, res.unresolved...)
		case 1:
			return Resolution_Label, labels, nil
		default:
			return Resolution_Error, nil, ambiguousImportError(impt, pluginId, labels)
		}
	}
	st.unresolved = append(st.unresolved, unresolved...)
	return Resolution_None, nil, nil
}

// symbolLabels returns the labels providing symbol-db entry `s` to importer `from`: its own
// label, or for a deferred symbol the (cached) resolution of its Import minus `from` itself.
func (host *GazelleHost) symbolLabels(
	c *config.Config,
	ix *resolve.RuleIndex,
	pluginId plugin.PluginId,
	s plugin.TargetSymbol,
	from label.Label,
	chain []plugin.TargetSymbol,
) (symbolResult, error) {
	if s.Import == nil {
		return symbolResult{labels: []label.Label{symbolLabel(s)}}, nil
	}

	d := host.resolveDeferred(c, ix, pluginId, s, chain)
	if d.err != nil {
		return symbolResult{}, d.err
	}

	self := normalizeLabel(from, c.RepoName)
	labels := make([]label.Label, 0, len(d.labels))
	for _, l := range d.labels {
		if !normalizeLabel(l, c.RepoName).Equal(self) {
			labels = append(labels, l)
		}
	}
	return symbolResult{
		labels:     labels,
		unresolved: d.unresolved,
		selfOnly:   len(d.labels) > 0 && len(labels) == 0,
	}, nil
}

// resolveDeferred resolves deferred symbol `s` in the scope of its declaring package, caching
// the result. `c` is used only if the declaring package's config was not recorded.
func (host *GazelleHost) resolveDeferred(
	c *config.Config,
	ix *resolve.RuleIndex,
	pluginId plugin.PluginId,
	s plugin.TargetSymbol,
	chain []plugin.TargetSymbol,
) *deferredResult {
	if r, ok := host.deferred.get(s.Import); ok {
		return r
	}
	for i, prev := range chain {
		if prev.Import == s.Import {
			return &deferredResult{err: &symbolCycleError{chain: append(slices.Clone(chain[i:]), s)}}
		}
	}

	if cfg, ok := host.packageConfigs.Load(s.Label.Pkg); ok {
		c = cfg.(*config.Config)
	}
	declarer := label.New(s.Label.Repo, s.Label.Pkg, "")
	st := &resolveState{chain: append(slices.Clone(chain), s)}

	resolutionType, labels, err := host.resolveImport(c, ix, pluginId, *s.Import, declarer, st)
	r := &deferredResult{labels: labels, err: err}
	if err == nil && !s.Import.Optional {
		r.unresolved = st.unresolved
		if resolutionType == Resolution_NotFound {
			r.unresolved = append(r.unresolved, unknownDependencyError(*s.Import))
		}
	}
	host.deferred.put(s.Import, r)
	return r
}

// rankSymbols orders same-id symbols nearest first: those whose package is an ancestor of the
// importer `from` (same repo), deepest first, then the rest in registration order.
func rankSymbols(symbols []plugin.TargetSymbol, from label.Label, mainRepo string) []plugin.TargetSymbol {
	fromRepo := normalizeRepo(from.Repo, mainRepo)
	depth := func(s plugin.TargetSymbol) int {
		if normalizeRepo(s.Label.Repo, mainRepo) != fromRepo || !isPkgAncestor(s.Label.Pkg, from.Pkg) {
			return -1
		}
		return len(s.Label.Pkg)
	}
	ranked := slices.Clone(symbols)
	slices.SortStableFunc(ranked, func(a, b plugin.TargetSymbol) int {
		return cmp.Compare(depth(b), depth(a))
	})
	return ranked
}

// isPkgAncestor reports whether package `anc` encloses (or equals) package `pkg`.
func isPkgAncestor(anc, pkg string) bool {
	return anc == "" || anc == pkg || strings.HasPrefix(pkg, anc+"/")
}

// symbolLabel converts a symbol-db entry's label into an absolute label.
func symbolLabel(s plugin.TargetSymbol) label.Label {
	return label.Label{
		Repo:     s.Label.Repo,
		Pkg:      s.Label.Pkg,
		Name:     s.Label.Name,
		Relative: false,
	}
}

// normalizeRepo maps the empty repo, which denotes the main repository, to `mainRepo`.
func normalizeRepo(repo, mainRepo string) string {
	if repo == "" {
		return mainRepo
	}
	return repo
}

func normalizeLabel(l label.Label, mainRepo string) label.Label {
	l.Repo = normalizeRepo(l.Repo, mainRepo)
	return l
}

// uniqueLabels removes duplicate labels, treating the empty repo as `mainRepo`.
func uniqueLabels(labels []label.Label, mainRepo string) []label.Label {
	unique := make([]label.Label, 0, len(labels))
	for _, l := range labels {
		n := normalizeLabel(l, mainRepo)
		if !slices.ContainsFunc(unique, func(u label.Label) bool { return normalizeLabel(u, mainRepo).Equal(n) }) {
			unique = append(unique, l)
		}
	}
	return unique
}
