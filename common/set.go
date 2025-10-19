package gazelle

import (
	"iter"
	"strings"

	BazelLog "github.com/aspect-build/aspect-gazelle/common/logger"
	"github.com/bazelbuild/bazel-gazelle/label"
	"github.com/bazelbuild/bazel-gazelle/rule"
	bzl "github.com/bazelbuild/buildtools/build"
	"github.com/emirpasic/gods/sets/treeset"
)

// A basic set of label.Labels with logging of set modifications.
type LabelSet struct {
	from   label.Label
	labels *treeset.Set
}

func LabelComparator(a, b interface{}) int {
	al := a.(label.Label)
	bl := b.(label.Label)

	if al.Relative && !bl.Relative {
		return -1
	} else if !al.Relative && bl.Relative {
		return +1
	}

	c := strings.Compare(al.Repo, bl.Repo)
	if c != 0 {
		return c
	}

	c = strings.Compare(al.Pkg, bl.Pkg)
	if c != 0 {
		return c
	}

	return strings.Compare(al.Name, bl.Name)
}

func NewLabelSet(from label.Label) *LabelSet {
	return &LabelSet{
		from:   from,
		labels: treeset.NewWith(LabelComparator),
	}
}

func (s *LabelSet) Add(l *label.Label) {
	if s.from.Equal(*l) {
		BazelLog.Debugf("ignore %q dependency on self", s.from.String())
		return
	}

	// Convert to a relative label for simpler labels in BUILD files
	relL := l.Rel(s.from.Repo, s.from.Pkg)

	s.labels.Add(relL)
}

func (s *LabelSet) Size() int {
	return s.labels.Size()
}

func (s *LabelSet) Empty() bool {
	return s.labels.Empty()
}

func (s *LabelSet) Labels() iter.Seq[label.Label] {
	return func(yield func(label.Label) bool) {
		for it := s.labels.Iterator(); it.Next(); {
			if !yield(it.Value().(label.Label)) {
				return
			}
		}
	}
}

var _ rule.BzlExprValue = (*LabelSet)(nil)

func (s *LabelSet) BzlExpr() bzl.Expr {
	le := bzl.ListExpr{
		List: make([]bzl.Expr, 0, s.labels.Size()),
	}

	for it := s.labels.Iterator(); it.Next(); {
		le.List = append(le.List, it.Value().(label.Label).BzlExpr())
	}

	return &le
}
