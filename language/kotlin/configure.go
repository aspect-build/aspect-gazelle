package gazelle

import (
	"flag"

	BazelLog "github.com/aspect-build/aspect-gazelle/common/logger"
	"github.com/aspect-build/aspect-gazelle/language/kotlin/kotlinconfig"
	jvm_javaconfig "github.com/bazel-contrib/rules_jvm/java/gazelle/javaconfig"
	jvm_maven "github.com/bazel-contrib/rules_jvm/java/gazelle/private/maven"
	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bazelbuild/bazel-gazelle/rule"
	"github.com/rs/zerolog"
)

var _ config.Configurer = (*kotlinLang)(nil)

var directivesByKey map[string]kotlinconfig.GenericDirective

func init() {
	directivesByKey = make(map[string]kotlinconfig.GenericDirective)
	for _, dir := range kotlinconfig.AllDirectives() {
		directivesByKey[dir.ConfigKey()] = dir
	}
}

func (kt *kotlinLang) KnownDirectives() []string {
	out := []string{
		jvm_javaconfig.JavaMavenInstallFile,
	}
	for _, dir := range kotlinconfig.AllDirectives() {
		out = append(out, dir.ConfigKey())
	}
	return out
}

func (kc *kotlinLang) initRootConfig(c *config.Config) kotlinconfig.Configs {
	if _, exists := c.Exts[LanguageName]; !exists {
		c.Exts[LanguageName] = kotlinconfig.Configs{
			"": kotlinconfig.New(c.RepoRoot),
		}
	}
	return c.Exts[LanguageName].(kotlinconfig.Configs)
}

func (kt *kotlinLang) Configure(c *config.Config, rel string, f *rule.File) {
	BazelLog.Tracef("Configure(%s): %s", LanguageName, rel)

	// Create the KotlinConfig for this package
	cfgs := kt.initRootConfig(c)
	cfg, exists := cfgs[rel]
	if !exists {
		parent := kotlinconfig.ParentForPackage(cfgs, rel)
		cfg = parent.NewChild(rel)
		cfgs[rel] = cfg
	}

	if f != nil {
		for _, d := range f.Directives {
			switch d.Key {

			case kotlinconfig.Directive_KotlinExtension:
				if err := kotlinconfig.EnabledDirective.Parse(d, cfg); err != nil {
					BazelLog.Fatalf("failed to parse directive %v: %v", d, err)
				}

			case jvm_javaconfig.JavaMavenInstallFile:
				cfg.SetMavenInstallFile(d.Value)

			default:
				if dir, ok := directivesByKey[d.Key]; ok {
					if err := dir.Parse(d, cfg); err != nil {
						BazelLog.Fatalf("error parsing kotlin directive: %v", err)
					}
				}
			}
		}
	}

	if kt.mavenResolver == nil {
		BazelLog.Tracef("Creating Maven resolver: %s", cfg.MavenInstallFile())

		// TODO: better zerolog configuration
		logger := zerolog.New(BazelLog.GetOutput()).Level(zerolog.TraceLevel)

		resolver, err := jvm_maven.NewResolver(
			jvm_maven.WithInstallFile(cfg.MavenInstallFile()),
			jvm_maven.WithLogger(logger),
		)
		if err != nil {
			BazelLog.Fatalf("error creating Maven resolver: %s", err.Error())
		}
		kt.mavenResolver = &resolver
	}
}

func (kc *kotlinLang) RegisterFlags(fs *flag.FlagSet, cmd string, c *config.Config) {
	// TODO: support rules_jvm flags such as 'java-maven-install-file'? (see rules_jvm java/gazelle/configure.go)
}

func (kc *kotlinLang) CheckFlags(fs *flag.FlagSet, c *config.Config) error {
	return nil
}
