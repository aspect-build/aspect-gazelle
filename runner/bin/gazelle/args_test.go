package main

import (
	"reflect"
	"testing"

	"github.com/aspect-build/aspect-gazelle/runner"
)

func TestParseArgs(t *testing.T) {
	cases := []struct {
		name      string
		argv      []string
		wantCmd   runner.GazelleCommand
		wantMode  runner.GazelleMode
		wantProg  bool
		wantCache cacheType
		wantArgs  []string
	}{
		// Default: `gazelle` with no args -> update + fix (matches bazel-gazelle default command).
		{
			name:     "no args",
			argv:     []string{},
			wantCmd:  runner.UpdateCmd,
			wantMode: runner.Fix,
			wantArgs: []string{},
		},

		// Explicit commands: bazel-gazelle accepts `update` and `fix` as the first positional arg.
		{
			name:     "update command",
			argv:     []string{"update"},
			wantCmd:  runner.UpdateCmd,
			wantMode: runner.Fix,
			wantArgs: []string{},
		},
		{
			name:     "fix command",
			argv:     []string{"fix"},
			wantCmd:  runner.FixCmd,
			wantMode: runner.Fix,
			wantArgs: []string{},
		},

		// --mode / -mode: bazel-gazelle uses Go's flag package which accepts both single
		// and double dashes, and both `flag=value` and `flag value`.
		{
			name:     "-mode=diff",
			argv:     []string{"-mode=diff"},
			wantCmd:  runner.UpdateCmd,
			wantMode: runner.Diff,
			wantArgs: []string{},
		},
		{
			name:     "--mode=diff",
			argv:     []string{"--mode=diff"},
			wantCmd:  runner.UpdateCmd,
			wantMode: runner.Diff,
			wantArgs: []string{},
		},
		{
			name:     "-mode diff (space-separated)",
			argv:     []string{"-mode", "diff"},
			wantCmd:  runner.UpdateCmd,
			wantMode: runner.Diff,
			wantArgs: []string{},
		},
		{
			name:     "--mode diff (space-separated)",
			argv:     []string{"--mode", "diff"},
			wantCmd:  runner.UpdateCmd,
			wantMode: runner.Diff,
			wantArgs: []string{},
		},
		{
			name:     "-mode=print",
			argv:     []string{"-mode=print"},
			wantCmd:  runner.UpdateCmd,
			wantMode: runner.Print,
			wantArgs: []string{},
		},

		// --progress is an aspect-gazelle addition but follows the same dash conventions.
		{
			name:     "-progress",
			argv:     []string{"-progress"},
			wantCmd:  runner.UpdateCmd,
			wantMode: runner.Fix,
			wantProg: true,
			wantArgs: []string{},
		},
		{
			name:     "--progress",
			argv:     []string{"--progress"},
			wantCmd:  runner.UpdateCmd,
			wantMode: runner.Fix,
			wantProg: true,
			wantArgs: []string{},
		},

		// Command + flags + positional dirs in combination.
		{
			name:     "fix + mode + progress + path",
			argv:     []string{"fix", "-mode=diff", "--progress", "some/path"},
			wantCmd:  runner.FixCmd,
			wantMode: runner.Diff,
			wantProg: true,
			wantArgs: []string{"some/path"},
		},
		{
			name:     "update + paths",
			argv:     []string{"update", "a/b", "c/d"},
			wantCmd:  runner.UpdateCmd,
			wantMode: runner.Fix,
			wantArgs: []string{"a/b", "c/d"},
		},

		// Flags we don't own must be forwarded unchanged to the underlying gazelle
		// FlagSet (e.g. -r, -patch, -print0, -known_import, -repo_config).
		{
			name:     "forwards unknown gazelle flags",
			argv:     []string{"-r=false", "-patch=out.patch", "-known_import=foo"},
			wantCmd:  runner.UpdateCmd,
			wantMode: runner.Fix,
			wantArgs: []string{"-r=false", "-patch=out.patch", "-known_import=foo"},
		},

		// Directory-only invocation: positional args must pass through untouched.
		{
			name:     "positional paths only",
			argv:     []string{"pkg/foo", "pkg/bar"},
			wantCmd:  runner.UpdateCmd,
			wantMode: runner.Fix,
			wantArgs: []string{"pkg/foo", "pkg/bar"},
		},

		// A non-command first arg must not be consumed as a command. bazel-gazelle
		// only treats `update`/`fix` in the first slot as commands.
		{
			name:     "non-command first arg is a path",
			argv:     []string{"pkg/foo", "update"},
			wantCmd:  runner.UpdateCmd,
			wantMode: runner.Fix,
			wantArgs: []string{"pkg/foo", "update"},
		},

		// Flag followed by command-like value: -mode update should set mode="update"
		// and leave no trailing command. Matches Go flag parsing semantics.
		{
			name:     "-mode consumes following token even if it looks like a command",
			argv:     []string{"-mode", "fix"},
			wantCmd:  runner.UpdateCmd,
			wantMode: runner.Fix,
			wantArgs: []string{},
		},

		// Flag ordering: command must come first, but otherwise flags/paths can interleave.
		{
			name:     "flag before path after command",
			argv:     []string{"fix", "--progress", "-mode=print", "x/y"},
			wantCmd:  runner.FixCmd,
			wantMode: runner.Print,
			wantProg: true,
			wantArgs: []string{"x/y"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd, mode, progress, ct, args := parseArgs(tc.argv)
			if cmd != tc.wantCmd {
				t.Errorf("cmd: got %q, want %q", cmd, tc.wantCmd)
			}
			if mode != tc.wantMode {
				t.Errorf("mode: got %q, want %q", mode, tc.wantMode)
			}
			if progress != tc.wantProg {
				t.Errorf("progress: got %v, want %v", progress, tc.wantProg)
			}
			if ct != tc.wantCache {
				t.Errorf("cache: got %q, want %q", ct, tc.wantCache)
			}
			if !reflect.DeepEqual(args, tc.wantArgs) {
				t.Errorf("args: got %v, want %v", args, tc.wantArgs)
			}
		})
	}
}

