package rule

import (
	"strings"
	"testing"

	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bazelbuild/bazel-gazelle/language"
	"github.com/bazelbuild/bazel-gazelle/rule"
	"github.com/emirpasic/gods/v2/sets/treeset"
)

// argsWith builds GenerateArgs with the given kind map and, optionally, a single
// existing rule in the package's BUILD file.
func argsWith(kindMap map[string]config.MappedKind, existing *rule.Rule) language.GenerateArgs {
	c := config.New()
	if kindMap != nil {
		c.KindMap = kindMap
	}
	f := rule.EmptyFile("BUILD.bazel", "pkg")
	if existing != nil {
		existing.Insert(f)
	}
	return language.GenerateArgs{Config: c, File: f, Rel: "pkg"}
}

func kinds(names ...string) *treeset.Set[string] {
	return treeset.NewWith(strings.Compare, names...)
}

func TestMapKind(t *testing.T) {
	args := argsWith(map[string]config.MappedKind{
		"orig_kind": {KindName: "mapped_kind"},
	}, nil)

	if got := MapKind(args, "orig_kind"); got != "mapped_kind" {
		t.Errorf("MapKind(orig_kind) = %q, want %q", got, "mapped_kind")
	}
	if got := MapKind(args, "unmapped"); got != "unmapped" {
		t.Errorf("MapKind(unmapped) = %q, want %q (unmapped kinds pass through)", got, "unmapped")
	}
}

func TestGetFileRuleByName(t *testing.T) {
	t.Run("nil file returns nil", func(t *testing.T) {
		if r := GetFileRuleByName(language.GenerateArgs{}, "foo"); r != nil {
			t.Errorf("expected nil for a nil File, got %v", r)
		}
	})

	t.Run("found", func(t *testing.T) {
		args := argsWith(nil, rule.NewRule("go_library", "foo"))
		got := GetFileRuleByName(args, "foo")
		if got == nil || got.Name() != "foo" {
			t.Errorf("expected to find rule %q, got %v", "foo", got)
		}
	})

	t.Run("not found returns nil", func(t *testing.T) {
		args := argsWith(nil, rule.NewRule("go_library", "foo"))
		if got := GetFileRuleByName(args, "bar"); got != nil {
			t.Errorf("expected nil for a missing rule, got %v", got)
		}
	})
}

func TestCheckCollisionErrors(t *testing.T) {
	tests := []struct {
		name           string
		existing       *rule.Rule // nil => no rule of that name exists yet
		expectedKind   string
		kindMap        map[string]config.MappedKind
		generatedKinds []string
		wantErr        bool
	}{
		{
			name:           "no existing rule",
			existing:       nil,
			expectedKind:   "ts_project",
			generatedKinds: []string{"ts_project"},
			wantErr:        false,
		},
		{
			name:           "same kind merge is allowed",
			existing:       rule.NewRule("ts_project", "foo"),
			expectedKind:   "ts_project",
			generatedKinds: nil, // allowed even when the kind is not one we generate
			wantErr:        false,
		},
		{
			name:           "same kind after mapping is allowed",
			existing:       rule.NewRule("my_ts_project", "foo"),
			expectedKind:   "ts_project",
			kindMap:        map[string]config.MappedKind{"ts_project": {KindName: "my_ts_project"}},
			generatedKinds: nil,
			wantErr:        false,
		},
		{
			name:           "different kind owned by this plugin is adaptable",
			existing:       rule.NewRule("ts_project", "foo"),
			expectedKind:   "ts_proto_library",
			generatedKinds: []string{"ts_project"},
			wantErr:        false,
		},
		{
			name:           "different kind not owned by this plugin collides",
			existing:       rule.NewRule("go_library", "foo"),
			expectedKind:   "ts_project",
			generatedKinds: []string{"ts_project"},
			wantErr:        true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			args := argsWith(tc.kindMap, tc.existing)
			err := CheckCollisionErrors("foo", tc.expectedKind, kinds(tc.generatedKinds...), args)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected a collision error, got nil")
				}
				if !strings.Contains(err.Error(), "already exists") {
					t.Errorf("error message %q should explain the name collision", err.Error())
				}
			} else if err != nil {
				t.Errorf("expected no error, got %v", err)
			}
		})
	}
}

// A nil File means nothing has been generated yet, so there can be no collision.
func TestCheckCollisionErrorsNilFile(t *testing.T) {
	args := language.GenerateArgs{Config: config.New()}
	if err := CheckCollisionErrors("foo", "ts_project", kinds("ts_project"), args); err != nil {
		t.Errorf("expected nil error when File is nil, got %v", err)
	}
}

func TestRemoveRule(t *testing.T) {
	t.Run("missing rule is a no-op", func(t *testing.T) {
		args := argsWith(nil, nil)
		var result language.GenerateResult
		RemoveRule(args, "foo", kinds("ts_project"), &result)
		if len(result.Empty) != 0 {
			t.Errorf("expected no empty rules, got %d", len(result.Empty))
		}
	})

	t.Run("rule owned by this plugin is emptied", func(t *testing.T) {
		args := argsWith(nil, rule.NewRule("ts_project", "foo"))
		var result language.GenerateResult
		RemoveRule(args, "foo", kinds("ts_project"), &result)
		if len(result.Empty) != 1 {
			t.Fatalf("expected 1 empty rule, got %d", len(result.Empty))
		}
		if got := result.Empty[0]; got.Kind() != "ts_project" || got.Name() != "foo" {
			t.Errorf("empty rule = %s(%s), want ts_project(foo)", got.Kind(), got.Name())
		}
	})

	t.Run("rule not owned by this plugin is left alone", func(t *testing.T) {
		args := argsWith(nil, rule.NewRule("go_library", "foo"))
		var result language.GenerateResult
		RemoveRule(args, "foo", kinds("ts_project"), &result)
		if len(result.Empty) != 0 {
			t.Errorf("expected a foreign rule to be left alone, got %d empty rules", len(result.Empty))
		}
	})
}
