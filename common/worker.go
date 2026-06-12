package common

import (
	"runtime"
	"sync"
	"sync/atomic"
)

// MaxWorkerCount is the maximum number of parallel workers
var MaxWorkerCount = runtime.GOMAXPROCS(0)

// A process-wide pool of persistent worker goroutines shared by all parallel
// work. A single long-lived pool avoids the per-call-site cost of spawning
// and parking goroutines (and the OS-thread wake churn that comes with it)
// when many small batches are processed, such as gazelle generating rules
// directory-by-directory.
//
// Two priority levels keep walk-critical work responsive: Parallelize tasks
// (a visitor is blocked waiting on them) run before background Submit tasks
// (speculative prefetching).
var (
	poolMu      sync.Mutex
	poolCond    = sync.NewCond(&poolMu)
	poolHigh    taskQueue
	poolLow     taskQueue
	poolStarted bool
)

// taskQueue is a FIFO queue that releases task references as they are
// popped and reuses the backing array once drained.
type taskQueue struct {
	tasks []func()
	head  int
}

func (q *taskQueue) push(t func()) {
	q.tasks = append(q.tasks, t)
}

func (q *taskQueue) pop() func() {
	if q.head == len(q.tasks) {
		return nil
	}
	t := q.tasks[q.head]
	q.tasks[q.head] = nil
	q.head++
	if q.head == len(q.tasks) {
		q.tasks = q.tasks[:0]
		q.head = 0
	}
	return t
}

func (q *taskQueue) empty() bool {
	return q.head == len(q.tasks)
}

// Submit schedules background work on the shared worker pool. Tasks run in
// FIFO order, after any pending Parallelize work. Submit never blocks; the
// queue is unbounded.
func Submit(task func()) {
	enqueue(task, false)
}

func enqueue(task func(), high bool) {
	poolMu.Lock()
	if !poolStarted {
		poolStarted = true
		for range MaxWorkerCount {
			go poolWorker()
		}
	}
	if high {
		poolHigh.push(task)
	} else {
		poolLow.push(task)
	}
	poolMu.Unlock()
	poolCond.Signal()
}

func poolWorker() {
	for {
		poolMu.Lock()
		for poolHigh.empty() && poolLow.empty() {
			poolCond.Wait()
		}
		task := poolHigh.pop()
		if task == nil {
			task = poolLow.pop()
		}
		poolMu.Unlock()

		task()
	}
}

// Parallelize an action over a set of string values on the shared worker pool.
// Returns a channel that emits results as they are produced.
func Parallelize[T any](values []string, process func(string) T) chan T {
	// Buffer all results so pool workers never block on a slow consumer.
	resultsCh := make(chan T, len(values))
	if len(values) == 0 {
		close(resultsCh)
		return resultsCh
	}

	var remaining atomic.Int64
	remaining.Store(int64(len(values)))

	for _, value := range values {
		enqueue(func() {
			resultsCh <- process(value)
			if remaining.Add(-1) == 0 {
				close(resultsCh)
			}
		}, true)
	}

	return resultsCh
}
