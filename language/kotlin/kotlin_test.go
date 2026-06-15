package gazelle

import (
	"testing"
)

func TestKotlinNative(t *testing.T) {
	testCases := []struct {
		library string
		want    bool
	}{
		{"kotlin.io", true},
		{"kotlinx.foo", true},
		{"java.foo", true},
		{"javax.net", true},
		{"javax.sql", true},
		{"javax.xml", true},
		{"org.xml.sax", true},
		{"javax.accessibility", true},
	}

	for _, tc := range testCases {
		t.Run(tc.library, func(t *testing.T) {
			got := IsNativeImport(tc.library)
			if got != tc.want {
				t.Errorf("IsNativeImport(%q) got %v, want %v", tc.library, got, tc.want)
			}
		})
	}
}
