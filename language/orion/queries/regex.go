package queries

import (
	"regexp"
	"sync"

	"github.com/aspect-build/aspect-gazelle/language/orion/plugin"
	"golang.org/x/sync/errgroup"
)

func runRegexQueries(sourceCode []byte, queries plugin.NamedQueries) (plugin.QueryResults, error) {
	results := make(plugin.QueryResults, len(queries))
	var mu sync.Mutex

	eg := errgroup.Group{}
	eg.SetLimit(10)

	for key, q := range queries {
		// Capture loop variables for goroutine
		key := key
		q := q
		eg.Go(func() error {
			r := runRegexQuery(sourceCode, q.(*plugin.RegexQuery).ExpressionRe())
			mu.Lock()
			results[key] = r
			mu.Unlock()
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}
	return results, nil
}

func runRegexQuery(sourceCode []byte, re *regexp.Regexp) plugin.QueryMatches {
	reMatches := re.FindAllSubmatch(sourceCode, -1)
	if reMatches == nil {
		return nil
	}

	matches := plugin.QueryMatches(nil)

	for _, reMatch := range reMatches {
		captures := make(plugin.QueryCapture)
		for i, name := range re.SubexpNames() {
			if i > 0 && i <= len(reMatch) {
				captures[name] = string(reMatch[i])
			}
		}

		matches = append(matches, plugin.NewQueryMatch(captures, reMatch[0]))
	}

	return matches
}
