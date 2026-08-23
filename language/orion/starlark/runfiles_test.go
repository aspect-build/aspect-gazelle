package stareval

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// writeFile creates dir/name with the given content, creating parents.
func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()

	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(p), err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}

	return p
}

func evalIn(t *testing.T, rootDir, starpath string) (starlark.StringDict, error) {
	t.Helper()
	return Eval(rootDir, starpath, make(map[string]starlark.Value), make(map[string]any))
}

func assertErrorContains(t *testing.T, err error, want string) {
	t.Helper()

	if err == nil {
		t.Fatalf("expected an error containing %q, got none", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Errorf("expected an error containing %q, got: %v", want, err)
	}
}

func assertGreeting(t *testing.T, globals starlark.StringDict, want string) {
	t.Helper()

	v, ok := globals["greeting"]
	if !ok {
		t.Fatalf("expected `greeting` to be defined, got %v", globals)
	}
	got, _ := starlark.AsString(v)
	if got != want {
		t.Errorf("expected greeting %q, got %q", want, got)
	}
}

func TestRelativeLoad(t *testing.T) {
	t.Run("sibling of the loading file", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, root, "plugins/helper.star", `VALUE = "hello"`)
		writeFile(t, root, "plugins/main.star", `
load("./helper.star", "VALUE")
greeting = VALUE
`)

		globals, err := evalIn(t, root, "plugins/main.star")
		if err != nil {
			t.Fatal(err)
		}
		assertGreeting(t, globals, "hello")
	})

	t.Run("parent of the loading file", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, root, "shared/helper.star", `VALUE = "from parent"`)
		writeFile(t, root, "plugins/main.star", `
load("./../shared/helper.star", "VALUE")
greeting = VALUE
`)

		globals, err := evalIn(t, root, "plugins/main.star")
		if err != nil {
			t.Fatal(err)
		}
		assertGreeting(t, globals, "from parent")
	})

	t.Run("transitively, from the loaded file", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, root, "a/b/leaf.star", `VALUE = "leaf"`)
		writeFile(t, root, "a/mid.star", `
load("./b/leaf.star", "VALUE")
MID = VALUE
`)
		writeFile(t, root, "main.star", `
load("./a/mid.star", "MID")
greeting = MID
`)

		globals, err := evalIn(t, root, "main.star")
		if err != nil {
			t.Fatal(err)
		}
		assertGreeting(t, globals, "leaf")
	})

	t.Run("outside the workspace root", func(t *testing.T) {
		// A plugin materialized outside the workspace (eg in a runfiles tree)
		// must still be able to load its own files.
		base := t.TempDir()
		root := filepath.Join(base, "workspace")
		if err := os.MkdirAll(root, 0o755); err != nil {
			t.Fatal(err)
		}
		writeFile(t, base, "external/helper.star", `VALUE = "external"`)
		writeFile(t, base, "external/main.star", `
load("./helper.star", "VALUE")
greeting = VALUE
`)

		globals, err := evalIn(t, root, "../external/main.star")
		if err != nil {
			t.Fatal(err)
		}
		assertGreeting(t, globals, "external")
	})
}

func TestWorkspaceRelativeLoadUnchanged(t *testing.T) {
	t.Run("bare path", func(t *testing.T) {
		// Paths without a "./" prefix keep resolving from the workspace root,
		// independent of the loading file's location.
		root := t.TempDir()
		writeFile(t, root, "shared/helper.star", `VALUE = "root relative"`)
		writeFile(t, root, "deep/nested/main.star", `
load("shared/helper.star", "VALUE")
greeting = VALUE
`)

		globals, err := evalIn(t, root, "deep/nested/main.star")
		if err != nil {
			t.Fatal(err)
		}
		assertGreeting(t, globals, "root relative")
	})

	t.Run("escaping the workspace with ..", func(t *testing.T) {
		// A "../" path resolves from the workspace root, not from the loading
		// file, so a plugin can reach shared code kept beside the workspace.
		// Plugins already depend on this; "./../" is the file-relative form.
		base := t.TempDir()
		root := filepath.Join(base, "workspace")
		if err := os.MkdirAll(root, 0o755); err != nil {
			t.Fatal(err)
		}
		writeFile(t, base, "shared/helper.star", `VALUE = "beside the workspace"`)
		writeFile(t, root, "deep/nested/main.star", `
load("../shared/helper.star", "VALUE")
greeting = VALUE
`)

		globals, err := evalIn(t, root, "deep/nested/main.star")
		if err != nil {
			t.Fatal(err)
		}
		assertGreeting(t, globals, "beside the workspace")
	})
}

