package common

import (
	"sync"
	"sync/atomic"
)

const (
	// MaxWorkerCount is the maximum number of parallel workers
	MaxWorkerCount = 12
)

// workerPool is a shared, process-wide pool of MaxWorkerCount worker goroutines
// created once and reused across all Parallelize calls.
var workerPool = sync.OnceValue(func() chan func() {
	tasks := make(chan func(), MaxWorkerCount)
	for range MaxWorkerCount {
		go func() {
			for task := range tasks {
				task()
			}
		}()
	}
	return tasks
})

// Parallelize an action over a set of string values.
// Returns a channel that emits results as they are produced.
func Parallelize[T any](values []string, process func(string) T) chan T {
	// Buffered to hold every result so a shared pool worker never blocks on its
	// send (a consumer that stops reading early must not wedge a pooled worker).
	resultsCh := make(chan T, len(values))
	if len(values) == 0 {
		close(resultsCh)
		return resultsCh
	}

	// Submit the work to the shared worker pool. The last task to finish closes
	// the channel; each decrement runs after that task's send, so reaching zero
	// means every result has been buffered.
	tasks := workerPool()
	var remaining atomic.Int64
	remaining.Store(int64(len(values)))
	go func() {
		for i := range values {
			tasks <- func() {
				defer func() {
					if remaining.Add(-1) == 0 {
						close(resultsCh)
					}
				}()
				resultsCh <- process(values[i])
			}
		}
	}()

	return resultsCh
}

// WorkerGroup runs a set of fallible tasks on the shared worker pool. It mirrors
// the subset of golang.org/x/sync/errgroup used here — Go submits a task, Wait
// blocks until every submitted task has finished and returns the first non-nil
// error — but it does not start a goroutine per task: tasks run on the shared
// pool, so total concurrency is bounded by the pool rather than per group (no
// SetLimit needed). The zero value is ready to use.
//
// As with Parallelize, a pooled task must not call Go/Wait on a group whose
// tasks contend for the pool, or it can starve it.
type WorkerGroup struct {
	wg      sync.WaitGroup
	errOnce sync.Once
	err     error
}

// Go submits fn to run on the shared worker pool. It blocks only when the pool's
// intake is full (back-pressure); it never spawns its own goroutine.
func (g *WorkerGroup) Go(fn func() error) {
	g.wg.Add(1)
	workerPool() <- func() {
		defer g.wg.Done()
		if err := fn(); err != nil {
			g.errOnce.Do(func() { g.err = err })
		}
	}
}

// Wait blocks until all tasks submitted via Go have finished and returns the
// first error any of them returned (nil if none did).
func (g *WorkerGroup) Wait() error {
	g.wg.Wait()
	return g.err
}
