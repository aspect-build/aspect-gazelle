package common

import (
	"slices"
	"strconv"
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

			// Results are returned in input order (index-aligned).
			results := Parallelize(values, func(v string) string { return v + "!" })

			if !slices.Equal(results, expected) {
				t.Errorf("Parallelize over %d values returned %v, want %v", size, results, expected)
			}
		})
	}
}
