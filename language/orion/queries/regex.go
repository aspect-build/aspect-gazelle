package queries

import (
	"regexp"

	"github.com/aspect-build/aspect-gazelle/language/orion/plugin"
)

func runRegexQueries(sourceCode []byte, queries plugin.NamedQueries) (plugin.QueryResults, error) {
	results := make(plugin.QueryResults, len(queries))
	for key, q := range queries {
		results[key] = runRegexQuery(sourceCode, q.(*plugin.RegexQuery).ExpressionRe())
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
