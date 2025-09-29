/*
 * Copyright 2022 Aspect Build Systems, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package runner

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"strings"
	"sync"

	"github.com/EngFlow/gazelle_cc/language/cc"
	"github.com/aspect-build/aspect-gazelle/common/cache"
	js "github.com/aspect-build/aspect-gazelle/language/js"
	kotlin "github.com/aspect-build/aspect-gazelle/language/kotlin"
	orion "github.com/aspect-build/aspect-gazelle/language/orion"
	"github.com/aspect-build/aspect-gazelle/runner/language/bzl"
	"github.com/aspect-build/aspect-gazelle/runner/pkg/git"
	"github.com/aspect-build/aspect-gazelle/runner/pkg/ibp"
	"github.com/aspect-build/aspect-gazelle/runner/progress"
	vendoredGazelle "github.com/aspect-build/aspect-gazelle/runner/vendored/gazelle"
	java "github.com/bazel-contrib/rules_jvm/java/gazelle"
	python "github.com/bazel-contrib/rules_python/gazelle/python"
	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bazelbuild/bazel-gazelle/language"
	"github.com/bazelbuild/bazel-gazelle/language/bazel/visibility"
	golang "github.com/bazelbuild/bazel-gazelle/language/go"
	"github.com/bazelbuild/bazel-gazelle/language/proto"
	"github.com/bazelbuild/bazel-gazelle/rule"
	buf "github.com/bufbuild/rules_buf/gazelle/buf"
	"go.opentelemetry.io/otel"
	traceAttr "go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/term"
)

type GazelleRunner struct {
	workspaceDir string

	tracer trace.Tracer

	interactive  bool
	showProgress bool

	languageKeys []string
	languages    []func() language.Language
}

// Builtin Gazelle languages
type GazelleLanguage = string

const (
	JavaScript        GazelleLanguage = js.LanguageName
	Orion                             = orion.GazelleLanguageName
	Kotlin                            = kotlin.LanguageName
	Go                                = "go"
	DefaultVisibility                 = "visibility_extension"
	Protobuf                          = "proto"
	Buf                               = "buf"
	Bzl                               = "starlark"
	Python                            = "python"
	CC                                = "cc"
	Java                              = "java"
)

// Gazelle command
type GazelleCommand = string

const (
	UpdateCmd GazelleCommand = "update"
	FixCmd                   = "fix"
)

// Gazelle --mode
type GazelleMode = string

const (
	Fix   GazelleMode = "fix"
	Print             = "print"
	Diff              = "diff"
)

func New(workspaceDir string, showProgress bool) *GazelleRunner {
	c := &GazelleRunner{
		workspaceDir: workspaceDir,

		tracer: otel.GetTracerProvider().Tracer("aspect-gazelle"),

		interactive:  term.IsTerminal(int(os.Stdout.Fd())) && os.Getenv("CI") == "" && os.Getenv("BAZEL_TEST") == "",
		showProgress: showProgress,
	}

	return c
}

func pluralize(s string, num int) string {
	if num == 1 {
		return s
	} else {
		return s + "s"
	}
}

func (c *GazelleRunner) Languages() []string {
	return c.languageKeys
}

func (c *GazelleRunner) AddLanguageFactory(lang string, langFactory func() language.Language) {
	c.languageKeys = append(c.languageKeys, lang)
	c.languages = append(c.languages, langFactory)
}

// Keep in sync with _VALID_LANGUAGES in runner/def.bzl
func (c *GazelleRunner) AddLanguage(lang GazelleLanguage) {
	switch lang {
	case JavaScript:
		c.AddLanguageFactory(lang, js.NewLanguage)
	case Kotlin:
		c.AddLanguageFactory(lang, kotlin.NewLanguage)
	case Orion:
		c.AddLanguageFactory(lang, func() language.Language {
			return orion.NewLanguage()
		})
	case Buf:
		c.AddLanguageFactory(lang, buf.NewLanguage)
	case Go:
		c.AddLanguageFactory(lang, golang.NewLanguage)
	case DefaultVisibility:
		c.AddLanguageFactory(lang, visibility.NewLanguage)
	case Protobuf:
		c.AddLanguageFactory(lang, proto.NewLanguage)
	case Bzl:
		c.AddLanguageFactory(lang, bzl.NewLanguage)
	case Python:
		c.AddLanguageFactory(lang, python.NewLanguage)
	case CC:
		c.AddLanguageFactory(lang, cc.NewLanguage)
	case Java:
		c.AddLanguageFactory(lang, java.NewLanguage)
	default:
		log.Fatalf("ERROR: unknown language %q", lang)
	}
}

func (runner *GazelleRunner) prepareGazelleArgs(mode GazelleMode, args []string) []string {
	// Append the aspect-cli mode flag to the args parsed by gazelle.
	fixArgs := []string{"--mode=" + mode}

	// Append additional args including specific directories to fix.
	fixArgs = append(fixArgs, args...)

	return fixArgs
}

// Instantiate an instance of each language enabled in this GazelleRunner instance.
func (runner *GazelleRunner) instantiateLanguages() []language.Language {
	languages := make([]language.Language, 0, len(runner.languages)+1)

	if runner.interactive && runner.showProgress {
		languages = append(languages, progress.NewLanguage())
	}

	for _, lang := range runner.languages {
		languages = append(languages, lang())
	}

	failOnOrionKindOverlaps(languages)

	return languages
}

// failOnOrionKindOverlaps aborts when an orion plugin registered a rule
// kind that another enabled gazelle language also owns. Gazelle silently
// lets the last-registered Kinds() entry win, which clobbers the other
// language's Resolve/merge metadata and breaks dependency resolution for
// that kind — failing loudly is better than generating subtly-wrong
// BUILD files.
func failOnOrionKindOverlaps(languages []language.Language) {
	var orionHost *orion.GazelleHost
	for _, lang := range languages {
		if h, ok := lang.(*orion.GazelleHost); ok {
			orionHost = h
			break
		}
	}
	if orionHost == nil {
		return
	}

	pluginKinds := orionHost.PluginRegisteredKinds()
	if len(pluginKinds) == 0 {
		return
	}

	for _, lang := range languages {
		if _, isOrion := lang.(*orion.GazelleHost); isOrion {
			continue
		}
		langName := lang.Name()
		for kind := range lang.Kinds() {
			if registered, clash := pluginKinds[kind]; clash {
				log.Fatalf("ERROR: orion plugin %q registered rule kind %q (From=%q) which is already provided by the %q gazelle language. Remove the aspect.gazelle_rule_kind(%q) call from %q — the %q language's rule resolution for %q is otherwise silently broken.", registered.RegisteredFrom, kind, registered.From, langName, kind, registered.RegisteredFrom, langName, kind)
			}
		}
	}
}

func (runner *GazelleRunner) instantiateConfigs() []config.Configurer {
	configs := []config.Configurer{
		cache.NewConfigurer(),
		git.NewConfigurer(),
	}
	return configs
}

func (runner *GazelleRunner) Generate(cmd GazelleCommand, mode GazelleMode, args []string) (bool, error) {
	_, t := runner.tracer.Start(context.Background(), "GazelleRunner.Generate", trace.WithAttributes(
		traceAttr.String("mode", mode),
		traceAttr.StringSlice("languages", runner.languageKeys),
		traceAttr.StringSlice("args", args),
	))
	defer t.End()

	fixArgs := runner.prepareGazelleArgs(mode, args)

	if mode == Fix && runner.interactive {
		fmt.Printf("Updating BUILD files for %s\n", strings.Join(runner.languageKeys, ", "))
	}

	// Run gazelle
	langs := runner.instantiateLanguages()
	configs := runner.instantiateConfigs()
	visited, updated, err := vendoredGazelle.RunGazelleFixUpdate(runner.workspaceDir, cmd, configs, langs, fixArgs)

	if mode == Fix && runner.interactive && err == nil {
		fmt.Printf("%v BUILD %s visited\n", visited, pluralize("file", visited))
		fmt.Printf("%v BUILD %s updated\n", updated, pluralize("file", updated))
	}

	return updated > 0, err
}

func (p *GazelleRunner) Watch(watchAddress string, cmd GazelleCommand, mode GazelleMode, args []string) error {
	watch := ibp.NewClient(watchAddress)

	watchCaps := map[ibp.WatchCapability]any{
		// Only watch for source changes, not runfiles changes
		ibp.WatchCapability_WatchScope: []ibp.WatchScope{ibp.WatchScope_Sources},
	}

	if err := watch.Connect(watchCaps); err != nil {
		return fmt.Errorf("failed to connect to watchman: %w", err)
	}

	defer watch.Disconnect()

	// Cache hits skip file I/O; misses hash-verify via the disk cache.
	// CYCLE messages evict entries mid-session (see loop below).
	wc := cache.NewWatchCache()
	cache.SetCacheFactory(wc.NewCache)

	invalidator := &walkCacheInvalidator{}

	// Params for the underlying gazelle call
	fixArgs := p.prepareGazelleArgs(mode, args)

	// Initial run and status update to stdout.
	fmt.Printf("Initialize BUILD file generation --watch in %v\n", p.workspaceDir)
	languages := p.instantiateLanguages()
	configs := append(p.instantiateConfigs(), invalidator)
	visited, updated, err := vendoredGazelle.RunGazelleFixUpdate(p.workspaceDir, cmd, configs, languages, fixArgs)
	if err != nil {
		return fmt.Errorf("failed to run gazelle fix/update: %w", err)
	}
	if updated > 0 {
		fmt.Printf("Initial %v/%v BUILD files updated\n", updated, visited)
	} else {
		fmt.Printf("Initial %v BUILD files visited\n", visited)
	}

	ctx, t := p.tracer.Start(context.Background(), "GazelleRunner.Watch", trace.WithAttributes(
		traceAttr.String("mode", mode),
		traceAttr.StringSlice("languages", p.languageKeys),
		traceAttr.StringSlice("args", args),
	))
	defer t.End()

	// Subscribe to further changes
	for cs, err := range watch.AwaitCycle() {
		if err != nil {
			fmt.Printf("ERROR: watch cycle error: %v\n", err)
			return err
		}

		_, t := p.tracer.Start(ctx, "GazelleRunner.Watch.Trigger")

		// Evict cache entries for paths the protocol reports changed.
		changedPaths := make([]string, 0, len(cs.Sources))
		invalidator.dirs = invalidator.dirs[:0]
		for f := range cs.Sources {
			changedPaths = append(changedPaths, f)
			invalidator.dirs = append(invalidator.dirs, path.Dir(f))
		}
		wc.Invalidate(changedPaths)

		// The directories that have changed which gazelle should update.
		// This assumes all enabled gazelle languages support incremental updates.
		changedDirs := computeUpdatedDirs(p.workspaceDir, cs.Sources)

		fmt.Printf("Detected changes in %v\n", changedDirs)

		// Run gazelle
		languages := p.instantiateLanguages()
		configs := append(p.instantiateConfigs(), invalidator)
		visited, updated, err := vendoredGazelle.RunGazelleFixUpdate(p.workspaceDir, cmd, configs, languages, append(fixArgs, changedDirs...))
		if err != nil {
			return fmt.Errorf("failed to run gazelle fix/update: %w", err)
		}

		// Only output when changes were made, otherwise hopefully the execution was fast enough to be unnoticeable.
		if updated > 0 {
			fmt.Printf("%v/%v BUILD files updated\n", updated, visited)
		}

		t.End()
	}

	fmt.Printf("BUILD file generation --watch exiting...\n")

	return nil
}

/**
 * Convert a set of changed source files to a set of directories that gazelle
 * should update.
 *
 * A simple `path.Dir` is not sufficient because `generation_mode update_only`
 * may require a parent directory to be updated.
 *
 * TODO: this should be solved in gazelle? Including invocations on cli?
 */
