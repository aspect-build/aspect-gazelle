package git

import (
	"flag"
	"log"

	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bazelbuild/bazel-gazelle/rule"
)

// Must align with patched gazelle key
const gitignoreProcessorExtKey = "__aspect:gitignoreProcessor"

// RegisterGitIgnoreProcessor wires gitignore support into Gazelle via config.Exts.
func RegisterGitIgnoreProcessor(c *config.Config) {
	c.Exts[gitignoreProcessorExtKey] = processGitignoreFile
}

type Configurer struct {
	enabled bool
}

// NewConfigurer returns a Configurer that registers gitignore processing.
// TODO: remove and align with gazelle after https://github.com/aspect-build/aspect-cli/issues/755
func NewConfigurer() config.Configurer {
	return &Configurer{enabled: true}
}

func (cc *Configurer) RegisterFlags(fs *flag.FlagSet, cmd string, c *config.Config) {
	fs.BoolVar(&cc.enabled, "gitignore", true, "Skip files matching .gitignore patterns when generating BUILD files.")
}

func (cc *Configurer) CheckFlags(fs *flag.FlagSet, c *config.Config) error {
	if cc.enabled {
		RegisterGitIgnoreProcessor(c)
	}
	return nil
}

func (*Configurer) KnownDirectives() []string { return []string{"gitignore"} }

func (*Configurer) Configure(c *config.Config, rel string, f *rule.File) {
	if f == nil {
		return
	}
	for _, d := range f.Directives {
		if d.Key == "gitignore" {
			log.Fatalf("the # gazelle:gitignore directive has been removed (found in //%s). Use the --gitignore[=true|false] command-line flag instead (default: true).", f.Pkg)
		}
	}
}
