package watchman

import (
	"os"
	"testing"
)

func TestLoadOrStoreFile(t *testing.T) {
	tmp := getTempDir(t)
	defer os.RemoveAll(tmp)

	if err := os.WriteFile(tmp+"/test", []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to write test file: %s", err)
	}

	w := createWatchman(tmp)

	err := w.Start()
	if err != nil {
		t.Errorf("Expected to start watching: %s", err)
	}
	defer w.Stop()
	defer w.Close()

	c := newWatchmanCache(w, getTempFile(t))

	computes := 0
	compute := func(path string, content []byte) (any, error) {
		computes++
		return string(content), nil
	}

	res, cached, err := c.LoadOrStoreFile(tmp, "test", "op1", compute)
	if err != nil {
		t.Errorf("Expected to load or store file: %s", err)
	}
	if cached {
		t.Errorf("Expected not cached on first load")
	}
	if res.(string) != "test" {
		t.Errorf("Expected content 'test', got '%s'", res.(string))
	}

	res2, cached2, err2 := c.LoadOrStoreFile(tmp, "test", "op1", compute)
	if err2 != nil {
		t.Errorf("Expected to load or store file: %s", err2)
	}
	if !cached2 {
		t.Errorf("Expected cached on second load")
	}
	if res2.(string) != res {
		t.Errorf("Expected content '%s', got '%s'", res, res2)
	}

	if computes != 1 {
		t.Errorf("Expected 1 compute, got %d", computes)
	}
}

func TestLoadOrStoreFileSymlink(t *testing.T) {
	tmp := getTempDir(t)
	defer os.RemoveAll(tmp)

	os.WriteFile(tmp+"/test", []byte("test"), 0644)
	os.Symlink("test", tmp+"/test-symlink")

	w := createWatchman(tmp)

	err := w.Start()
	if err != nil {
		t.Errorf("Expected to start watching: %s", err)
	}
	defer w.Stop()
	defer w.Close()

	c := newWatchmanCache(w, getTempFile(t))

	computes := 0
	compute := func(path string, content []byte) (any, error) {
		computes++
		return string(content), nil
	}

	res, _, _ := c.LoadOrStoreFile(tmp, "test", "op1", compute)
	res2, _, err := c.LoadOrStoreFile(tmp, "test-symlink", "op1", compute)

	if err != nil {
		t.Errorf("Expected to load or store file: %s", err)
	}

	if res2 != res {
		t.Errorf("Expected content '%s', got '%s'", res, res2)
	}

	if computes != 1 {
		t.Errorf("Expected 1 compute, got %d", computes)
	}
}
