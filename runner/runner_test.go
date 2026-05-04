package runner

import "testing"

func TestWalkCacheEntryInvalidated(t *testing.T) {
	cases := []struct {
		name string
		rel  string
		dirs []string
		want bool
	}{
		{
			name: "no invalidated dirs",
			rel:  "pkg",
			dirs: nil,
			want: false,
		},
		{
			name: "exact match",
			rel:  "pkg/sub",
			dirs: []string{"pkg/sub"},
			want: true,
		},
		{
			name: "descendant of invalidated dir",
			rel:  "pkg/sub/leaf",
			dirs: []string{"pkg/sub"},
			want: true,
		},
		{
			name: "unrelated dir",
			rel:  "other",
			dirs: []string{"pkg"},
			want: false,
		},
		// Guards against the "pkg" prefix accidentally matching "pkg-foo": the '/' check
		// in the predicate is the reason this returns false.
		{
			name: "prefix-but-not-descendant",
			rel:  "pkg-foo",
			dirs: []string{"pkg"},
			want: false,
		},
		// Root-change cases: path.Dir returns "." for files at the workspace root,
		// but the gazelle walk cache keys the root entry as "". Both must invalidate
		// every entry, matching invalidateWalkCache in pkg/watchman/cache.go.
		{
			name: "root dot invalidates root entry",
			rel:  "",
			dirs: []string{"."},
			want: true,
		},
		{
			name: "root dot invalidates nested entry",
			rel:  "pkg/sub",
			dirs: []string{"."},
			want: true,
		},
		{
			name: "root empty invalidates root entry",
			rel:  "",
			dirs: []string{""},
			want: true,
		},
		{
			name: "root empty invalidates nested entry",
			rel:  "pkg/sub",
			dirs: []string{""},
			want: true,
		},
		{
			name: "any matching dir wins",
			rel:  "pkg/sub",
			dirs: []string{"other", "pkg/sub", "more"},
			want: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := walkCacheEntryInvalidated(tc.rel, tc.dirs)
			if got != tc.want {
				t.Errorf("walkCacheEntryInvalidated(%q, %v) = %v, want %v", tc.rel, tc.dirs, got, tc.want)
			}
		})
	}
}