func TestExtractFlag(t *testing.T) {
	cases := []struct {
		name         string
		flag         string
		defaultValue bool
		args         []string
		wantValue    bool
		wantArgs     []string
	}{
		{
			name:      "single-dash present",
			flag:      "progress",
			args:      []string{"-progress"},
			wantValue: true,
			wantArgs:  []string{},
		},
		{
			name:      "double-dash present",
			flag:      "progress",
			args:      []string{"--progress"},
			wantValue: true,
			wantArgs:  []string{},
		},
		{
			name:         "absent returns default true",
			flag:         "progress",
			defaultValue: true,
			args:         []string{"foo", "bar"},
			wantValue:    true,
			wantArgs:     []string{"foo", "bar"},
		},
		{
			name:      "absent returns default false",
			flag:      "progress",
			args:      []string{"foo", "bar"},
			wantValue: false,
			wantArgs:  []string{"foo", "bar"},
		},
		{
			name:      "removed from middle preserves order",
			flag:      "progress",
			args:      []string{"a", "-progress", "b"},
			wantValue: true,
			wantArgs:  []string{"a", "b"},
		},
		{
			name:      "does not match substring",
			flag:      "pro",
			args:      []string{"-progress"},
			wantValue: false,
			wantArgs:  []string{"-progress"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotValue, gotArgs := extractFlag(tc.flag, tc.defaultValue, tc.args)
			if gotValue != tc.wantValue {
				t.Errorf("value: got %v, want %v", gotValue, tc.wantValue)
			}
			if !reflect.DeepEqual(gotArgs, tc.wantArgs) {
				t.Errorf("args: got %v, want %v", gotArgs, tc.wantArgs)
			}
		})
	}
}

func TestExtractArg(t *testing.T) {
	cases := []struct {
		name         string
		flag         string
		defaultValue string
		args         []string
		wantValue    string
		wantArgs     []string
	}{
		// Go flag parsing accepts `-flag value`, `-flag=value`, `--flag value`, `--flag=value`.
		{
			name:      "-flag value",
			flag:      "mode",
			args:      []string{"-mode", "diff"},
			wantValue: "diff",
			wantArgs:  []string{},
		},
		{
			name:      "-flag=value",
			flag:      "mode",
			args:      []string{"-mode=diff"},
			wantValue: "diff",
			wantArgs:  []string{},
		},
		{
			name:      "--flag value",
			flag:      "mode",
			args:      []string{"--mode", "diff"},
			wantValue: "diff",
			wantArgs:  []string{},
		},
		{
			name:      "--flag=value",
			flag:      "mode",
			args:      []string{"--mode=diff"},
			wantValue: "diff",
			wantArgs:  []string{},
		},
		{
			name:         "absent returns default",
			flag:         "mode",
			defaultValue: "fix",
			args:         []string{"some/path"},
			wantValue:    "fix",
			wantArgs:     []string{"some/path"},
		},
		{
			name:      "value may be empty with =",
			flag:      "mode",
			args:      []string{"-mode="},
			wantValue: "",
			wantArgs:  []string{},
		},
		{
			name:      "surrounding args preserved (space form)",
			flag:      "mode",
			args:      []string{"a", "-mode", "diff", "b"},
			wantValue: "diff",
			wantArgs:  []string{"a", "b"},
		},
		{
			name:      "surrounding args preserved (= form)",
			flag:      "mode",
			args:      []string{"a", "--mode=print", "b"},
			wantValue: "print",
			wantArgs:  []string{"a", "b"},
		},
		{
			// bazel-gazelle's flag package also matches the flag name exactly (not as a prefix).
			name:         "does not match flag with matching prefix",
			flag:         "mode",
			defaultValue: "unchanged",
			args:         []string{"-modex=diff"},
			wantValue:    "unchanged",
			wantArgs:     []string{"-modex=diff"},
		},
		{
			name:      "first occurrence wins",
			flag:      "mode",
			args:      []string{"-mode=diff", "-mode=print"},
			wantValue: "diff",
			wantArgs:  []string{"-mode=print"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotValue, gotArgs := extractArg(tc.flag, tc.defaultValue, tc.args)
			if gotValue != tc.wantValue {
				t.Errorf("value: got %q, want %q", gotValue, tc.wantValue)
			}
			if !reflect.DeepEqual(gotArgs, tc.wantArgs) {
				t.Errorf("args: got %v, want %v", gotArgs, tc.wantArgs)
			}
		})
	}
}

