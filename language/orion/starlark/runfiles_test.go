package stareval

import (
	"os"
	"path/filepath"
	"testing"

	"go.starlark.net/starlark"
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
load("../shared/helper.star", "VALUE")
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
	// Paths without a "./" or "../" prefix keep resolving from the workspace
	// root, independent of the loading file's location.
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

	t.Run("missing file reports the runfiles path", func(t *testing.T) {
		root, _ := setupRunfiles(t, mapping)
		writeFile(t, root, "tools/main.star", `
load("@shared_orion//orion:nope.star", "VALUE")
`)

		if _, err := evalIn(t, root, "tools/main.star"); err == nil {
			t.Fatal("expected an error for a file missing from runfiles")
		}
	})

	t.Run("without a runfiles tree", func(t *testing.T) {
		root := t.TempDir()
		t.Setenv("RUNFILES_DIR", "")
		t.Setenv("RUNFILES_MANIFEST_FILE", "")
		writeFile(t, root, "tools/main.star", `
load("@shared_orion//orion:helper.star", "VALUE")
`)

		_, err := evalIn(t, root, "tools/main.star")
		if err == nil {
			t.Fatal("expected an error when RUNFILES_DIR is unset")
		}
	})
}

func TestSourceRepo(t *testing.T) {
	runfiles := "/tmp/x.runfiles"

	for _, tc := range []struct {
		name string
		file string
		want string
	}{
		{"external repo", filepath.Join(runfiles, "shared_orion+", "orion", "lib.star"), "shared_orion+"},
		{"main repo in runfiles", filepath.Join(runfiles, "_main", "tools", "main.star"), ""},
		{"outside the runfiles tree", "/workspace/tools/main.star", ""},
		{"empty", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := sourceRepo(runfiles, tc.file); got != tc.want {
				t.Errorf("sourceRepo(%q) = %q, want %q", tc.file, got, tc.want)
			}
		})
	}
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
