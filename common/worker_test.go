package common

import (
	"errors"
	"slices"
	"strconv"
	"sync/atomic"
	"testing"
)

func TestParallelize(t *testing.T) {
	for _, size := range []int{0, 1, 2, 5, 100} {
		t.Run(strconv.Itoa(size), func(t *testing.T) {
			values := make([]string, size)
			expected := make([]string, size)
			for i := range values {
				values[i] = strconv.Itoa(i)
				expected[i] = strconv.Itoa(i) + "!"
			}

			results := make([]string, 0, size)
			for r := range Parallelize(values, func(v string) string { return v + "!" }) {
				results = append(results, r)
			}

			slices.Sort(expected)
			slices.Sort(results)
			if !slices.Equal(results, expected) {
				t.Errorf("Parallelize over %d values returned %v, want %v", size, results, expected)
			}
		})
	}
}

// A consumer that abandons the channel early must not wedge a shared pool
// worker; the buffered result channel lets workers finish regardless. A leaked
// worker would eventually starve the pool and hang this test.
func TestParallelizeAbandonedConsumer(t *testing.T) {
	for range 100 {
		<-Parallelize([]string{"a", "b", "c"}, func(v string) string { return v })
	}
	for r := range Parallelize([]string{"x"}, func(v string) string { return v }) {
		if r != "x" {
			t.Errorf("pool unusable after abandoned consumers: got %q", r)
		}
	}
}

func TestWorkerGroup(t *testing.T) {
	// Every submitted task runs, and Wait returns nil when none fail.
	var g WorkerGroup
	var count atomic.Int64
	for range 50 {
		g.Go(func() error { count.Add(1); return nil })
	}
	if err := g.Wait(); err != nil || count.Load() != 50 {
		t.Fatalf("Wait() = %v, ran %d tasks; want nil and 50", err, count.Load())
	}

	// Wait surfaces a task's error.
	var g2 WorkerGroup
	sentinel := errors.New("boom")
	g2.Go(func() error { return nil })
	g2.Go(func() error { return sentinel })
	g2.Go(func() error { return nil })
	if err := g2.Wait(); err != sentinel {
		t.Errorf("Wait() = %v, want %v", err, sentinel)
	}
}
