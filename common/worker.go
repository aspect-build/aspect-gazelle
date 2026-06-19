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
