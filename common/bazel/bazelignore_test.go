package bazel

import (
	"os"
	"path"
	"strings"
	"testing"
)

func TestLoadBazelIgnore(t *testing.T) {
	dir := t.TempDir()
	content := "# comment\nfoo\n\n./bar/baz\nstar-*-glob\n"
	if err := os.WriteFile(path.Join(dir, ".bazelignore"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	excludes, err := loadBazelIgnore(dir)
	if err != nil {
		t.Fatalf("loadBazelIgnore returned error: %v", err)
	}
	if len(excludes) != 2 || excludes[0] != "foo" || excludes[1] != "bar/baz" {
		t.Errorf("unexpected excludes: %v", excludes)
	}
}

func TestLoadBazelIgnoreScannerError(t *testing.T) {
	dir := t.TempDir()

	// A line longer than bufio.MaxScanTokenSize (64KB) causes the scanner to
	// fail mid-file. The error must be reported, not silently truncate the
	// ignore list.
	content := "before\n" + strings.Repeat("x", 1024*1024) + "\nafter\n"
	if err := os.WriteFile(path.Join(dir, ".bazelignore"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := loadBazelIgnore(dir)
	if err == nil {
		t.Fatal("expected an error for a .bazelignore line exceeding the scanner buffer, got nil")
	}
}
