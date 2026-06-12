package prefetch

import (
	"flag"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bazelbuild/bazel-gazelle/config"
)

func testCoordinator(t *testing.T) (*Coordinator, *config.Config) {
	t.Helper()
	c := config.New()
	pc := NewConfigurer(true)
	if err := pc.CheckFlags(&flag.FlagSet{}, c); err != nil {
		t.Fatal(err)
	}
	co := Get(c)
	if co == nil {
		t.Fatal("Get returned nil after CheckFlags")
	}
	return co, c
}

func TestNilCoordinator(t *testing.T) {
	var co *Coordinator

	co.Prefetch("k", func() (any, error) { return nil, nil })
	co.Register(func(rel string, regularFiles []string, pkgRel string) {})

	v, err := co.LoadOrCompute("k", func() (any, error) { return 42, nil })
	if err != nil || v != 42 {
		t.Errorf("nil LoadOrCompute = (%v, %v), want (42, nil)", v, err)
	}
}

func TestGetUnconfigured(t *testing.T) {
	if co := Get(config.New()); co != nil {
		t.Errorf("Get on unconfigured config = %v, want nil", co)
	}
}

func TestPrefetchThenLoad(t *testing.T) {
	co, _ := testCoordinator(t)

	var computes atomic.Int32
	compute := func() (any, error) {
		computes.Add(1)
		return "value", nil
	}

	co.Prefetch("k", compute)
	co.Prefetch("k", compute) // duplicate is a no-op

	v, err := co.LoadOrCompute("k", compute)
	if err != nil || v != "value" {
		t.Errorf("LoadOrCompute = (%v, %v), want (value, nil)", v, err)
	}
	if got := computes.Load(); got != 1 {
		t.Errorf("compute ran %d times, want 1", got)
	}
}

func TestLoadWithoutPrefetch(t *testing.T) {
	co, _ := testCoordinator(t)

	v, err := co.LoadOrCompute("never-prefetched", func() (any, error) { return 7, nil })
	if err != nil || v != 7 {
		t.Errorf("LoadOrCompute = (%v, %v), want (7, nil)", v, err)
	}
}

func TestLoadAwaitsInflight(t *testing.T) {
	co, _ := testCoordinator(t)

	started := make(chan struct{})
	release := make(chan struct{})
	co.Prefetch("k", func() (any, error) {
		close(started)
		<-release
		return "slow", nil
	})

	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("prefetch task never started")
	}

	loaded := make(chan any, 1)
	go func() {
		v, _ := co.LoadOrCompute("k", func() (any, error) { return "stolen", nil })
		loaded <- v
	}()

	select {
	case v := <-loaded:
		t.Fatalf("LoadOrCompute returned %v before in-flight compute finished", v)
	case <-time.After(50 * time.Millisecond):
	}

	close(release)
	select {
	case v := <-loaded:
		if v != "slow" {
			t.Errorf("LoadOrCompute = %v, want the in-flight result \"slow\"", v)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("LoadOrCompute never returned")
	}
}

func TestDirLoadedFanout(t *testing.T) {
	co, _ := testCoordinator(t)

	// Delivery is asynchronous (dispatched on the shared pool); collect the
	// expected number of calls through a channel.
	calls := make(chan string, 4)
	co.Register(func(rel string, regularFiles []string, pkgRel string) {
		if rel != "some/dir" {
			return // the parent dir announcements
		}
		if len(regularFiles) != 2 || pkgRel != "some" {
			t.Errorf("prefetcher got (%q, %v, %q)", rel, regularFiles, pkgRel)
		}
		calls <- "live"
	})

	// Parent-first, as the traversal guarantees: the root and "some" are
	// packages; "some/dir" has no build file and flattens into "some".
	co.dirLoaded("", nil, "")
	co.dirLoaded("some", nil, "some")
	co.dirLoaded("some/dir", []string{"a.ts", "b.ts"}, "some")

	// A prefetcher registered after the announcement gets the log replayed.
	co.Register(func(rel string, regularFiles []string, pkgRel string) {
		if rel != "some/dir" {
			return // replay of the parent dirs
		}
		if len(regularFiles) != 2 || pkgRel != "some" {
			t.Errorf("late prefetcher got (%q, %v, %q)", rel, regularFiles, pkgRel)
		}
		calls <- "replayed"
	})

	got := map[string]int{}
	for range 2 {
		select {
		case kind := <-calls:
			got[kind]++
		case <-time.After(5 * time.Second):
			t.Fatalf("timed out waiting for prefetcher calls, got %v", got)
		}
	}
	if got["live"] != 1 || got["replayed"] != 1 {
		t.Errorf("prefetcher calls = %v, want one live and one replayed", got)
	}
}
