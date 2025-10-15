package watchman

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// Workaround: https://github.com/facebook/watchman/issues/662#issuecomment-1135757635
// Watchman does not like it when the root is a symlink
func getTempDir(t *testing.T) string {
	tmp, err := filepath.EvalSymlinks("/tmp")
	if err != nil {
		t.Fatal(err)
	}
	tmp, err = os.MkdirTemp(tmp, "watchman-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Log(tmp)
	return tmp
}

func getTempFile(t *testing.T) string {
	f, err := os.CreateTemp(os.TempDir(), "test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Log(f.Name())
	return f.Name()
}

func createWatchman(root string) *WatchmanWatcher {
	w := NewWatchman(root)
	if runtime.GOOS == "darwin" {
		w.watchmanPath = "/opt/homebrew/bin/watchman"
	}
	return w
}

func TestWatchStart(t *testing.T) {
	tmp := getTempDir(t)
	defer os.RemoveAll(tmp)

	w := createWatchman(tmp)

	err := w.Start()
	if err != nil {
		t.Errorf("Expected to start watching: %s", err)
	}
	defer w.Stop()
	defer w.Close()

	os.WriteFile(tmp+"/test", []byte("test"), 0644)

	changeset, err := w.GetDiff("")
	if err != nil {
		t.Errorf("Expected to get diff: %s", err)
	}

	if len(changeset.Paths) != 1 {
		t.Errorf("Expected to get one change")
	}

	if changeset.Paths[0] != "test" {
		t.Errorf("Expected to get test file")
	}
}

func TestSubscribe(t *testing.T) {
	tmp := getTempDir(t)
	defer os.RemoveAll(tmp)

	w := createWatchman(tmp)

	err := w.Start()
	if err != nil {
		t.Errorf("Expected to start watching: %s", err)
	}
	defer w.Stop()

	changeset := make(chan ChangeSet)
	go func() {
		for cs := range w.Subscribe(context.TODO(), "") {
			t.Log(cs)
			if len(cs.Paths) == 0 {
				break
			}
			changeset <- *cs
			break
		}

		if err != nil {
			t.Errorf("Expected to subscribe: %s", err)
		}
	}()

	os.WriteFile(tmp+"/test", []byte("test"), 0644)

	cs := <-changeset
	if len(cs.Paths) != 1 {
		t.Errorf("Expected to get one change")
	}

	if cs.Paths[0] != "test" {
		t.Errorf("Expected to get test file")
	}

}
