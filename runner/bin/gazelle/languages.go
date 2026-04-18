package main

import (
	"os"
	"slices"
	"strings"

	BazelLog "github.com/aspect-build/aspect-gazelle/common/logger"
	"github.com/aspect-build/aspect-gazelle/runner"
)

var envLanguages = []runner.GazelleLanguage{
	// The gazelle defaults, including initial ordering: https://github.com/bazel-contrib/bazel-gazelle/blob/v0.47.0/def.bzl#L59-L63
	runner.DefaultVisibility,
	runner.Protobuf,
	runner.Go,

	// Additional aspect-runner defaults.
	// Kotlin not included in the prebuild because it interferes with normal operation
	// and there is no directive to disable it.
	// CC not included due to Gazelle CC causing issues in many scenarios with unrelated targets.
	runner.Bzl,
	runner.Python,
	runner.Orion,
	runner.JavaScript,
}

func init() {
	enableLangs := os.Getenv("ENABLE_LANGUAGES")
	envLanguages = resolveLanguages(envLanguages, enableLangs, os.Getenv("ORION_EXTENSIONS"), os.Getenv("ORION_EXTENSIONS_DIR"))
	if enableLangs != "" {
		BazelLog.Infof("Using ENABLE_LANGUAGES from environment: %v", envLanguages)
	}
}

// resolveLanguages applies ENABLE_LANGUAGES and ORION_EXTENSIONS[_DIR] to the
// default list. Pure — env lookups live in init().
func resolveLanguages(defaults []string, enableLangs, orionExt, orionExtDir string) []string {
	var result []string
	if enableLangs == "" {
		result = defaults
	} else {
		result = strings.Split(enableLangs, ",")
	}

	// Automatically include orion if extensions are specified
	if (orionExt != "" || orionExtDir != "") && !slices.Contains(result, runner.Orion) {
		result = append(slices.Clone(result), runner.Orion)
	}

	return result
}
