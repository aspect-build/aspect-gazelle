package gazelle

import (
	"strings"
	"testing"
)

func TestParsePackageJsonImports(t *testing.T) {
	t.Run("basic file refs", func(t *testing.T) {
		assertParsePackageJsonImports(t, `{"main":"foo.js"}`, "foo.js")
		assertParsePackageJsonImports(t, `{"main":"foo/bar.js"}`, "foo/bar.js")
		assertParsePackageJsonImports(t, `{"main":"foo/../bar.js"}`, "bar.js")
		assertParsePackageJsonImports(t, `{"types":"foo.d.ts"}`, "foo.d.ts")
		assertParsePackageJsonImports(t, `{"typings":"foo.d.ts"}`, "foo.d.ts")
	})

	t.Run("package exports", func(t *testing.T) {
		// String
		assertParsePackageJsonImports(t, `{"exports":"./foo.js"}`, "foo.js")

		// Object
		assertParsePackageJsonImports(t, `{"exports":{"entry-name":"./foo.js"}}`, "foo.js")
		assertParsePackageJsonImports(t, `{"exports":{"./subpath":"./foo.js"}}`, "foo.js")
		assertParsePackageJsonImports(t, `{"exports":{"./subpath":null}}`)
		assertParsePackageJsonImports(t, `{"exports":{"./subpath":"./foo.js","./subpath2":"./bar.js"}}`, "foo.js", "bar.js")

		// Array
		assertParsePackageJsonImports(t, `{"exports":[]}`)
		assertParsePackageJsonImports(t, `{"exports":["./foo.js"]}`, "foo.js")
	})

	t.Run("invalid exports", func(t *testing.T) {
		assertParsePackageJsonImports(t, `{"exports":null}`)
		assertParsePackageJsonImports(t, `{"exports":{"./subpath":123, "x": []}}`)
	})
}

func TestParsePackageJsonTypesField(t *testing.T) {
	t.Run("types field", func(t *testing.T) {
		assertParsePackageJsonTypesField(t, `{"types":"./src/index.ts"}`, "src/index.ts")
		assertParsePackageJsonTypesField(t, `{"types":"./dist/index.d.ts"}`, "dist/index.d.ts")
	})

	t.Run("typings fallback", func(t *testing.T) {
		assertParsePackageJsonTypesField(t, `{"typings":"./src/index.ts"}`, "src/index.ts")
	})

	t.Run("no types fields", func(t *testing.T) {
		assertParsePackageJsonTypesField(t, `{}`, "")
		assertParsePackageJsonTypesField(t, `{"main":"./dist/index.js"}`, "")
		assertParsePackageJsonTypesField(t, `{"main":"./src/index.ts"}`, "")
	})

	t.Run("precedence", func(t *testing.T) {
		assertParsePackageJsonTypesField(t, `{"types":"./src/index.ts","typings":"./other.ts"}`, "src/index.ts")
		assertParsePackageJsonTypesField(t, `{"types":"./src/index.ts","main":"./other.js"}`, "src/index.ts")
	})

	t.Run("path normalization", func(t *testing.T) {
		assertParsePackageJsonTypesField(t, `{"types":"./src/../src/index.ts"}`, "src/index.ts")
	})

	t.Run("malformed json", func(t *testing.T) {
		_, err := ParsePackageJsonTypesField(strings.NewReader(`{invalid`))
		if err == nil {
			t.Error("ParsePackageJsonTypesField should return error for malformed JSON")
		}
	})
}

func assertParsePackageJsonTypesField(t *testing.T, packageJson string, expected string) {
	result, err := ParsePackageJsonTypesField(strings.NewReader(packageJson))
	if err != nil {
		t.Errorf("ParsePackageJsonTypesField failed: %v:\n\t%s", err, packageJson)
		return
	}
	if result != expected {
		t.Errorf("ParsePackageJsonTypesField(%q) expected %q, got %q", packageJson, expected, result)
	}
}

func assertParsePackageJsonImports(t *testing.T, packageJson string, expectedImports ...string) {
	imps, err := ParsePackageJsonImports(strings.NewReader(packageJson))

	if err != nil {
		t.Errorf("ParsePackageJsonImports failed: %v:\n\t%s", err, packageJson)
		return
	}
	if len(imps) != len(expectedImports) {
		t.Errorf("ParsePackageJsonImports expected %d imports, got %d:\n\t%s", len(expectedImports), len(imps), packageJson)
		return
	}
	for i, expected := range expectedImports {
		if imps[i] != expected {
			t.Errorf("ParsePackageJsonImports(%q) expected import %d to be %q, got %q", packageJson, i, expected, imps[i])
		}
	}
}
