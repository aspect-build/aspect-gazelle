package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/aspect-build/aspect-gazelle/runner"
)

func TestParseArgsPositionalArgs(t *testing.T) {
	// parseArgs reads the config file relative to the working directory.
	dir := t.TempDir()
	configYaml := `configure:
  languages:
    go: true
  plugins:
    - my/plugin.star
`
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(configYaml), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	// parseArgs parses os.Args[1:].
	origArgs := os.Args
	t.Cleanup(func() { os.Args = origArgs })
	os.Args = []string{"configure", "--mode=fix", "--config=config.yaml", "foo/bar", "baz"}

	mode, languages, plugins, args := parseArgs()

	if mode != runner.Fix {
		t.Errorf("mode: got %q, want %q", mode, runner.Fix)
	}
	if want := []string{"go"}; !reflect.DeepEqual(languages, want) {
		t.Errorf("languages: got %v, want %v", languages, want)
	}
	if want := []string{"my/plugin.star"}; !reflect.DeepEqual(plugins, want) {
		t.Errorf("plugins: got %v, want %v", plugins, want)
	}

	// Positional arguments after the flags must be returned.
	if want := []string{"foo/bar", "baz"}; !reflect.DeepEqual(args, want) {
		t.Errorf("args: got %v, want %v", args, want)
	}
}
