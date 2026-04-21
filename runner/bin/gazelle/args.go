package main

import (
	"log"
	"slices"
	"strings"

	"github.com/aspect-build/aspect-gazelle/runner"
)

// cacheType selects a cache implementation for --cache[=disk|watchman].
type cacheType string

const (
	cacheDefault  cacheType = ""
	cacheDisk     cacheType = "disk"
	cacheWatchman cacheType = "watchman"
)

/**
 * Parse and extract arguments not directly passed along to gazelle.
 */
func parseArgs(args []string) (runner.GazelleCommand, runner.GazelleMode, bool, cacheType, []string) {
	// The optional initial command argument
	cmd := runner.UpdateCmd
	if len(args) > 0 && (args[0] == runner.UpdateCmd || args[0] == runner.FixCmd) {
		cmd = args[0]
		args = args[1:]
	}

	// The optional --mode flag
	mode, args := extractArg("mode", runner.Fix, args)

	// The optional --progress flag
	progress, args := extractFlag("progress", false, args)

	// The optional --cache[=disk|watchman] flag; bare --cache defaults to disk.
	cacheRaw, args := extractOptionalArg("cache", string(cacheDisk), args)
	ct := cacheType(cacheRaw)
	switch ct {
	case cacheDefault, cacheDisk, cacheWatchman:
	default:
		log.Fatalf("ERROR: invalid --cache value %q, expected \"disk\" or \"watchman\"", cacheRaw)
	}

	return cmd, mode, progress, ct, args
}

func extractFlag(flag string, defaultValue bool, args []string) (bool, []string) {
	if i := slices.Index(args, "--"+flag); i != -1 {
		args = slices.Delete(args, i, i+1)
		return true, args
	}

	// Also support single-dash flags to align with golang FlagSet and gazelle
	if i := slices.Index(args, "-"+flag); i != -1 {
		args = slices.Delete(args, i, i+1)
		return true, args
	}

	return defaultValue, args
}

func extractArg(flag string, defaultValue string, args []string) (string, []string) {
	// Also support single-dash flags to align with golang FlagSet and gazelle
	f1 := "-" + flag
	f2 := "--" + flag

	i := slices.IndexFunc(args, func(s string) bool {
		return s == f1 || s == f2 || strings.HasPrefix(s, f1+"=") || strings.HasPrefix(s, f2+"=")
	})

	if i == -1 {
		return defaultValue, args
	}

	usedFlag, value, hasEqual := strings.Cut(args[i], "=")
	if !hasEqual {
		if len(args) <= i+1 {
			log.Fatalf("ERROR: %s flag requires an argument", usedFlag)
			return defaultValue, args
		}
		value := args[i+1]
		args = slices.Delete(args, i, i+2)
		return value, args
	}

	args = slices.Delete(args, i, i+1)
	return value, args
}

// Extract a --flag[=value] flag. Bare --flag returns defaultBare without consuming a following token.
func extractOptionalArg(flag, defaultBare string, args []string) (string, []string) {
	f1 := "-" + flag
	f2 := "--" + flag

	i := slices.IndexFunc(args, func(s string) bool {
		return s == f1 || s == f2 || strings.HasPrefix(s, f1+"=") || strings.HasPrefix(s, f2+"=")
	})

	if i == -1 {
		return "", args
	}

	_, value, _ := strings.Cut(args[i], "=")
	if value == "" {
		value = defaultBare
	}
	return value, slices.Delete(args, i, i+1)
}
