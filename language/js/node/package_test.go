package node

import (
	"slices"
	"strings"
	"testing"
)

func TestParsePackageJson(t *testing.T) {
	t.Run("basic file refs", func(t *testing.T) {
		assertParsePackageJsonEntries(t, `{"main":"foo.js"}`, "foo.js")
		assertParsePackageJsonEntries(t, `{"main":"foo/bar.js"}`, "foo/bar.js")
		assertParsePackageJsonEntries(t, `{"main":"foo/../bar.js"}`, "bar.js")
		assertParsePackageJsonEntries(t, `{"types":"foo.d.ts"}`, "foo.d.ts")
		assertParsePackageJsonEntries(t, `{"typings":"foo.d.ts"}`, "foo.d.ts")
	})

	t.Run("package exports", func(t *testing.T) {
		// String
		assertParsePackageJsonEntries(t, `{"exports":"./foo.js"}`, "foo.js")

		// Object
		assertParsePackageJsonEntries(t, `{"exports":{"entry-name":"./foo.js"}}`, "foo.js")
		assertParsePackageJsonEntries(t, `{"exports":{"./subpath":"./foo.js"}}`, "foo.js")
		assertParsePackageJsonEntries(t, `{"exports":{"./subpath":null}}`)
		assertParsePackageJsonEntries(t, `{"exports":{"./subpath":"./foo.js","./subpath2":"./bar.js"}}`, "foo.js", "bar.js")

		// Array
		assertParsePackageJsonEntries(t, `{"exports":[]}`)
		assertParsePackageJsonEntries(t, `{"exports":["./foo.js"]}`, "foo.js")
	})

	t.Run("invalid exports", func(t *testing.T) {
		assertParsePackageJsonEntries(t, `{"exports":null}`)
		assertParsePackageJsonEntries(t, `{"exports":{"./subpath":123, "x": []}}`)
	})

	t.Run("package files", func(t *testing.T) {
		assertParsePackageJsonFiles(t, `{}`)
		assertParsePackageJsonFiles(t, `{"files":[]}`)
		assertParsePackageJsonFiles(t, `{"files":["lib"]}`, "lib")
		assertParsePackageJsonFiles(t, `{"files":["./lib/", "README.md"]}`, "lib", "README.md")
		assertParsePackageJsonFiles(t, `{"files":["dist/**/*.js", "!dist/test"]}`, "dist/**/*.js", "!dist/test")
		assertParsePackageJsonFiles(t, `{"files":["!./dist/test/"]}`, "!dist/test")

		// Patterns escaping the package are dropped.
		assertParsePackageJsonFiles(t, `{"files":[".", "..", "../sibling", "lib"]}`, "lib")

		// A leading "/" anchors to the package root and is treated as relative.
		assertParsePackageJsonFiles(t, `{"files":["/dist", "//lib/", "/../escape"]}`, "dist", "lib")

		// 'files' patterns are not entry points.
		assertParsePackageJsonEntries(t, `{"main":"start.js", "files":["lib", "docs/*.md"]}`, "start.js")
	})

	t.Run("package name", func(t *testing.T) {
		pkg := parsePackageJson(t, `{"name":"@scope/pkg","main":"foo.js"}`)
		if pkg.Name != "@scope/pkg" {
			t.Errorf("expected package name %q, got %q", "@scope/pkg", pkg.Name)
		}
	})

	t.Run("exports by subpath", func(t *testing.T) {
		// The package root "." normalizes to "", subpaths drop the leading "./".
		assertResolveExport(t, parsePackageJson(t, `{"exports":"./foo.js"}`), "", "foo.js")
		assertResolveExport(t, parsePackageJson(t, `{"exports":{".":"./foo.js"}}`), "", "foo.js")
		assertResolveExport(t, parsePackageJson(t, `{"exports":{"./sub":"./lib/sub.js"}}`), "sub", "lib/sub.js")

		// Array exports map to the package root.
		assertResolveExport(t, parsePackageJson(t, `{"exports":["./foo.js","./bar.js"]}`), "", "bar.js", "foo.js")

		// Top-level non-"." keys are conditions of the package root export.
		// Targets are sorted regardless of JSON condition order.
		assertResolveExport(t, parsePackageJson(t, `{"exports":{"node":"./foo.js","default":"./bar.js"}}`), "", "bar.js", "foo.js")

		// Conditional subpath exports.
		conds := parsePackageJson(t, `{"exports":{".":{"node":"./foo.js"},"./sub":{"types":"./sub.d.ts","default":"./sub.js"}}}`)
		assertResolveExport(t, conds, "", "foo.js")
		assertResolveExport(t, conds, "sub", "sub.d.ts", "sub.js")

		// Excluded and unknown subpaths.
		assertResolveExport(t, parsePackageJson(t, `{"exports":{"./sub":null}}`), "sub")
		assertResolveExport(t, parsePackageJson(t, `{"main":"foo.js"}`), "")
		assertResolveExport(t, parsePackageJson(t, `{"exports":"./foo.js"}`), "unknown")
	})

	t.Run("exports subpath patterns", func(t *testing.T) {
		pkg := parsePackageJson(t, `{"exports":{
			"./feat/*": "./src/feat/*.js",
			"./feat/special": "./special.js",
			"./feat/deep/*": "./src/deep/*.mjs",
			"./multi/*": {"node": "./n/*.cjs", "default": "./d/*.js"}
		}}`)

		// Exact matches take precedence over patterns.
		assertResolveExport(t, pkg, "feat/special", "special.js")

		// Pattern matches, '*' may span '/'.
		assertResolveExport(t, pkg, "feat/foo", "src/feat/foo.js")
		assertResolveExport(t, pkg, "feat/a/b", "src/feat/a/b.js")

		// The longest matching prefix wins.
		assertResolveExport(t, pkg, "feat/deep/x", "src/deep/x.mjs")

		// Conditional pattern targets are all expanded.
		assertResolveExport(t, pkg, "multi/x", "d/x.js", "n/x.cjs")

		// No match: unknown subpaths and empty '*' matches.
		assertResolveExport(t, pkg, "unknown")
		assertResolveExport(t, pkg, "feat/")

		// Invalid pattern keys with multiple '*'s are dropped when parsed.
		invalid := parsePackageJson(t, `{"exports":{"./bad/*/*":"./x/*.js"}}`)
		assertResolveExport(t, invalid, "bad/a/b")
	})

	t.Run("subpath imports", func(t *testing.T) {
		// Internal file targets are entries, external package targets are not
		assertParsePackageJsonEntries(t, `{"imports":{"#utils":"./src/utils.js"}}`, "src/utils.js")
		assertParsePackageJsonEntries(t, `{"imports":{"#dep":"external-pkg"}}`)
		assertParsePackageJsonEntries(t, `{"imports":{"#dep":null}}`)

		assertPackageJsonImports(t, `{"main":"foo.js"}`, nil)
		assertPackageJsonImports(t, `{"imports":{"#utils":"./src/utils.js"}}`, map[string][]string{"#utils": {"./src/utils.js"}})
		assertPackageJsonImports(t, `{"imports":{"#dep":"external-pkg"}}`, map[string][]string{"#dep": {"external-pkg"}})
		assertPackageJsonImports(t, `{"imports":{"#dep":null}}`, nil)

		// Conditional subpath imports. Targets are sorted regardless of JSON
		// condition order.
		assertPackageJsonImports(t,
			`{"imports":{"#dep":{"node":"./node.js","default":"external-pkg"}}}`,
			map[string][]string{"#dep": {"./node.js", "external-pkg"}},
		)
		assertPackageJsonImports(t,
			`{"imports":{"#dep":{"default":"external-pkg","browser":"./b.js","node":"./a.js"}}}`,
			map[string][]string{"#dep": {"./a.js", "./b.js", "external-pkg"}},
		)

		// Invalid types
		assertPackageJsonImports(t, `{"imports":"./foo.js"}`, nil)
		assertPackageJsonImports(t, `{"imports":{"#dep":123}}`, nil)
	})

	t.Run("subpath import patterns", func(t *testing.T) {
		pkg := parsePackageJson(t, `{"imports":{
			"#internal/*": "./src/internal/*.js",
			"#internal/special": "./special.js",
			"#internal/deep/*": "./src/deep/*.mjs",
			"#suffix/*.js": "./s/*.js",
			"#multi/*": {"node": "./n/*.cjs", "default": "ext-pkg/*"}
		}}`)

		// Exact matches take precedence over patterns
		assertResolveImport(t, pkg, "#internal/special", "./special.js")

		// Pattern matches, '*' may span '/'
		assertResolveImport(t, pkg, "#internal/foo", "./src/internal/foo.js")
		assertResolveImport(t, pkg, "#internal/a/b", "./src/internal/a/b.js")

		// The longest matching prefix wins
		assertResolveImport(t, pkg, "#internal/deep/x", "./src/deep/x.mjs")

		// With equal prefixes, the longest matching suffix wins
		suffixes := parsePackageJson(t, `{"imports":{"#a/*":"./plain/*.js","#a/*.js":"./js/*.mjs"}}`)
		assertResolveImport(t, suffixes, "#a/foo.js", "./js/foo.mjs")

		// A higher-priority pattern that prefix/suffix-matches but has only an
		// empty '*' is skipped, falling through to a lower-priority match.
		shadow := parsePackageJson(t, `{"imports":{"#x/*":"./a/*.js","#x/y*":"./b/*.js"}}`)
		assertResolveImport(t, shadow, "#x/y", "./a/y.js")

		// Patterns with a suffix
		assertResolveImport(t, pkg, "#suffix/y.js", "./s/y.js")

		// Conditional pattern targets are all expanded
		assertResolveImport(t, pkg, "#multi/x", "./n/x.cjs", "ext-pkg/x")

		// No match: unknown specifiers and empty '*' matches
		assertResolveImport(t, pkg, "#unknown")
		assertResolveImport(t, pkg, "#internal/")

		// Invalid pattern keys with multiple '*'s are dropped when parsed
		invalid := parsePackageJson(t, `{"imports":{"#bad/*/*":"./x/*.js"}}`)
		assertResolveImport(t, invalid, "#bad/a/b")

		// An overlapping prefix and suffix has no (non-empty) '*' match and
		// must not panic on the out-of-range substring: '#abc' starts with
		// '#ab' and ends with 'bc' but is shorter than their sum.
		overlap := parsePackageJson(t, `{"imports":{"#ab*bc":"./x/*.js"}}`)
		assertResolveImport(t, overlap, "#abc")
	})
}

