package workspace

import (
	"errors"
	"io/fs"
	"os"
	"path"
	"testing"
	"time"
)

// fakeFileInfo is a minimal fs.FileInfo whose only meaningful property is IsDir.
type fakeFileInfo struct {
	name  string
	isDir bool
}

func (f fakeFileInfo) Name() string       { return f.name }
func (f fakeFileInfo) Size() int64        { return 0 }
func (f fakeFileInfo) Mode() fs.FileMode  { return 0 }
func (f fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (f fakeFileInfo) IsDir() bool        { return f.isDir }
func (f fakeFileInfo) Sys() any           { return nil }

// statFor returns a fake os.Stat backed by the given path -> isDir map. Any path
// not in the map reports os.ErrNotExist (so Find keeps walking up).
func statFor(files map[string]bool) func(string) (fs.FileInfo, error) {
	return func(name string) (fs.FileInfo, error) {
		isDir, ok := files[name]
		if !ok {
			return nil, os.ErrNotExist
		}
		return fakeFileInfo{name: path.Base(name), isDir: isDir}, nil
	}
}

func TestFind(t *testing.T) {
	tests := []struct {
		name     string
		startDir string
		files    map[string]bool // path -> isDir
		want     string
	}{
		{
			name:     "boundary file in the start dir",
			startDir: "/a/b/c",
			files:    map[string]bool{"/a/b/c/MODULE.bazel": false},
			want:     "/a/b/c",
		},
		{
			name:     "walks up to an ancestor boundary",
			startDir: "/a/b/c",
			files:    map[string]bool{"/a/WORKSPACE": false},
			want:     "/a",
		},
		{
			name:     "a directory named like a boundary file is skipped",
			startDir: "/a/b/c",
			files: map[string]bool{
				"/a/b/c/WORKSPACE": true,  // a directory, must not count as the boundary
				"/a/WORKSPACE":     false, // the real boundary file higher up
			},
			want: "/a",
		},
		{
			name:     "multiple boundary files in the same dir resolve to that dir",
			startDir: "/a/b",
			files: map[string]bool{
				"/a/b/MODULE.bazel": false,
				"/a/b/WORKSPACE":    false,
			},
			want: "/a/b",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := &finder{osStat: statFor(tc.files)}
			got, err := f.Find(tc.startDir)
			if err != nil {
				t.Fatalf("Find(%q): unexpected error: %v", tc.startDir, err)
			}
			if got != tc.want {
				t.Errorf("Find(%q) = %q, want %q", tc.startDir, got, tc.want)
			}
		})
	}
}

// With no boundary file anywhere up the tree, Find returns a *NotFoundError that
// records the directory the search started from.
func TestFindNotFound(t *testing.T) {
	f := &finder{osStat: statFor(nil)}

	_, err := f.Find("/a/b/c")
	if !IsNotFoundError(err) {
		t.Fatalf("expected a NotFoundError, got %v", err)
	}
	var nfe *NotFoundError
	if !errors.As(err, &nfe) {
		t.Fatalf("error is not a *NotFoundError: %v", err)
	}
	if nfe.StartDir != "/a/b/c" {
		t.Errorf("NotFoundError.StartDir = %q, want %q", nfe.StartDir, "/a/b/c")
	}
}

// A stat failure other than not-exist (e.g. a permission error) is surfaced
// rather than swallowed as "keep looking".
func TestFindStatError(t *testing.T) {
	boom := errors.New("permission denied")
	f := &finder{osStat: func(string) (fs.FileInfo, error) { return nil, boom }}

	_, err := f.Find("/a/b/c")
	if !errors.Is(err, boom) {
		t.Fatalf("expected the stat error to be surfaced, got %v", err)
	}
	if IsNotFoundError(err) {
		t.Error("a stat error must not be reported as NotFoundError")
	}
}

func TestIsNotFoundError(t *testing.T) {
	if IsNotFoundError(nil) {
		t.Error("nil is not a NotFoundError")
	}
	if !IsNotFoundError(&NotFoundError{StartDir: "/x"}) {
		t.Error("*NotFoundError should be recognized")
	}
	if IsNotFoundError(errors.New("some other error")) {
		t.Error("an unrelated error is not a NotFoundError")
	}
}
