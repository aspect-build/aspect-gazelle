package gazelle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	BazelLog "github.com/aspect-build/aspect-gazelle/common/logger"
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

// A load failure must name the extension that failed. Several of the starlark
// errors carry no filename of their own ("cannot load ./lib.star: ..."), so
// with more than one extension configured the message alone would not say
// which file to go fix.
func TestLoadPluginErrorNamesThePlugin(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "broken.axl"), []byte("x = 'unterminated\n"), 0o644); err != nil {
		t.Fatalf("write broken.axl: %v", err)
	}

	err := newGazelleHost().LoadPlugin(dir, "broken.axl")
	if err == nil {
		t.Fatal("LoadPlugin of an unparseable extension returned nil, want an error")
	}
	if !strings.Contains(err.Error(), `"broken.axl"`) {
		t.Errorf("LoadPlugin error = %q, want it to name the plugin", err)
	}
	// The sandbox/plugin directory must not leak into the message.
	if strings.Contains(err.Error(), dir) {
		t.Errorf("LoadPlugin error = %q, want %q stripped", err, dir)
	}
}

// Loading after gazelle has read Kinds()/ApparentLoads() cannot work -- the
// cached rule metadata has already been handed out -- and must be reported
// rather than silently accepted.
func TestLoadPluginAfterConfigurationStarted(t *testing.T) {
	h := newGazelleHost()
	h.Kinds() // gazelle calls this once configuration/data-collection starts.

	err := h.LoadPlugin(t.TempDir(), "late.axl")
	if err == nil {
		t.Fatal("LoadPlugin after configuration returned nil, want an error")
	}
	if !strings.Contains(err.Error(), "after configuration has started") {
		t.Errorf("LoadPlugin error = %q, want it to explain the ordering", err)
	}
}

// The failure must also reach the log: anyone debugging from ASPECT_LOG_FILE
// (or a bazel test's undeclared outputs, which is where BazelLog goes here)
// otherwise sees LoadProxy's "Evaluate orion plugin" line and no indication
// that the run then died of a plugin error.
//
// This pins that the record exists, not its level. Bazel tests force
// DebugLevel, so Errorf and Infof are indistinguishable here; error level
// matters outside tests, where the default WarnLevel drops info.
func TestLoadPluginErrorIsLogged(t *testing.T) {
	outputs := os.Getenv("TEST_UNDECLARED_OUTPUTS_DIR")
	if outputs == "" || BazelLog.GetOutput() == os.Stderr {
		t.Skip("BazelLog is not writing to a file in this environment")
	}
	logs, err := filepath.Glob(filepath.Join(outputs, "*.log"))
	if err != nil || len(logs) == 0 {
		t.Skipf("no BazelLog file in %s", outputs)
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "logged.axl"), []byte("y = 'unterminated\n"), 0o644); err != nil {
		t.Fatalf("write logged.axl: %v", err)
	}
	if err := newGazelleHost().LoadPlugin(dir, "logged.axl"); err == nil {
		t.Fatal("LoadPlugin of an unparseable extension returned nil, want an error")
	}

	for _, l := range logs {
		content, err := os.ReadFile(l)
		if err != nil {
			t.Fatalf("read %s: %v", l, err)
		}
		// Match the failure, not LoadProxy's "Evaluate orion plugin" line,
		// which names the same file whether or not the failure is recorded.
		if strings.Contains(string(content), `Failed to load orion plugin "logged.axl"`) {
			return
		}
	}
	t.Errorf("no BazelLog file in %s records the failed load of logged.axl", outputs)
}