// setupRunfiles builds a minimal runfiles tree containing the main repo and one
// external repo, plus the _repo_mapping file Bazel writes alongside them.
func setupRunfiles(t *testing.T, repoMapping string) (root, runfiles string) {
	t.Helper()

	base := t.TempDir()
	root = filepath.Join(base, "workspace")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}

	runfiles = filepath.Join(base, "bin", "gazelle.runfiles")
	writeFile(t, runfiles, "shared_orion+/orion/helper.star", `VALUE = "from external repo"`)
	writeFile(t, runfiles, "shared_orion+/orion/lib.star", `
load("./helper.star", "VALUE")
SHARED = VALUE
`)
	writeFile(t, runfiles, "_repo_mapping", repoMapping)

	t.Setenv("RUNFILES_DIR", runfiles)

	return root, runfiles
}

func TestRepositoryLoad(t *testing.T) {
	const mapping = ",shared_orion,shared_orion+\nshared_orion+,shared_orion,shared_orion+\n"

	t.Run("from a main repository plugin", func(t *testing.T) {
		root, _ := setupRunfiles(t, mapping)
		writeFile(t, root, "tools/main.star", `
load("@shared_orion//orion:lib.star", "SHARED")
greeting = SHARED
`)

		globals, err := evalIn(t, root, "tools/main.star")
		if err != nil {
			t.Fatal(err)
		}
		assertGreeting(t, globals, "from external repo")
	})

	t.Run("apparent name resolved via repo mapping", func(t *testing.T) {
		// The main repo knows the module under a different apparent name than
		// its canonical one.
		root, _ := setupRunfiles(t, ",my_alias,shared_orion+\n")
		writeFile(t, root, "tools/main.star", `
load("@my_alias//orion:helper.star", "VALUE")
greeting = VALUE
`)

		globals, err := evalIn(t, root, "tools/main.star")
		if err != nil {
			t.Fatal(err)
		}
		assertGreeting(t, globals, "from external repo")
	})

	t.Run("canonical name without a mapping entry", func(t *testing.T) {
		root, _ := setupRunfiles(t, "")
		writeFile(t, root, "tools/main.star", `
load("@shared_orion+//orion:helper.star", "VALUE")
greeting = VALUE
`)

		globals, err := evalIn(t, root, "tools/main.star")
		if err != nil {
			t.Fatal(err)
		}
		assertGreeting(t, globals, "from external repo")
	})

	t.Run("a plugin loading its own files relatively", func(t *testing.T) {
		// The shared plugin's own load() needs no repository mapping, which is
		// why relative paths are the form to reach for when publishing plugins.
		root, _ := setupRunfiles(t, mapping)
		writeFile(t, root, "tools/main.star", `
load("@shared_orion//orion:lib.star", "SHARED")
greeting = SHARED
`)

		globals, err := evalIn(t, root, "tools/main.star")
		if err != nil {
			t.Fatal(err)
		}
		assertGreeting(t, globals, "from external repo")
	})

	t.Run("path without an explicit target", func(t *testing.T) {
		// label.Parse infers Name from the last segment of Pkg when no target
		// is given, so the file must not be appended twice.
		root, _ := setupRunfiles(t, mapping)
		writeFile(t, root, "tools/main.star", `
load("@shared_orion//orion/helper.star", "VALUE")
greeting = VALUE
`)

		globals, err := evalIn(t, root, "tools/main.star")
		if err != nil {
			t.Fatal(err)
		}
		assertGreeting(t, globals, "from external repo")
	})

	t.Run("missing file reports the runfiles lookup", func(t *testing.T) {
		root, _ := setupRunfiles(t, mapping)
		writeFile(t, root, "tools/main.star", `
load("@shared_orion//orion:nope.star", "VALUE")
`)

		_, err := evalIn(t, root, "tools/main.star")
		assertErrorContains(t, err, "not found in runfiles")
	})

	t.Run("unknown repository names the label", func(t *testing.T) {
		root, _ := setupRunfiles(t, mapping)
		writeFile(t, root, "tools/main.star", `
load("@never_declared//orion:helper.star", "VALUE")
`)

		_, err := evalIn(t, root, "tools/main.star")
		assertErrorContains(t, err, "@never_declared//orion:helper.star")
	})
}

