package gazelle

import (
	"fmt"
	"path"
	"reflect"
	"slices"
	"testing"
)

func TestGenerate(t *testing.T) {
	for _, tc := range []struct {
		pkg, from, impt string
		expected        string
	}{
		// Empty import path
		{
			pkg:      "",
			from:     "from.ts",
			impt:     "",
			expected: "",
		},
		// Simple
		{
			pkg:      "",
			from:     "from.ts",
			impt:     "./empty",
			expected: "empty",
		},
		{
			pkg:      "",
			from:     "from/sub.ts",
			impt:     "./empty",
			expected: "from/empty",
		},
		{
			pkg:      "foo",
			from:     "from.ts",
			impt:     "./bar",
			expected: "foo/bar",
		},
		{
			pkg:      "foo",
			from:     "from/sub.ts",
			impt:     "./bar",
			expected: "foo/from/bar",
		},
		// Absolute
		{
			pkg:      "",
			from:     "from.ts",
			impt:     "workspace/is/common",
			expected: "workspace/is/common",
		},
		{
			pkg:      "",
			from:     "from.ts",
			impt:     "/workspace/is/common",
			expected: "workspace/is/common",
		},
		{
			pkg:      "dont-use-me",
			from:     "from.ts",
			impt:     "workspace/is/common",
			expected: "workspace/is/common",
		},
		// Parent (..)
		{
			pkg:      "",
			from:     "from.ts",
			impt:     "./foo/../bar",
			expected: "bar",
		},
		{
			pkg:      "",
			from:     "from/sub.ts",
			impt:     "./foo/../bar",
			expected: "from/bar",
		},
		{
			pkg:      "foo",
			from:     "from.ts",
			impt:     "../bar",
			expected: "bar",
		},
		{
			pkg:      "foo",
			from:     "from/sub.ts",
			impt:     "../bar",
			expected: "foo/bar",
		},
		{
			pkg:      "foo",
			from:     "from.ts",
			impt:     "./baz/../bar",
			expected: "foo/bar",
		},
		{
			pkg:      "foo",
			from:     "from/sub.ts",
			impt:     "./baz/../bar",
			expected: "foo/from/bar",
		},
		// Absolute parent
		{
			pkg:      "dont-use-me",
			from:     "from.ts",
			impt:     "baz/../bar",
			expected: "bar",
		},
		{
			pkg:      "dont-use-me",
			from:     "from/sub.ts",
			impt:     "baz/../bar",
			expected: "bar",
		},
		// URLs
		{
			pkg:      "dont-use-me",
			from:     "anywhere.ts",
			impt:     "https://me.com",
			expected: "https://me.com",
		},
		{
			pkg:      "dont-use-me",
			from:     "anywhere.ts",
			impt:     "http://me.com",
			expected: "http://me.com",
		},
		{
			pkg:      "dont-use-me",
			from:     "anywhere.ts",
			impt:     "anything://me",
			expected: "anything://me",
		},
		// Query params
		{
			pkg:      "",
			from:     "from.ts",
			impt:     "./styles.css?no-inline",
			expected: "styles.css",
		},
		{
			pkg:      "",
			from:     "from.ts",
			impt:     "./styles.css#no-inline",
			expected: "styles.css",
		},
		{
			pkg:      "",
			from:     "from.ts",
			impt:     "./styles.css?no-inline#hash",
			expected: "styles.css",
		},
		{
			pkg:      "dont-use-me",
			from:     "anywhere.ts",
			impt:     "https://me.com/file.css?no-inline",
			expected: "https://me.com/file.css",
		},
		// Bare package imports should not be resolved as relative paths
		{
			pkg:      "packages/app",
			from:     "packages/app/src/component.tsx",
			impt:     "react",
			expected: "react",
		},
		{
			pkg:      "packages/app",
			from:     "packages/app/src/component.tsx",
			impt:     "lodash",
			expected: "lodash",
		},
		{
			pkg:      "packages/app",
			from:     "packages/app/src/component.tsx",
			impt:     "@scope/package",
			expected: "@scope/package",
		},
	} {
		desc := fmt.Sprintf("toImportSpecPath(%s, %s, %s)", tc.pkg, tc.from, tc.impt)

		t.Run(desc, func(t *testing.T) {
			importPath := toImportSpecPath("", path.Join(tc.pkg, tc.from), tc.impt)

			if !reflect.DeepEqual(importPath, tc.expected) {
				t.Errorf("toImportSpecPath('%s', '%s', '%s'): \nactual:   %s\nexpected:  %s\n", tc.pkg, tc.from, tc.impt, importPath, tc.expected)
			}
		})
	}

	// Edge cases for the importDir optimization (strings.LastIndex-based directory extraction)
	t.Run("toImportSpecPath importDir edge cases", func(t *testing.T) {
		for _, tc := range []struct {
			from, impt string
			expected   string
		}{
			// No slash in importFrom — directory portion is empty
			{from: "file.ts", impt: "./foo", expected: "foo"},
			{from: "file.ts", impt: "bar", expected: "bar"},

			// Deep nesting
			{from: "deep/nested/dir/file.ts", impt: "../sibling", expected: "deep/nested/sibling"},
			{from: "a/b/c.ts", impt: "../../x", expected: "x"},

			// importPath with multiple parent traversals
			{from: "a/b/c/d.ts", impt: "../../../root", expected: "root"},
			{from: "a/b/c/d.ts", impt: "./local", expected: "a/b/c/local"},

			// Dot-only import
			{from: "a/b.ts", impt: ".", expected: "a"},
			{from: "a/b.ts", impt: "..", expected: "."},
		} {
			desc := fmt.Sprintf("toImportSpecPath(from=%s, impt=%s)", tc.from, tc.impt)
			t.Run(desc, func(t *testing.T) {
				actual := toImportSpecPath("", tc.from, tc.impt)
				if actual != tc.expected {
					t.Errorf("toImportSpecPath('', %q, %q):\n\tactual:   %s\n\texpected: %s", tc.from, tc.impt, actual, tc.expected)
				}
			})
		}
	})

	t.Run("toImportPaths", func(t *testing.T) {
		// Traditional [.d].ts[x] don't require an extension
		assertImports(t, "bar.ts", []string{"bar", "bar.d.ts", "bar.js"})
		assertImports(t, "bar.tsx", []string{"bar", "bar.d.ts", "bar.js"})
		assertImports(t, "bar.d.ts", []string{"bar", "bar.d.ts", "bar.js"})
		assertImports(t, "foo/bar.ts", []string{"foo/bar", "foo/bar.d.ts", "foo/bar.js"})
		assertImports(t, "foo/bar.tsx", []string{"foo/bar", "foo/bar.d.ts", "foo/bar.js"})
		assertImports(t, "foo/bar.d.ts", []string{"foo/bar", "foo/bar.d.ts", "foo/bar.js"})

		// Traditional [.d].ts[x] index files
		assertImports(t, "bar/index.ts", []string{"bar/index", "bar/index.d.ts", "bar/index.js", "bar"})
		assertImports(t, "bar/index.d.ts", []string{"bar/index", "bar/index.d.ts", "bar/index.js", "bar"})
		assertImports(t, "bar/index.tsx", []string{"bar/index", "bar/index.d.ts", "bar/index.js", "bar"})

		// .mjs and .cjs files require an extension
		assertImports(t, "bar.mts", []string{"bar.mjs", "bar.d.mts"})
		assertImports(t, "bar/index.mts", []string{"bar/index.mjs", "bar/index.d.mts", "bar"})
		assertImports(t, "bar.d.mts", []string{"bar.d.mts", "bar.mjs"})
		assertImports(t, "bar.cts", []string{"bar.cjs", "bar.d.cts"})
		assertImports(t, "bar/index.cts", []string{"bar/index.cjs", "bar/index.d.cts", "bar"})
		assertImports(t, "bar.d.cts", []string{"bar.d.cts", "bar.cjs"})
	})

	// Test bare paths with absolutePathBase (used for new URL() imports)
	t.Run("toImportSpecPath bare paths with absolutePathBase", func(t *testing.T) {
		for _, tc := range []struct {
			pkg, from, impt string
			expected        string
		}{
			{
				pkg:      "",
				from:     "from.ts",
				impt:     "asset.png",
				expected: "asset.png",
			},
			{
				pkg:      "",
				from:     "sub/from.ts",
				impt:     "asset.png",
				expected: "sub/asset.png",
			},
			{
				pkg:      "foo",
				from:     "sub/from.ts",
				impt:     "./asset.png",
				expected: "foo/sub/asset.png",
			},
			{
				pkg:      "foo",
				from:     "sub/from.ts",
				impt:     "/asset.png",
				expected: "foo/sub/asset.png",
			},
			{
				pkg:      "foo",
				from:     "sub/from.ts",
				impt:     "asset.png",
				expected: "foo/sub/asset.png",
			},
			{
				pkg:      "",
				from:     "from.ts",
				impt:     "asset.png?no-inline",
				expected: "asset.png",
			},
			{
				pkg:      "",
				from:     "from.ts",
				impt:     "https://me.com/asset.png",
				expected: "https://me.com/asset.png",
			},
		} {
			desc := fmt.Sprintf("toImportSpecPath(%s, %s, %s) bare path", tc.pkg, tc.from, tc.impt)
			t.Run(desc, func(t *testing.T) {
				// Using "." as absolutePathBase triggers bare path handling (treats bare paths as relative)
				importPath := toImportSpecPath(".", path.Join(tc.pkg, tc.from), tc.impt)

				if !reflect.DeepEqual(importPath, tc.expected) {
					t.Errorf("toImportSpecPath(\".\", '%s', '%s') : \nactual:    %s\nexpected:  %s\n", path.Join(tc.pkg, tc.from), tc.impt, importPath, tc.expected)
				}
			})
		}
	})

	// Test absolute paths with absolutePathBase (used for JSX <img src="/..."> imports)
	t.Run("toImportSpecPath absolute paths with absolutePathBase", func(t *testing.T) {
		for _, tc := range []struct {
			base, from, impt string
			expected         string
		}{
			// Absolute path resolved relative to base (package.json location)
			{
				base:     "packages/app",
				from:     "packages/app/src/component.tsx",
				impt:     "/images/logo.png",
				expected: "packages/app/images/logo.png",
			},
			// Absolute path under nested directory resolved relative to base
			{
				base:     "packages/app",
				from:     "packages/app/src/component.tsx",
				impt:     "/sub/images/logo.png",
				expected: "packages/app/sub/images/logo.png",
			},
			// Absolute path from nested subdirectory
			{
				base:     "packages/app",
				from:     "packages/app/src/deep/nested/component.tsx",
				impt:     "/assets/video.mp4",
				expected: "packages/app/assets/video.mp4",
			},
			// Absolute path with query param (should be stripped)
			{
				base:     "packages/app",
				from:     "packages/app/src/component.tsx",
				impt:     "/images/logo.png?v=1",
				expected: "packages/app/images/logo.png",
			},
			// Absolute path with query and hash (should be stripped)
			{
				base:     "packages/app",
				from:     "packages/app/src/component.tsx",
				impt:     "/images/logo.png?inline#hash",
				expected: "packages/app/images/logo.png",
			},
			// Absolute path with hash (should be stripped)
			{
				base:     "packages/app",
				from:     "packages/app/src/component.tsx",
				impt:     "/images/logo.png#section",
				expected: "packages/app/images/logo.png",
			},
			// Absolute path with parent segments (should be cleaned)
			{
				base:     "packages/app",
				from:     "packages/app/src/component.tsx",
				impt:     "/images/../logo.png",
				expected: "packages/app/logo.png",
			},
			// Relative path still resolves relative to source file, not base
			{
				base:     "packages/app",
				from:     "packages/app/src/component.tsx",
				impt:     "./local.png",
				expected: "packages/app/src/local.png",
			},
			// Relative path with ..
			{
				base:     "packages/app",
				from:     "packages/app/src/deep/component.tsx",
				impt:     "../images/logo.png",
				expected: "packages/app/src/images/logo.png",
			},
			// Root-level base
			{
				base:     "",
				from:     "src/component.tsx",
				impt:     "/images/logo.png",
				expected: "images/logo.png",
			},
			// URL should pass through unchanged
			{
				base:     "packages/app",
				from:     "packages/app/src/component.tsx",
				impt:     "https://example.com/image.png",
				expected: "https://example.com/image.png",
			},
			// Bare package imports should not be resolved as relative paths
			{
				base:     "packages/app",
				from:     "packages/app/src/component.tsx",
				impt:     "react",
				expected: "react",
			},
			{
				base:     "packages/app",
				from:     "packages/app/src/component.tsx",
				impt:     "lodash",
				expected: "lodash",
			},
			{
				base:     "packages/app",
				from:     "packages/app/src/component.tsx",
				impt:     "@scope/package",
				expected: "@scope/package",
			},
		} {
			desc := fmt.Sprintf("toImportSpecPath(base=%s, from=%s, impt=%s)", tc.base, tc.from, tc.impt)
			t.Run(desc, func(t *testing.T) {
				importPath := toImportSpecPath(tc.base, tc.from, tc.impt)

				if !reflect.DeepEqual(importPath, tc.expected) {
					t.Errorf("toImportSpecPath('%s', '%s', '%s') : \nactual:    %s\nexpected:  %s\n", tc.base, tc.from, tc.impt, importPath, tc.expected)
				}
			})
		}
	})
}

func assertImports(t *testing.T, p string, expected []string) {
	actual := toImportPaths(p)

	// Order doesn't matter so sort to ignore order
	slices.Sort(actual)
	slices.Sort(expected)

	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("toImportPaths('%s'): \nactual:   %s\nexpected:  %s\n", p, actual, expected)
	}
}
