package gazelle

import (
	"os"
	"path/filepath"
	"testing"
)

// writeHostFile creates dir/name with empty content, creating parents.
func writeHostFile(t *testing.T, dir, name string) string {
	t.Helper()

	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(p), err)
	}
	if err := os.WriteFile(p, nil, 0o644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}

	return p
}

func TestResolveEnvPluginPath(t *testing.T) {
	base := t.TempDir()
	wksp := filepath.Join(base, "wksp")
	runfiles := filepath.Join(base, "bin", "gazelle.runfiles")

	writeHostFile(t, wksp, "tools/local.axl")
	writeHostFile(t, base, "escape.axl")
	writeHostFile(t, runfiles, "escape.axl")
	shared := writeHostFile(t, runfiles, "orion_test_plugins+/plugins/shared.axl")

	t.Setenv("RUNFILES_DIR", runfiles)

	for _, tc := range []struct {
		name string
		path string
		want string
	}{
		{"absolute path untouched", shared, shared},
		{"main repository path from the source tree", "tools/local.axl", "tools/local.axl"},
		{"external repository path from runfiles", "../orion_test_plugins+/plugins/shared.axl", filepath.ToSlash(shared)},
		{"path outside the workspace prefers the source tree", "../escape.axl", "../escape.axl"},
		{"missing path untouched", "../orion_test_plugins+/plugins/nope.axl", "../orion_test_plugins+/plugins/nope.axl"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveEnvPluginPath(wksp, tc.path); got != tc.want {
				t.Errorf("resolveEnvPluginPath(%q) = %q, want %q", tc.path, got, tc.want)
			}
		})
	}
}
