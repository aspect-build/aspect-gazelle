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

	t.Run("conditional exports", func(t *testing.T) {
		assertParsePackageJsonEntries(t, `{"exports":{"node":"./foo.js","default":"./bar.js"}}`, "foo.js", "bar.js")
		assertParsePackageJsonEntries(t,
			`{"exports":{".":{"node":"./foo.js"},"./sub":{"types":"./sub.d.ts","default":"./sub.js"}}}`,
			"foo.js", "sub.d.ts", "sub.js",
		)
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