// TestProgressFlag focuses on the aspect-specific --progress flag. Standard
// bazel-gazelle has no equivalent, so we pin down: both dash forms, default
// off, no value-accepting form, and that the flag is consumed (not forwarded
// to the underlying gazelle FlagSet which would reject it).
func TestProgressFlag(t *testing.T) {
	cases := []struct {
		name     string
		argv     []string
		wantProg bool
		wantArgs []string
	}{
		{
			name:     "default is off",
			argv:     []string{},
			wantProg: false,
			wantArgs: []string{},
		},
		{
			name:     "single dash enables",
			argv:     []string{"-progress"},
			wantProg: true,
			wantArgs: []string{},
		},
		{
			name:     "double dash enables",
			argv:     []string{"--progress"},
			wantProg: true,
			wantArgs: []string{},
		},
		{
			name:     "consumed from forwarded args",
			argv:     []string{"--progress", "-mode=diff", "pkg"},
			wantProg: true,
			wantArgs: []string{"pkg"},
		},
		{
			name:     "after command",
			argv:     []string{"fix", "--progress"},
			wantProg: true,
			wantArgs: []string{},
		},
		{
			// --progress is boolean-only. "=true"/"=false" are not recognized —
			// they fall through and are forwarded to gazelle as-is.
			name:     "value form is not recognized",
			argv:     []string{"--progress=true"},
			wantProg: false,
			wantArgs: []string{"--progress=true"},
		},
		{
			// Substring name is not matched.
			name:     "--progres (typo) does not match",
			argv:     []string{"--progres"},
			wantProg: false,
			wantArgs: []string{"--progres"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, progress, _, args := parseArgs(tc.argv)
			if progress != tc.wantProg {
				t.Errorf("progress: got %v, want %v", progress, tc.wantProg)
			}
			if !reflect.DeepEqual(args, tc.wantArgs) {
				t.Errorf("args: got %v, want %v", args, tc.wantArgs)
			}
		})
	}
}

// TestCacheFlag covers the aspect-specific --cache[=disk|watchman] flag.
func TestCacheFlag(t *testing.T) {
	cases := []struct {
		name      string
		argv      []string
		wantCache cacheType
		wantArgs  []string
	}{
		{
			name:      "default is off",
			argv:      []string{},
			wantCache: cacheDefault,
			wantArgs:  []string{},
		},
		{
			name:      "bare --cache enables disk",
			argv:      []string{"--cache"},
			wantCache: cacheDisk,
			wantArgs:  []string{},
		},
		{
			name:      "bare -cache enables disk",
			argv:      []string{"-cache"},
			wantCache: cacheDisk,
			wantArgs:  []string{},
		},
		{
			name:      "--cache=disk",
			argv:      []string{"--cache=disk"},
			wantCache: cacheDisk,
			wantArgs:  []string{},
		},
		{
			name:      "--cache=watchman",
			argv:      []string{"--cache=watchman"},
			wantCache: cacheWatchman,
			wantArgs:  []string{},
		},
		{
			name:      "-cache=watchman",
			argv:      []string{"-cache=watchman"},
			wantCache: cacheWatchman,
			wantArgs:  []string{},
		},
		{
			name:      "--cache= (empty) treated as bare",
			argv:      []string{"--cache="},
			wantCache: cacheDisk,
			wantArgs:  []string{},
		},
		{
			// --cache does NOT consume a following positional arg as its value,
			// unlike --mode. This matches the cli's stdlib IsBoolFlag behavior.
			name:      "bare --cache does not consume following positional",
			argv:      []string{"--cache", "pkg/foo"},
			wantCache: cacheDisk,
			wantArgs:  []string{"pkg/foo"},
		},
		{
			name:      "consumed from forwarded args",
			argv:      []string{"--cache=disk", "-mode=diff", "pkg"},
			wantCache: cacheDisk,
			wantArgs:  []string{"pkg"},
		},
		{
			// Substring name is not matched.
			name:      "--cach (typo) does not match",
			argv:      []string{"--cach"},
			wantCache: cacheDefault,
			wantArgs:  []string{"--cach"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, _, ct, args := parseArgs(tc.argv)
			if ct != tc.wantCache {
				t.Errorf("cache: got %q, want %q", ct, tc.wantCache)
			}
			if !reflect.DeepEqual(args, tc.wantArgs) {
				t.Errorf("args: got %v, want %v", args, tc.wantArgs)
			}
		})
	}
}
