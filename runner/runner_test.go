package runner

import (
	"errors"
	"testing"

	"github.com/aspect-build/aspect-gazelle/runner/vendored/bzl"
	"github.com/bazelbuild/bazel-gazelle/language"
)

func TestWalkCacheEntryInvalidated(t *testing.T) {
	cases := []struct {
		name string
		rel  string
		dirs []string
		want bool
	}{
		{
			name: "no invalidated dirs",
			rel:  "pkg",
			dirs: nil,
			want: false,
		},
		{
			name: "exact match",
			rel:  "pkg/sub",
			dirs: []string{"pkg/sub"},
			want: true,
		},
		{
			name: "descendant of invalidated dir",
			rel:  "pkg/sub/leaf",
			dirs: []string{"pkg/sub"},
			want: true,
		},
		{
			name: "unrelated dir",
			rel:  "other",
			dirs: []string{"pkg"},
			want: false,
		},
		// Guards against the "pkg" prefix accidentally matching "pkg-foo": the '/' check
		// in the predicate is the reason this returns false.
		{
			name: "prefix-but-not-descendant",
			rel:  "pkg-foo",
			dirs: []string{"pkg"},
			want: false,
		},
		// Root-change cases: path.Dir returns "." for files at the workspace root,
		// but the gazelle walk cache keys the root entry as "". Both must invalidate
		// every entry, matching invalidateWalkCache in pkg/watchman/cache.go.
		{
			name: "root dot invalidates root entry",
			rel:  "",
			dirs: []string{"."},
			want: true,
		},
		{
			name: "root dot invalidates nested entry",
			rel:  "pkg/sub",
			dirs: []string{"."},
			want: true,
		},
		{
			name: "root empty invalidates root entry",
			rel:  "",
			dirs: []string{""},
			want: true,
		},
		{
			name: "root empty invalidates nested entry",
			rel:  "pkg/sub",
			dirs: []string{""},
			want: true,
		},
		{
			name: "any matching dir wins",
			rel:  "pkg/sub",
			dirs: []string{"other", "pkg/sub", "more"},
			want: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := walkCacheEntryInvalidated(tc.rel, tc.dirs)
			if got != tc.want {
				t.Errorf("walkCacheEntryInvalidated(%q, %v) = %v, want %v", tc.rel, tc.dirs, got, tc.want)
			}
		})
	}
}

// A language that fails to build must come back as an error, never take the
// process with it. instantiateLanguages runs again on every --watch cycle, so
// an exit there would kill the watcher -- skipping Watch's deferred
// watch.Disconnect() and stranding the IBP subscription -- over an extension
// the user is in the middle of fixing.
func TestInstantiateLanguagesSurvivesABrokenLanguage(t *testing.T) {
	broken := errors.New(`Failed to load orion plugin "broken.axl": broken.axl:1:1: syntax error`)

	failing := true
	c := New(t.TempDir(), false)
	c.AddLanguageFactoryOrError("orion", func() (language.Language, error) {
		if failing {
			return nil, broken
		}
		return bzl.NewLanguage(), nil
	})

	langs, err := c.instantiateLanguages()
	if err == nil {
		t.Fatal("instantiateLanguages returned nil error for a broken language")
	}
	if langs != nil {
		t.Errorf("instantiateLanguages returned %d languages alongside an error, want none", len(langs))
	}
	var setupErr *SetupError
	if !errors.As(err, &setupErr) {
		t.Errorf("instantiateLanguages error is %T, want *SetupError so callers can map it to ExitCodeSetupError", err)
	}
	if !errors.Is(err, broken) {
		t.Errorf("instantiateLanguages error = %q, want it to wrap the language's own error", err)
	}

	// The next watch cycle must recover once the extension parses again.
	failing = false
	langs, err = c.instantiateLanguages()
	if err != nil {
		t.Fatalf("instantiateLanguages after the fix: %v", err)
	}
	if len(langs) != 1 {
		t.Errorf("instantiateLanguages returned %d languages, want 1", len(langs))
	}
}

// ExitCodeSetupError must stay off 1, which already means both "BUILD files
// are out of date" (--mode=diff) and log.Fatalf in the runner binaries.
func TestExitCodeSetupErrorIsDistinct(t *testing.T) {
	if ExitCodeSetupError == 0 || ExitCodeSetupError == 1 {
		t.Errorf("ExitCodeSetupError = %d, want a code gazelle does not already use", ExitCodeSetupError)
	}
}
