package gazelle

import (
	"strings"
	"testing"

	"github.com/bazelbuild/bazel-gazelle/label"
	bzl "github.com/bazelbuild/buildtools/build"
)

func TestLabelComparator(t *testing.T) {
	tests := []struct {
		name     string
		a        label.Label
		b        label.Label
		expected int
	}{
		{
			name:     "relative label comes before absolute",
			a:        label.Label{Relative: true, Name: "foo"},
			b:        label.Label{Relative: false, Name: "foo"},
			expected: -1,
		},
		{
			name:     "absolute label comes after relative",
			a:        label.Label{Relative: false, Name: "foo"},
			b:        label.Label{Relative: true, Name: "foo"},
			expected: 1,
		},
		{
			name:     "same labels are equal",
			a:        label.Label{Repo: "repo", Pkg: "pkg", Name: "name"},
			b:        label.Label{Repo: "repo", Pkg: "pkg", Name: "name"},
			expected: 0,
		},
		{
			name:     "different repos - a < b",
			a:        label.Label{Repo: "aaa", Pkg: "pkg", Name: "name"},
			b:        label.Label{Repo: "bbb", Pkg: "pkg", Name: "name"},
			expected: -1,
		},
		{
			name:     "different repos - a > b",
			a:        label.Label{Repo: "bbb", Pkg: "pkg", Name: "name"},
			b:        label.Label{Repo: "aaa", Pkg: "pkg", Name: "name"},
			expected: 1,
		},
		{
			name:     "same repo, different pkg - a < b",
			a:        label.Label{Repo: "repo", Pkg: "aaa", Name: "name"},
			b:        label.Label{Repo: "repo", Pkg: "bbb", Name: "name"},
			expected: -1,
		},
		{
			name:     "same repo, different pkg - a > b",
			a:        label.Label{Repo: "repo", Pkg: "bbb", Name: "name"},
			b:        label.Label{Repo: "repo", Pkg: "aaa", Name: "name"},
			expected: 1,
		},
		{
			name:     "same repo and pkg, different name - a < b",
			a:        label.Label{Repo: "repo", Pkg: "pkg", Name: "aaa"},
			b:        label.Label{Repo: "repo", Pkg: "pkg", Name: "bbb"},
			expected: -1,
		},
		{
			name:     "same repo and pkg, different name - a > b",
			a:        label.Label{Repo: "repo", Pkg: "pkg", Name: "bbb"},
			b:        label.Label{Repo: "repo", Pkg: "pkg", Name: "aaa"},
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := LabelComparator(tt.a, tt.b)
			if tt.expected < 0 {
				if result >= 0 {
					t.Errorf("expected a < b, got %d", result)
				}
			} else if tt.expected > 0 {
				if result <= 0 {
					t.Errorf("expected a > b, got %d", result)
				}
			} else {
				if result != 0 {
					t.Errorf("expected a == b, got %d", result)
				}
			}
		})
	}
}

func TestLabelSet_Add(t *testing.T) {
	t.Run("add single label", func(t *testing.T) {
		from := label.Label{Repo: "repo", Pkg: "pkg", Name: "target"}
		ls := NewLabelSet(from)

		l := label.Label{Repo: "repo", Pkg: "other", Name: "dep"}
		ls.Add(&l)

		if ls.Size() != 1 {
			t.Errorf("Size: got %d, want 1", ls.Size())
		}
		if ls.Empty() {
			t.Error("expected Empty() to be false")
		}
	})

	t.Run("add multiple labels", func(t *testing.T) {
		from := label.Label{Repo: "repo", Pkg: "pkg", Name: "target"}
		ls := NewLabelSet(from)

		l1 := label.Label{Repo: "repo", Pkg: "pkg1", Name: "dep1"}
		l2 := label.Label{Repo: "repo", Pkg: "pkg2", Name: "dep2"}
		l3 := label.Label{Repo: "repo", Pkg: "pkg3", Name: "dep3"}

		ls.Add(&l1)
		ls.Add(&l2)
		ls.Add(&l3)

		if ls.Size() != 3 {
			t.Errorf("Size: got %d, want 3", ls.Size())
		}
	})

	t.Run("add duplicate labels", func(t *testing.T) {
		from := label.Label{Repo: "repo", Pkg: "pkg", Name: "target"}
		ls := NewLabelSet(from)

		l := label.Label{Repo: "repo", Pkg: "other", Name: "dep"}
		ls.Add(&l)
		ls.Add(&l)

		if ls.Size() != 1 {
			t.Errorf("duplicate labels should not increase size: got %d, want 1", ls.Size())
		}
	})

	t.Run("ignore self-reference", func(t *testing.T) {
		from := label.Label{Repo: "repo", Pkg: "pkg", Name: "target"}
		ls := NewLabelSet(from)

		ls.Add(&from)

		if ls.Size() != 0 {
			t.Errorf("self-reference should be ignored: got size %d, want 0", ls.Size())
		}
		if !ls.Empty() {
			t.Error("expected Empty() to be true")
		}
	})

	t.Run("converts to relative label", func(t *testing.T) {
		from := label.Label{Repo: "repo", Pkg: "pkg", Name: "target"}
		ls := NewLabelSet(from)

		// Add an absolute label in the same package
		l := label.Label{Repo: "repo", Pkg: "pkg", Name: "other"}
		ls.Add(&l)

		// Check that it was stored as a relative label
		count := 0
		for lbl := range ls.Labels() {
			count++
			if !lbl.Relative {
				t.Error("label should be relative")
			}
			if lbl.Name != "other" {
				t.Errorf("Name: got %s, want other", lbl.Name)
			}
		}
		if count != 1 {
			t.Errorf("count: got %d, want 1", count)
		}
	})
}

