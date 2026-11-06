package gazelle

import (
	"testing"

	plugin "github.com/aspect-build/aspect-gazelle/language/orion/plugin"
	"github.com/bazelbuild/bazel-gazelle/label"
)

func sym(repo, pkg, name string) plugin.TargetSymbol {
	return plugin.TargetSymbol{
		Symbol: plugin.Symbol{Id: "s", Provider: "p"},
		Label:  plugin.Label{Repo: repo, Pkg: pkg, Name: name},
	}
}

func TestRankSymbols(t *testing.T) {
	cases := []struct {
		name     string
		symbols  []plugin.TargetSymbol
		from     label.Label
		mainRepo string
		want     []string
	}{
		{
			name:    "deepest ancestor first, non-ancestors last in registration order",
			symbols: []plugin.TargetSymbol{sym("", "x", "x1"), sym("", "", "root"), sym("", "a/b", "ab"), sym("", "y", "y1"), sym("", "a", "a")},
			from:    label.New("", "a/b/c", "t"),
			want:    []string{"ab", "a", "root", "x1", "y1"},
		},
		{
			name:    "equal depth keeps registration order",
			symbols: []plugin.TargetSymbol{sym("", "a", "first"), sym("", "a", "second")},
			from:    label.New("", "a", "t"),
			want:    []string{"first", "second"},
		},
		{
			name:     "empty repo is the main repo",
			symbols:  []plugin.TargetSymbol{sym("main", "", "far"), sym("", "a", "near")},
			from:     label.New("main", "a/b", "t"),
			mainRepo: "main",
			want:     []string{"near", "far"},
		},
		{
			name:     "other repos are not ancestors",
			symbols:  []plugin.TargetSymbol{sym("other", "a", "other"), sym("", "", "root")},
			from:     label.New("", "a", "t"),
			mainRepo: "main",
			want:     []string{"root", "other"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ranked := rankSymbols(tc.symbols, tc.from, tc.mainRepo)
			got := make([]string, len(ranked))
			for i, s := range ranked {
				got[i] = s.Label.Name
			}
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("got %v, want %v", got, tc.want)
				}
			}
		})
	}
}

func TestUniqueLabels(t *testing.T) {
	got := uniqueLabels([]label.Label{
		label.New("", "a", "a"),
		label.New("main", "a", "a"),
		label.New("", "b", "b"),
		label.New("other", "a", "a"),
	}, "main")
	want := []label.Label{label.New("", "a", "a"), label.New("", "b", "b"), label.New("other", "a", "a")}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range got {
		if !got[i].Equal(want[i]) {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestIsPkgAncestor(t *testing.T) {
	cases := []struct {
		anc, pkg string
		want     bool
	}{
		{"", "a", true},
		{"a", "a", true},
		{"a", "a/b", true},
		{"a", "ab", false},
		{"a/b", "a", false},
	}
	for _, tc := range cases {
		if got := isPkgAncestor(tc.anc, tc.pkg); got != tc.want {
			t.Errorf("isPkgAncestor(%q, %q) = %v, want %v", tc.anc, tc.pkg, got, tc.want)
		}
	}
}

func TestSymbolCycleError(t *testing.T) {
	ping := plugin.TargetSymbol{Symbol: plugin.Symbol{Id: "ping", Provider: "p"}, Label: plugin.Label{Pkg: "a"}}
	pong := plugin.TargetSymbol{Symbol: plugin.Symbol{Id: "pong", Provider: "p"}, Label: plugin.Label{Pkg: "b"}}
	err := &symbolCycleError{chain: []plugin.TargetSymbol{ping, pong, ping}}
	want := `add_symbol() Imports form a cycle: "ping" (provider "p") in //a -> "pong" (provider "p") in //b -> "ping" (provider "p") in //a`
	if err.Error() != want {
		t.Errorf("got %q, want %q", err.Error(), want)
	}
}
