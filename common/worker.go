package common

import (
	"math"
	"sync"
)

const (
	// MaxWorkerCount is the maximum number of parallel workers
	MaxWorkerCount = 12
)

// Parallelize an action over a set of string values, returning the results in
// input order. Workers are created per call and coordinated with a WaitGroup;
// each writes into its own slice slot, so no channels are involved.
//
// The values are split into contiguous chunks, one worker per chunk (up to
// MaxWorkerCount). This is a static partition — simple and channel-free, but it
// assumes roughly even per-value cost; a single very expensive value can leave
// its worker straggling.
func Parallelize[T any](values []string, process func(string) T) []T {
	results := make([]T, len(values))
	if len(values) == 0 {
		return results
	}

	// Don't create more workers than necessary.
	workerCount := int(math.Min(MaxWorkerCount, float64(1+len(values)/2)))
	chunk := (len(values) + workerCount - 1) / workerCount

	var wg sync.WaitGroup
	for start := 0; start < len(values); start += chunk {
		end := min(start+chunk, len(values))
		wg.Add(1)
		go func(start, end int) {
			defer wg.Done()
			for i := start; i < end; i++ {
				results[i] = process(values[i])
			}
		}(start, end)
	}

	wg.Wait()
	return results
}