func assertResolveImport(t *testing.T, pkg PackageJson, specifier string, expectedTargets ...string) {
	t.Helper()

	actual := pkg.ResolveImport(specifier)
	if len(expectedTargets) == 0 {
		if actual != nil {
			t.Errorf("ResolveImport(%q) expected no targets, got %q", specifier, actual)
		}
		return
	}

	if !slices.Equal(actual, expectedTargets) {
		t.Errorf("ResolveImport(%q) expected %q, got %q", specifier, expectedTargets, actual)
	}
}

func assertResolveExport(t *testing.T, pkg PackageJson, subpath string, expectedTargets ...string) {
	t.Helper()

	actual := pkg.ResolveExport(subpath)
	if len(expectedTargets) == 0 {
		if actual != nil {
			t.Errorf("ResolveExport(%q) expected no targets, got %q", subpath, actual)
		}
		return
	}

	if !slices.Equal(actual, expectedTargets) {
		t.Errorf("ResolveExport(%q) expected %q, got %q", subpath, expectedTargets, actual)
	}
}

func parsePackageJson(t *testing.T, packageJson string) PackageJson {
	t.Helper()

	pkg, err := ParsePackageJson(strings.NewReader(packageJson))
	if err != nil {
		t.Fatalf("ParsePackageJson failed: %v:\n\t%s", err, packageJson)
	}
	return pkg
}

