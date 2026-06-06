package gazelle

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

	t.Run("package name", func(t *testing.T) {
		pkg := parsePackageJson(t, `{"name":"@scope/pkg","main":"foo.js"}`)
		if pkg.Name != "@scope/pkg" {
			t.Errorf("expected package name %q, got %q", "@scope/pkg", pkg.Name)
		}
	})

	t.Run("subpath imports", func(t *testing.T) {
		// Internal file targets are entries, external package targets are not
		assertParsePackageJsonEntries(t, `{"imports":{"#utils":"./src/utils.js"}}`, "src/utils.js")
		assertParsePackageJsonEntries(t, `{"imports":{"#dep":"external-pkg"}}`)
		assertParsePackageJsonEntries(t, `{"imports":{"#dep":null}}`)

		assertPackageJsonImports(t, `{"main":"foo.js"}`, nil)
		assertPackageJsonImports(t, `{"imports":{"#utils":"./src/utils.js"}}`, map[string][]string{"#utils": {"./src/utils.js"}})
		assertPackageJsonImports(t, `{"imports":{"#dep":"external-pkg"}}`, map[string][]string{"#dep": {"external-pkg"}})
		assertPackageJsonImports(t, `{"imports":{"#dep":null}}`, map[string][]string{})

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
		assertPackageJsonImports(t, `{"imports":{"#dep":123}}`, map[string][]string{})
	})
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

func assertPackageJsonImports(t *testing.T, packageJson string, expectedImports map[string][]string) {
	t.Helper()
	assertSpecifierMap(t, "imports", packageJson, parsePackageJson(t, packageJson).Imports, expectedImports)
}

func assertSpecifierMap(t *testing.T, field, packageJson string, actual, expected map[string][]string) {
	t.Helper()

	if (actual == nil) != (expected == nil) || len(actual) != len(expected) {
		t.Errorf("ParsePackageJson(%q) expected %s %v, got %v", packageJson, field, expected, actual)
		return
	}

	for specifier, expectedTargets := range expected {
		// Order-sensitive: targets are sorted when parsed for deterministic resolution.
		if !slices.Equal(actual[specifier], expectedTargets) {
			t.Errorf("ParsePackageJson(%q) expected %s[%q] %q, got %q", packageJson, field, specifier, expectedTargets, actual[specifier])
		}
	}
}
