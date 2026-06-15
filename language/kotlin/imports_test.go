package gazelle

import (
	"reflect"
	"testing"

	"github.com/bazelbuild/bazel-gazelle/resolve"
)

// TestJavaFullyQualifiedName verifies that ImportStatement parses fully qualified
// Java/Kotlin package names into correct segment parts and formats back to string.
func TestJavaFullyQualifiedName(t *testing.T) {
	tests := []struct {
		imp  string
		want []string
	}{
		{"com.example.foo", []string{"com", "example", "foo"}},
		{"com", []string{"com"}},
		{"", []string{""}},
	}

	for _, tt := range tests {
		t.Run(tt.imp, func(t *testing.T) {
			is := &ImportStatement{
				ImportSpec: resolve.ImportSpec{
					Imp: tt.imp,
				},
			}
			fqn := is.packageFullyQualifiedName()
			if !reflect.DeepEqual(fqn.parts, tt.want) {
				t.Errorf("packageFullyQualifiedName() = %v, want %v", fqn.parts, tt.want)
			}
			if fqn.String() != tt.imp {
				t.Errorf("String() = %q, want %q", fqn.String(), tt.imp)
			}
		})
	}
}

// TestJavaFullyQualifiedNameParent verifies that Parent() correctly returns the
// parent package hierarchy or nil when no parent exists.
func TestJavaFullyQualifiedNameParent(t *testing.T) {
	tests := []struct {
		parts []string
		want  []string
	}{
		{[]string{"com", "example", "foo"}, []string{"com", "example"}},
		{[]string{"com", "example"}, []string{"com"}},
		{[]string{"com"}, nil},
		{nil, nil},
	}

	for _, tt := range tests {
		var fqn *javaFullyQualifiedName
		if tt.parts != nil {
			fqn = &javaFullyQualifiedName{parts: tt.parts}
		}
		parent := fqn.Parent()
		var got []string
		if parent != nil {
			got = parent.parts
		}
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("Parent() for parts %v = %v, want %v", tt.parts, got, tt.want)
		}
	}
}