func computeUpdatedDirs(rootDir string, changedFiles ibp.SourceInfoMap) []string {
	changedDirs := make([]string, 0, 1)
	processedDirs := make(map[string]bool, len(changedFiles))

	for f := range changedFiles {
		dir := path.Dir(f)
		for !processedDirs[dir] {
			processedDirs[dir] = true

			if hasBuildFile(rootDir, dir) {
				changedDirs = append(changedDirs, dir)
				break
			}

			dir = path.Dir(dir)
		}
	}

	return changedDirs
}

func hasBuildFile(rootDir, rel string) bool {
	for _, f := range config.DefaultValidBuildFileNames {
		if _, err := os.Stat(path.Join(rootDir, rel, f)); err == nil {
			return true
		}
	}

	return false
}

// Opt-in via ASPECT_GAZELLE_WALK_CACHE. Carries gazelle's walker cache across
// successive RunGazelleFixUpdate calls in the same process, evicting `dirs`
// (the cycle's changed dirs) before transferring.
type walkCacheInvalidator struct {
	dirs              []string
	previousWalkCache *sync.Map
}

func (i *walkCacheInvalidator) RegisterFlags(fs *flag.FlagSet, cmd string, c *config.Config) {}
func (i *walkCacheInvalidator) CheckFlags(fs *flag.FlagSet, c *config.Config) error {
	if os.Getenv("ASPECT_GAZELLE_WALK_CACHE") == "" {
		return nil
	}
	c.Exts["aspect:walkCache:load"] = func(arg any) {
		newCache := arg.(*sync.Map)
		if i.previousWalkCache != nil {
			i.previousWalkCache.Range(func(key, value any) bool {
				if !walkCacheEntryInvalidated(key.(string), i.dirs) {
					newCache.Store(key, value)
				}
				return true
			})
		}
		i.previousWalkCache = newCache
	}
	return nil
}
func (i *walkCacheInvalidator) KnownDirectives() []string                            { return nil }
func (i *walkCacheInvalidator) Configure(c *config.Config, rel string, f *rule.File) {}

func walkCacheEntryInvalidated(rel string, dirs []string) bool {
	for _, d := range dirs {
		// A change at the root invalidates every entry.
		// Support "." returned by path.Dir() in addition to standard ""
		if d == "." || d == "" {
			return true
		}
		if rel == d {
			return true
		}
		if len(rel) > len(d) && rel[len(d)] == '/' && strings.HasPrefix(rel, d) {
			return true
		}
	}
	return false
}