func assertParsePackageJsonEntries(t *testing.T, packageJson string, expectedEntries ...string) {
	t.Helper()

	entries := slices.Clone(parsePackageJson(t, packageJson).Entries)

	slices.Sort(expectedEntries)
	slices.Sort(entries)

	if !slices.Equal(entries, expectedEntries) {
		t.Errorf("ParsePackageJson(%q) expected entries %q, got %q", packageJson, expectedEntries, entries)
	}
}

// assertParsePackageJsonFiles asserts the normalized 'files' patterns, order-sensitive.
func assertParsePackageJsonFiles(t *testing.T, packageJson string, expectedFiles ...string) {
	t.Helper()

	files := parsePackageJson(t, packageJson).Files

	if len(files) == 0 && len(expectedFiles) == 0 {
		return
	}

	if !slices.Equal(files, expectedFiles) {
		t.Errorf("ParsePackageJson(%q) expected files %q, got %q", packageJson, expectedFiles, files)
	}
}

// assertPackageJsonImports asserts the exact (non-pattern) 'imports' compiled
// when parsing the package.json. A nil expectation asserts no 'imports' field.
func assertPackageJsonImports(t *testing.T, packageJson string, expectedImports map[string][]string) {
	t.Helper()

	pkg := parsePackageJson(t, packageJson)

	if (pkg.Imports == nil) != (expectedImports == nil) {
		t.Errorf("ParsePackageJson(%q) expected imports %v, got %v", packageJson, expectedImports, pkg.Imports)
		return
	}

	if len(pkg.ImportPatterns) > 0 || len(pkg.Imports) != len(expectedImports) {
		t.Errorf("ParsePackageJson(%q) expected exact imports %v, got %v patterns %v", packageJson, expectedImports, pkg.Imports, pkg.ImportPatterns)
		return
	}

	for specifier, expectedTargets := range expectedImports {
		// Order-sensitive: targets are sorted when parsed for deterministic resolution.
		if !slices.Equal(pkg.Imports[specifier], expectedTargets) {
			t.Errorf("ParsePackageJson(%q) expected imports[%q] %q, got %q", packageJson, specifier, expectedTargets, pkg.Imports[specifier])
		}
	}
}
