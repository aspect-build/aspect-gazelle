package main

import (
	"log"
	"os"
	"slices"
	"strings"

	"github.com/aspect-build/aspect-gazelle/common/bazel"
	BazelLog "github.com/aspect-build/aspect-gazelle/common/logger"
	"github.com/aspect-build/aspect-gazelle/runner"
	"github.com/aspect-build/aspect-gazelle/runner/pkg/ibp"
)

var envLanguages = []runner.GazelleLanguage{
	// Kotlin not included in the prebuild because it interferes with normal operation
	// and there is no directive to disable it.
	// runner.Kotlin,
	// CC not included due to Gazelle CC causing issues in many scenarios with unrelated targets.
	runner.Go,
	runner.Protobuf,
	runner.Bzl,
	runner.Python,
	runner.Orion,
	runner.JavaScript,
}

func init() {
	if langs := os.Getenv("ENABLE_LANGUAGES"); langs != "" {
		envLanguages = strings.Split(langs, ",")

		BazelLog.Infof("Using ENABLE_LANGUAGES from environment: %v", envLanguages)

		// Automatically include orion if extensions are specified
		if (os.Getenv("ORION_EXTENSIONS") != "" || os.Getenv("ORION_EXTENSIONS_DIR") != "") && !slices.Contains(envLanguages, runner.Orion) {
			envLanguages = append(envLanguages, runner.Orion)
		}
	}
}

/**
 * A `gazelle_binary` replacement where languages can be toggled at runtime.
 *
 * Supports additional features such as incremental builds via the Incremental Build Protocol,
 * interactive terminal progress, tracing and more.
 */
func main() {
	log.SetPrefix("aspect-gazelle: ")
	log.SetFlags(0) // don't print timestamps

	wd := bazel.FindWorkspaceDirectory()

	cmd, mode, progress, args := parseArgs()

	c := runner.New(wd, progress)

	// Add languages
	for _, lang := range envLanguages {
		c.AddLanguage(lang)
	}

	if watchSocket := os.Getenv(ibp.PROTOCOL_SOCKET_ENV); watchSocket != "" {
		err := c.Watch(watchSocket, cmd, mode, args)
		if err != nil {
			log.Fatalf("Error running gazelle watcher: %v", err)
		}
	} else {
		_, err := c.Generate(cmd, mode, args)
		if err != nil {
			log.Fatalf("Error running gazelle: %v", err)
		}
	}
}

/**
 * Parse and extract arguments not directly passed along to gazelle.
 */
func parseArgs() (runner.GazelleCommand, runner.GazelleMode, bool, []string) {
	args := os.Args[1:]

	// The optional initial command argument
	cmd := runner.UpdateCmd
	if len(args) > 0 && (args[0] == runner.UpdateCmd || args[0] == runner.FixCmd) {
		cmd = args[0]
		args = args[1:]
	}

	// The optional --mode flag
	mode, args := extractArg("--mode", runner.Fix, args)

	// The optional --progress flag
	progress, args := extractFlag("--progress", false, args)

	return cmd, mode, progress, args
}

func extractFlag(flag string, defaultValue bool, args []string) (bool, []string) {
	if i := slices.Index(args, flag); i != -1 {
		args = append(args[:i], args[i+1:]...)
		return true, args
	}

	return defaultValue, args
}

func extractArg(flag string, defaultValue string, args []string) (string, []string) {
	i := slices.IndexFunc(args, func(s string) bool {
		return s == flag || strings.HasPrefix(s, flag+"=")
	})

	if i == -1 {
		return defaultValue, args
	}

	if args[i] == flag {
		if len(args) == i {
			log.Fatalf("ERROR: %s flag requires an argument", flag)
			return defaultValue, args
		}
		value := args[i+1]
		args = append(args[:i], args[i+2:]...)
		return value, args
	}

	value := strings.SplitN(args[i], "=", 2)[1]
	args = append(args[:i], args[i+1:]...)
	return value, args
}
