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
	"fmt"
	"log"
	"os"
	"path"
	"strings"

	"github.com/EngFlow/gazelle_cc/language/cc"
	"github.com/aspect-build/aspect-gazelle/common/bazel"
	js "github.com/aspect-build/aspect-gazelle/language/js"
	orion "github.com/aspect-build/aspect-gazelle/language/orion"
	"github.com/aspect-build/aspect-gazelle/runner/language/bzl"
	"github.com/aspect-build/aspect-gazelle/runner/pkg/git"
	"github.com/aspect-build/aspect-gazelle/runner/pkg/ibp"
	"github.com/aspect-build/aspect-gazelle/runner/progress"
	vendoredGazelle "github.com/aspect-build/aspect-gazelle/runner/vendored/gazelle"
	python "github.com/bazel-contrib/rules_python/gazelle/python"
	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bazelbuild/bazel-gazelle/language"
	golang "github.com/bazelbuild/bazel-gazelle/language/go"
	"github.com/bazelbuild/bazel-gazelle/language/proto"
	"go.opentelemetry.io/otel"
	traceAttr "go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/term"
)

type GazelleRunner struct {
	tracer trace.Tracer

	languageKeys []string
	languages    []func() language.Language
}

// Builtin Gazelle languages
type GazelleLanguage = string

const (
	JavaScript GazelleLanguage = js.LanguageName
	Orion                      = orion.GazelleLanguageName
	Go                         = "go"
	Protobuf                   = "proto"
	Bzl                        = "starlark"
	Python                     = "python"
	CC                         = "cc"
)

// Gazelle --mode
type GazelleMode = string

const (
	Fix   GazelleMode = "fix"
	Print             = "update"
	Diff              = "diff"
)

// Setup gitignore within Gazelle.
func init() {
	git.SetupGitIgnore()
}

func New() *GazelleRunner {
	c := &GazelleRunner{
		tracer: otel.GetTracerProvider().Tracer("aspect-gazelle"),
	}

	if os.Getenv("GAZELLE_PROGRESS") != "" && term.IsTerminal(int(os.Stdout.Fd())) {
		c.AddLanguageFactory("progress", progress.NewLanguage)
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

func (c *GazelleRunner) AddLanguage(lang GazelleLanguage) {
	switch lang {
	case JavaScript:
		c.AddLanguageFactory(lang, js.NewLanguage)
	case Orion:
		c.AddLanguageFactory(lang, func() language.Language {
			return orion.NewLanguage()
		})
	case Go:
		c.AddLanguageFactory(lang, golang.NewLanguage)
	case Protobuf:
		c.AddLanguageFactory(lang, proto.NewLanguage)
	case Bzl:
		c.AddLanguageFactory(lang, bzl.NewLanguage)
	case Python:
		c.AddLanguageFactory(lang, python.NewLanguage)
	case CC:
		c.AddLanguageFactory(lang, cc.NewLanguage)
	default:
		log.Fatalf("ERROR: unknown language %q", lang)
	}
}

func (runner *GazelleRunner) PrepareGazelleArgs(mode GazelleMode, excludes []string, args []string) (string, []string) {
	var wd string
	if wsRoot := bazel.FindWorkspaceDirectory(); wsRoot != "" {
		wd = wsRoot
	} else {
		var err error
		if wd, err = os.Getwd(); err != nil {
			log.Fatal(err)
		}
	}

	// Append the aspect-cli mode flag to the args parsed by gazelle.
	fixArgs := []string{"--mode=" + mode}

	for _, exclude := range excludes {
		fixArgs = append(fixArgs, "--exclude="+exclude)
	}

	// Append additional args including specific directories to fix.
	fixArgs = append(fixArgs, args...)

	return wd, fixArgs
}

// Instantiate an instance of each language enabled in this GazelleRunner instance.
func (runner *GazelleRunner) InstantiateLanguages() []language.Language {
	languages := make([]language.Language, 0, len(runner.languages))
	for _, lang := range runner.languages {
		languages = append(languages, lang())
	}
	return languages
}

func (runner *GazelleRunner) Generate(mode GazelleMode, excludes []string, args []string) (bool, error) {
	_, t := runner.tracer.Start(context.Background(), "GazelleRunner.Generate", trace.WithAttributes(
		traceAttr.String("mode", mode),
		traceAttr.StringSlice("languages", runner.languageKeys),
		traceAttr.StringSlice("excludes", excludes),
		traceAttr.StringSlice("args", args),
	))
	defer t.End()

	wd, fixArgs := runner.PrepareGazelleArgs(mode, excludes, args)

	if mode == "fix" {
		fmt.Printf("Updating BUILD files for %s\n", strings.Join(runner.languageKeys, ", "))
	}

	// Run gazelle
	visited, updated, err := vendoredGazelle.RunGazelleFixUpdate(wd, runner.InstantiateLanguages(), fixArgs)

	if mode == "fix" {
		fmt.Printf("%v BUILD %s visited\n", visited, pluralize("file", visited))
		fmt.Printf("%v BUILD %s updated\n", updated, pluralize("file", updated))
	}

	return updated > 0, err
}

// A Generate()-like function desinged for running in gazelle test targets
func (runner *GazelleRunner) Test() (bool, error) {
	wd, fixArgs := runner.PrepareGazelleArgs(Fix, []string{}, []string{
		os.Getenv("BUILD_WORKSPACE_DIRECTORY"),
	})
	_, updated, err := vendoredGazelle.RunGazelleFixUpdate(wd, runner.InstantiateLanguages(), fixArgs)
	return updated > 0, err
}

func (p *GazelleRunner) Watch(watchAddress string, mode GazelleMode, excludes []string, args []string) error {
	watch := ibp.NewClient(watchAddress)
	if err := watch.Connect(); err != nil {
		return fmt.Errorf("failed to connect to watchman: %w", err)
	}

	// Params for the underlying gazelle call
	wd, fixArgs := p.PrepareGazelleArgs(mode, excludes, args)

	// Initial run and status update to stdout.
	fmt.Printf("Initialize BUILD file generation --watch in %v\n", wd)
	languages := p.InstantiateLanguages()
	visited, updated, err := vendoredGazelle.RunGazelleFixUpdate(wd, languages, fixArgs)
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
		traceAttr.StringSlice("excludes", excludes),
		traceAttr.StringSlice("args", args),
	))
	defer t.End()

	// Subscribe to further changes
	for cs := range watch.AwaitCycle() {
		_, t := p.tracer.Start(ctx, "GazelleRunner.Watch.Trigger")

		// The directories that have changed which gazelle should update.
		// This assumes all enabled gazelle languages support incremental updates.
		changedDirs := computeUpdatedDirs(wd, cs.Sources)

		fmt.Printf("Detected changes in %v\n", changedDirs)

		// Run gazelle
		visited, updated, err := vendoredGazelle.RunGazelleFixUpdate(wd, languages, append(fixArgs, changedDirs...))
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

	for f, _ := range changedFiles {
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
