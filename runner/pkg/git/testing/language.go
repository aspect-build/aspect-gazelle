package git_testing

import (
	"github.com/aspect-build/aspect-gazelle/runner/pkg/git"
	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bazelbuild/bazel-gazelle/label"
	"github.com/bazelbuild/bazel-gazelle/language"
	"github.com/bazelbuild/bazel-gazelle/repo"
	"github.com/bazelbuild/bazel-gazelle/resolve"
	"github.com/bazelbuild/bazel-gazelle/rule"
)

// A noop language designed only to register gitignore processing.

var _ language.Language = (*gitLang)(nil)

// gitLang embeds *git.Configurer so it inherits the --gitignore flag
// registration, directive rejection, and processor wiring — the same
// behavior as the production configurer in the runner.
type gitLang struct {
	*git.Configurer
}

// NewLanguage returns a new git language instance.
func NewLanguage() language.Language {
	return &gitLang{Configurer: git.NewConfigurer().(*git.Configurer)}
}
func (p *gitLang) Name() string { return "gitignore_TESTING" }
func (p *gitLang) GenerateRules(args language.GenerateArgs) language.GenerateResult {
	return language.GenerateResult{}
}
func (p *gitLang) DoneGeneratingRules() {}
func (p *gitLang) Resolve(c *config.Config, ix *resolve.RuleIndex, rc *repo.RemoteCache, r *rule.Rule, imports any, from label.Label) {
}
func (p *gitLang) Loads() []rule.LoadInfo                              { return nil }
func (p *gitLang) Kinds() map[string]rule.KindInfo                     { return nil }
func (p *gitLang) Fix(c *config.Config, f *rule.File)                  {}
func (p *gitLang) Embeds(r *rule.Rule, from label.Label) []label.Label { return nil }
func (p *gitLang) Imports(c *config.Config, r *rule.Rule, f *rule.File) []resolve.ImportSpec {
	return nil
}
