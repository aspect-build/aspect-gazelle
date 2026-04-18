package main

import (
	"log"
	"slices"
	"strings"

	"github.com/aspect-build/aspect-gazelle/runner"
)

/**
 * Parse and extract arguments not directly passed along to gazelle.
 */
func parseArgs(args []string) (runner.GazelleCommand, runner.GazelleMode, bool, []string) {
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

	return cmd, mode, progress, args
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
