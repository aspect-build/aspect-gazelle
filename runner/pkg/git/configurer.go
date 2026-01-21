package git

import (
	"flag"

	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bazelbuild/bazel-gazelle/rule"
)

// Must align with patched gazelle key
const gitignoreProcessorExtKey = "__aspect:gitignoreProcessor"

// RegisterGitIgnoreProcessor wires gitignore support into Gazelle via config.Exts.
func RegisterGitIgnoreProcessor(c *config.Config) {
	c.Exts[gitignoreProcessorExtKey] = processGitignoreFile
}

type configurer struct{}

// NewConfigurer returns a Configurer that registers gitignore processing.
// TODO: remove and align with gazelle after https://github.com/aspect-build/aspect-cli/issues/755
func NewConfigurer() config.Configurer {
	return &configurer{}
}

func (cc *configurer) RegisterFlags(fs *flag.FlagSet, cmd string, c *config.Config) {}

func (cc *configurer) CheckFlags(fs *flag.FlagSet, c *config.Config) error {
	RegisterGitIgnoreProcessor(c)
	return nil
}

func (*configurer) KnownDirectives() []string                            { return nil }
func (*configurer) Configure(c *config.Config, rel string, f *rule.File) {}