func TestLabelSet_Size(t *testing.T) {
	from := label.Label{Repo: "repo", Pkg: "pkg", Name: "target"}
	ls := NewLabelSet(from)

	if ls.Size() != 0 {
		t.Errorf("Size: got %d, want 0", ls.Size())
	}

	l1 := label.Label{Repo: "repo", Pkg: "pkg1", Name: "dep1"}
	ls.Add(&l1)
	if ls.Size() != 1 {
		t.Errorf("Size: got %d, want 1", ls.Size())
	}

	l2 := label.Label{Repo: "repo", Pkg: "pkg2", Name: "dep2"}
	ls.Add(&l2)
	if ls.Size() != 2 {
		t.Errorf("Size: got %d, want 2", ls.Size())
	}
}

func TestLabelSet_Labels(t *testing.T) {
	t.Run("empty set", func(t *testing.T) {
		from := label.Label{Repo: "repo", Pkg: "pkg", Name: "target"}
		ls := NewLabelSet(from)

		count := 0
		for range ls.Labels() {
			count++
		}
		if count != 0 {
			t.Errorf("count: got %d, want 0", count)
		}
	})

	t.Run("iterate over labels", func(t *testing.T) {
		from := label.Label{Repo: "repo", Pkg: "pkg", Name: "target"}
		ls := NewLabelSet(from)

		l1 := label.Label{Repo: "repo", Pkg: "pkg1", Name: "dep1"}
		l2 := label.Label{Repo: "repo", Pkg: "pkg2", Name: "dep2"}
		l3 := label.Label{Repo: "repo", Pkg: "pkg3", Name: "dep3"}

		ls.Add(&l1)
		ls.Add(&l2)
		ls.Add(&l3)

		labels := make([]label.Label, 0)
		for lbl := range ls.Labels() {
			labels = append(labels, lbl)
		}

		if len(labels) != 3 {
			t.Errorf("len(labels): got %d, want 3", len(labels))
		}
	})

	t.Run("labels are sorted", func(t *testing.T) {
		from := label.Label{Repo: "repo", Pkg: "pkg", Name: "target"}
		ls := NewLabelSet(from)

		// Add labels in non-alphabetical order
		l3 := label.Label{Repo: "repo", Pkg: "zzz", Name: "dep3"}
		l1 := label.Label{Repo: "repo", Pkg: "aaa", Name: "dep1"}
		l2 := label.Label{Repo: "repo", Pkg: "mmm", Name: "dep2"}

		ls.Add(&l3)
		ls.Add(&l1)
		ls.Add(&l2)

		labels := make([]label.Label, 0)
		for lbl := range ls.Labels() {
			labels = append(labels, lbl)
		}

		// Should be sorted by comparator (relative first, then by repo/pkg/name)
		if len(labels) != 3 {
			t.Errorf("len(labels): got %d, want 3", len(labels))
		}
		// Check they're in sorted order by package
		if labels[0].Pkg != "aaa" {
			t.Errorf("labels[0].Pkg: got %s, want aaa", labels[0].Pkg)
		}
		if labels[1].Pkg != "mmm" {
			t.Errorf("labels[1].Pkg: got %s, want mmm", labels[1].Pkg)
		}
		if labels[2].Pkg != "zzz" {
			t.Errorf("labels[2].Pkg: got %s, want zzz", labels[2].Pkg)
		}
	})
}