// TestRepositoryLoadFromBazelRunfiles resolves a repository load() through the
// runfiles tree Bazel staged for this test, covering the real tree layout and
// the real _repo_mapping rather than the fakes built by setupRunfiles.
// paths.bzl is staged via the test target's data.
func TestRepositoryLoadFromBazelRunfiles(t *testing.T) {
	if os.Getenv("RUNFILES_DIR") == "" && os.Getenv("RUNFILES_MANIFEST_FILE") == "" {
		t.Skip("no Bazel runfiles tree; run via bazel test")
	}

	root := t.TempDir()
	writeFile(t, root, "tools/main.star", `
load("@bazel_skylib//lib:paths.bzl", "paths")
greeting = paths.basename("dir/leaf")
`)

	// paths.bzl uses the bazel `struct` builtin, which is not part of the
	// starlark-go base environment.
	libs := starlark.StringDict{
		"struct": starlark.NewBuiltin("struct", starlarkstruct.Make),
	}
	globals, err := Eval(root, "tools/main.star", libs, make(map[string]any))
	if err != nil {
		t.Fatal(err)
	}
	assertGreeting(t, globals, "leaf")
}

func TestSourceRepo(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "x.runfiles")
	t.Setenv("RUNFILES_DIR", dir)

	for _, tc := range []struct {
		name string
		file string
		want string
	}{
		{"external repo", filepath.Join(dir, "shared_orion+", "orion", "lib.star"), "shared_orion+"},
		{"main repo in runfiles", filepath.Join(dir, "_main", "tools", "main.star"), ""},
		{"outside the runfiles tree", "/workspace/tools/main.star", ""},
		{"sibling sharing a name prefix", filepath.Join(dir+"2", "shared_orion+", "lib.star"), ""},
		{"empty", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := sourceRepo(tc.file); got != tc.want {
				t.Errorf("sourceRepo(%q) = %q, want %q", tc.file, got, tc.want)
			}
		})
	}

	t.Run("without a runfiles directory", func(t *testing.T) {
		t.Setenv("RUNFILES_DIR", "")
		if got := sourceRepo(filepath.Join(dir, "shared_orion+", "lib.star")); got != "" {
			t.Errorf("expected the main repository, got %q", got)
		}
	})
}

func TestResolveRunfile(t *testing.T) {
	base := t.TempDir()
	runfiles := filepath.Join(base, "gazelle.runfiles")
	writeFile(t, runfiles, "shared_orion+/orion/maven.star", "")
	writeFile(t, runfiles, "_main/tools/local.star", "")
	t.Setenv("RUNFILES_DIR", runfiles)

	t.Run("external short path resolves", func(t *testing.T) {
		got, ok := ResolveRunfile("../shared_orion+/orion/maven.star")
		if !ok {
			t.Fatal("expected the external plugin to resolve")
		}
		if want := filepath.Join(runfiles, "shared_orion+/orion/maven.star"); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("main repo short path is left alone", func(t *testing.T) {
		// Main repo plugins are read from the source tree so edits apply
		// without rebuilding the gazelle target.
		if _, ok := ResolveRunfile("tools/local.star"); ok {
			t.Error("expected a main repository short path not to resolve")
		}
	})

	t.Run("missing external file does not resolve", func(t *testing.T) {
		if _, ok := ResolveRunfile("../shared_orion+/orion/nope.star"); ok {
			t.Error("expected a missing file not to resolve")
		}
	})
}
