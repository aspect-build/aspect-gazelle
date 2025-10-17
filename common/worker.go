package gazelle

import (
	"math"
	"sync"

	"github.com/emirpasic/gods/sets/treeset"
)

const (
	// MaxWorkerCount is the maximum number of parallel workers
	MaxWorkerCount = 12
)

// Parallelize an action over a set of string values.
// Returns a channel that emits results as they are produced.
func Parallelize[T any](values *treeset.Set, process func(string) T) chan T {
	// The channel of inputs
	valuesCh := make(chan string)

	// The channel of outputs.
	resultsCh := make(chan T)

	// The number of workers. Don't create more workers than necessary.
	workerCount := int(math.Min(MaxWorkerCount, float64(1+values.Size()/2)))

	// Start the worker goroutines.
	var wg sync.WaitGroup
	for range workerCount {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for value := range valuesCh {
				resultsCh <- process(value)
			}
		}()
	}

	// Send values to the workers.
	go func() {
		valueChannelIt := values.Iterator()
		for valueChannelIt.Next() {
			valuesCh <- valueChannelIt.Value().(string)
		}

		close(valuesCh)
	}()

	// Wait for all workers to finish.
	go func() {
		wg.Wait()
		close(resultsCh)
	}()

	return resultsCh
}