func TestLabelSet_BzlExpr(t *testing.T) {
	t.Run("empty set", func(t *testing.T) {
		from := label.Label{Repo: "repo", Pkg: "pkg", Name: "target"}
		ls := NewLabelSet(from)

		expr := ls.BzlExpr()
		if expr == nil {
			t.Fatal("expected non-nil expr")
		}

		listExpr, ok := expr.(*bzl.ListExpr)
		if !ok {
			t.Fatal("should be a ListExpr")
		}
		if len(listExpr.List) != 0 {
			t.Errorf("len(listExpr.List): got %d, want 0", len(listExpr.List))
		}
	})

	t.Run("set with labels", func(t *testing.T) {
		from := label.Label{Repo: "repo", Pkg: "pkg", Name: "target"}
		ls := NewLabelSet(from)

		l1 := label.Label{Repo: "repo", Pkg: "pkg1", Name: "dep1"}
		l2 := label.Label{Repo: "repo", Pkg: "pkg2", Name: "dep2"}

		ls.Add(&l1)
		ls.Add(&l2)

		expr := ls.BzlExpr()
		if expr == nil {
			t.Fatal("expected non-nil expr")
		}

		listExpr, ok := expr.(*bzl.ListExpr)
		if !ok {
			t.Fatal("should be a ListExpr")
		}
		if len(listExpr.List) != 2 {
			t.Errorf("len(listExpr.List): got %d, want 2", len(listExpr.List))
		}

		// Each element should be a string expression
		for _, e := range listExpr.List {
			_, ok := e.(*bzl.StringExpr)
			if !ok {
				t.Error("each element should be a StringExpr")
			}
		}
	})

	t.Run("sorted order in BzlExpr", func(t *testing.T) {
		from := label.Label{Repo: "repo", Pkg: "pkg", Name: "target"}
		ls := NewLabelSet(from)

		// Add in reverse order
		l3 := label.Label{Repo: "repo", Pkg: "zzz", Name: "dep3"}
		l1 := label.Label{Repo: "repo", Pkg: "aaa", Name: "dep1"}
		l2 := label.Label{Repo: "repo", Pkg: "mmm", Name: "dep2"}

		ls.Add(&l3)
		ls.Add(&l1)
		ls.Add(&l2)

		expr := ls.BzlExpr()
		listExpr, ok := expr.(*bzl.ListExpr)
		if !ok {
			t.Fatal("should be a ListExpr")
		}
		if len(listExpr.List) != 3 {
			t.Errorf("len(listExpr.List): got %d, want 3", len(listExpr.List))
		}

		// Verify they're sorted in the output
		str0, ok := listExpr.List[0].(*bzl.StringExpr)
		if !ok {
			t.Fatal("element 0 should be a StringExpr")
		}
		if !strings.Contains(str0.Value, "aaa") {
			t.Errorf("str0.Value should contain 'aaa', got %s", str0.Value)
		}

		str1, ok := listExpr.List[1].(*bzl.StringExpr)
		if !ok {
			t.Fatal("element 1 should be a StringExpr")
		}
		if !strings.Contains(str1.Value, "mmm") {
			t.Errorf("str1.Value should contain 'mmm', got %s", str1.Value)
		}

		str2, ok := listExpr.List[2].(*bzl.StringExpr)
		if !ok {
			t.Fatal("element 2 should be a StringExpr")
		}
		if !strings.Contains(str2.Value, "zzz") {
			t.Errorf("str2.Value should contain 'zzz', got %s", str2.Value)
		}
	})
}

func TestLabelSet_IntegrationScenarios(t *testing.T) {
	t.Run("mixed labels from different packages and repos", func(t *testing.T) {
		from := label.Label{Repo: "myrepo", Pkg: "mypackage", Name: "mytarget"}
		ls := NewLabelSet(from)

		// Same package
		samePkg := label.Label{Repo: "myrepo", Pkg: "mypackage", Name: "dep1"}
		// Different package, same repo
		diffPkg := label.Label{Repo: "myrepo", Pkg: "other", Name: "dep2"}
		// Different repo
		diffRepo := label.Label{Repo: "otherrepo", Pkg: "pkg", Name: "dep3"}

		ls.Add(&samePkg)
		ls.Add(&diffPkg)
		ls.Add(&diffRepo)

		if ls.Size() != 3 {
			t.Errorf("Size: got %d, want 3", ls.Size())
		}

		// Collect labels
		labels := make([]label.Label, 0)
		for lbl := range ls.Labels() {
			labels = append(labels, lbl)
		}

		// Should have 3 labels stored (all converted via Rel())
		if len(labels) != 3 {
			t.Errorf("len(labels): got %d, want 3", len(labels))
		}
	})

	t.Run("deduplication across additions", func(t *testing.T) {
		from := label.Label{Repo: "repo", Pkg: "pkg", Name: "target"}
		ls := NewLabelSet(from)

		l1 := label.Label{Repo: "repo", Pkg: "other", Name: "dep"}
		l2 := label.Label{Repo: "repo", Pkg: "other", Name: "dep"}

		ls.Add(&l1)
		ls.Add(&l1)
		if ls.Size() != 1 {
			t.Errorf("Size after first add: got %d, want 1", ls.Size())
		}

		ls.Add(&l2)
		ls.Add(&l2)
		if ls.Size() != 1 {
			t.Errorf("adding duplicate should not increase size: got %d, want 1", ls.Size())
		}
	})
}
