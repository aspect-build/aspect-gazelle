package pnpm

import (
	"io"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestPnpmLockParseDependencies(t *testing.T) {
	t.Run("lockfile version", func(t *testing.T) {
		for content, expected := range map[string]string{
			"lockfileVersion: 5.4":   "5.4",
			"lockfileVersion: '6.0'": "6.0",
			// Multi-digit versions must parse so a future bump fails with
			// "unsupported version" instead of a version-parse error.
			"lockfileVersion: '10.0'": "10.0",
			// A leading document separator is what pnpm 12 writes.
			"---\nlockfileVersion: '9.0'": "9.0",
		} {
			v, e := parseVersionOfFirstDocument(t, content)
			if e != nil {
				t.Error(e)
			} else if v != expected {
				t.Errorf("Failed to parse lockfile version %s from %q, got %q", expected, content, v)
			}
		}
	})

	t.Run("empty lock file", func(t *testing.T) {
		emptyLock, err := parsePnpmLockDependencies(strings.NewReader(""))
		if err != nil {
			t.Error("Parse failure: ", err)
		}
		if emptyLock != nil {
			t.Errorf("Empty lock file returned non-nil, got: %v", emptyLock)
		}
	})

	t.Run("unsupported version", func(t *testing.T) {
		_, err := parsePnpmLockDependencies(strings.NewReader("lockfileVersion: 4.0"))
		if err == nil {
			t.Error("Expected error for unsupported version (4.0)")
		}

		_, err2 := parsePnpmLockDependencies(strings.NewReader("lockfileVersion: '4.0'"))
		if err2 == nil {
			t.Error("Expected error for unsupported version ('4.0')")
		}

		_, err3 := parsePnpmLockDependencies(strings.NewReader("lockfileVersion: 10.0"))
		if err3 == nil {
			t.Error("Expected error for unsupported version (10)")
		}
	})

	t.Run("basic deps (lockfile v5)", func(t *testing.T) {
		basic, err := parsePnpmLockDependencies(strings.NewReader(`lockfileVersion: 5.4

specifiers:
  '@aspect-test/a': 5.0.2
  '@aspect-test/c': 2.0.2
  jquery: 3.6.1

dependencies:
  '@aspect-test/a': 5.0.2

devDependencies:
  '@aspect-test/c': 2.0.2

peerDependencies:
  jquery: 3.6.1
`))

		if err != nil {
			t.Error("Parse failure: ", err)
		}

		if len(basic) != 1 || basic["."] == nil {
			t.Error("Simple deps parse error. Expected only '.' workspace, found ", len(basic))
		}

		if len(basic["."]) != 3 {
			t.Error("Simple deps parse error. Expected 3 deps in 1 workspace entry, found ", len(basic["."]))
		}

		if basic["."]["jquery"] != "3.6.1" {
			t.Errorf("Simple deps parse error. Expected 2.0.2 version for @aspect-test/c, found %q", basic["."]["@aspect-test/c"])
		}
	})

	t.Run("basic deps (lockfile v6)", func(t *testing.T) {
		basic, err := parsePnpmLockDependencies(strings.NewReader(`lockfileVersion: '6.0'

dependencies:
  '@aspect-test/a':
    specifier: 5.0.2
    version: 5.0.2
  jquery:
    specifier: 3.6.1
    version: 3.6.1

devDependencies:
  '@aspect-test/c':
    specifier: 2.0.2
    version: 2.0.2
`))

		if err != nil {
			t.Error("Parse failure: ", err)
		}

		if len(basic) != 1 || basic["."] == nil {
			t.Error("Simple deps parse error. Expected only '.' workspace, found ", len(basic))
		}

		if len(basic["."]) != 3 {
			t.Error("Simple deps parse error. Expected 3 deps in 1 workspace entry, found ", len(basic["."]))
		}

		if basic["."]["jquery"] != "3.6.1" {
			t.Errorf("Simple deps parse error. Expected 2.0.2 version for @aspect-test/c, found %q", basic["."]["@aspect-test/c"])
		}
	})

	t.Run("basic deps (lockfile v9)", func(t *testing.T) {
		basic, err := parsePnpmLockDependencies(strings.NewReader(`lockfileVersion: '9.0'

dependencies:
  '@aspect-test/a':
    specifier: 5.0.2
    version: 5.0.2
  jquery:
    specifier: 3.6.1
    version: 3.6.1

devDependencies:
  '@aspect-test/c':
    specifier: 2.0.2
    version: 2.0.2
`))

		if err != nil {
			t.Error("Parse failure: ", err)
		}

		if len(basic) != 1 || basic["."] == nil {
			t.Error("Simple deps parse error. Expected only '.' workspace, found ", len(basic))
		}

		if len(basic["."]) != 3 {
			t.Error("Simple deps parse error. Expected 3 deps in 1 workspace entry, found ", len(basic["."]))
		}

		if basic["."]["jquery"] != "3.6.1" {
			t.Errorf("Simple deps parse error. Expected 2.0.2 version for @aspect-test/c, found %q", basic["."]["@aspect-test/c"])
		}
	})

	t.Run("basic deps in single project workspace (lockfile v6)", func(t *testing.T) {
		basic, err := parsePnpmLockDependencies(strings.NewReader(`lockfileVersion: '6.0'

importers:
  .:
    dependencies:
      '@aspect-test/a':
        specifier: 5.0.2
        version: 5.0.2
      jquery:
        specifier: 3.6.1
        version: 3.6.1
    devDependencies:
      '@aspect-test/c':
        specifier: ^2.0.2
        version: 2.0.2
`))

		if err != nil {
			t.Error("Parse failure: ", err)
		}

		if len(basic) != 1 || basic["."] == nil {
			t.Error("Simple deps parse error. Expected only '.' workspace, found ", len(basic))
		}

		if len(basic["."]) != 3 {
			t.Error("Simple deps parse error. Expected 3 deps in 1 workspace entry, found ", len(basic["."]))
		}

		if basic["."]["jquery"] != "3.6.1" {
			t.Errorf("Simple deps parse error. Expected 2.0.2 version for @aspect-test/c, found %q", basic["."]["@aspect-test/c"])
		}
	})

	t.Run("basic deps in single project workspace (lockfile v9)", func(t *testing.T) {
		basic, err := parsePnpmLockDependencies(strings.NewReader(`lockfileVersion: '9.0'

importers:
  .:
    dependencies:
      '@aspect-test/a':
        specifier: 5.0.2
        version: 5.0.2
      jquery:
        specifier: 3.6.1
        version: 3.6.1
    devDependencies:
      '@aspect-test/c':
        specifier: ^2.0.2
        version: 2.0.2
`))

		if err != nil {
			t.Error("Parse failure: ", err)
		}

		if len(basic) != 1 || basic["."] == nil {
			t.Error("Simple deps parse error. Expected only '.' workspace, found ", len(basic))
		}

		if len(basic["."]) != 3 {
			t.Error("Simple deps parse error. Expected 3 deps in 1 workspace entry, found ", len(basic["."]))
		}

		if basic["."]["jquery"] != "3.6.1" {
			t.Errorf("Simple deps parse error. Expected 2.0.2 version for @aspect-test/c, found %q", basic["."]["@aspect-test/c"])
		}
	})

	t.Run("catalog deps (lockfile v9)", func(t *testing.T) {
		// Catalog entries (pnpm 9.5+) keep the resolved version on the importer
		// dependency, with the catalog indirection only in 'specifier'.
		basic, err := parsePnpmLockDependencies(strings.NewReader(`lockfileVersion: '9.0'

catalogs:
  default:
    jquery:
      specifier: 3.6.1
      version: 3.6.1
  tools:
    '@aspect-test/a':
      specifier: ^5.0.0
      version: 5.0.2

importers:
  .:
    dependencies:
      jquery:
        specifier: 'catalog:'
        version: 3.6.1
    devDependencies:
      '@aspect-test/a':
        specifier: catalog:tools
        version: 5.0.2
`))

		if err != nil {
			t.Error("Parse failure: ", err)
		}

		if len(basic) != 1 || basic["."] == nil {
			t.Error("Catalog deps parse error. Expected only '.' workspace, found ", len(basic))
		}

		if len(basic["."]) != 2 {
			t.Error("Catalog deps parse error. Expected 2 deps in 1 workspace entry, found ", len(basic["."]))
		}

		if basic["."]["jquery"] != "3.6.1" {
			t.Errorf("Catalog deps parse error. Expected 3.6.1 version for jquery, found %q", basic["."]["jquery"])
		}

		if basic["."]["@aspect-test/a"] != "5.0.2" {
			t.Errorf("Catalog deps parse error. Expected 5.0.2 version for @aspect-test/a, found %q", basic["."]["@aspect-test/a"])
		}
	})

	t.Run("basic deps in single project workspace (lockfile v5)", func(t *testing.T) {
		basic, err := parsePnpmLockDependencies(strings.NewReader(`lockfileVersion: 5.4

importers:
  .:
    specifiers:
      '@aspect-test/a': 5.0.2
      '@aspect-test/c': 2.0.2
      jquery: 3.6.1

    dependencies:
      '@aspect-test/a': 5.0.2
      '@aspect-test/c': 2.0.2
      jquery: 3.6.1
`))

		if err != nil {
			t.Error("Parse failure: ", err)
		}

		if len(basic) != 1 || basic["."] == nil {
			t.Error("Simple deps parse error. Expected only '.' workspace, found ", len(basic))
		}

		if len(basic["."]) != 3 {
			t.Error("Simple deps parse error. Expected 3 deps in 1 workspace entry, found ", len(basic["."]))
		}

		if basic["."]["jquery"] != "3.6.1" {
			t.Errorf("Simple deps parse error. Expected 2.0.2 version for @aspect-test/c, found %q", basic["."]["@aspect-test/c"])
		}
	})

	t.Run("no deps property", func(t *testing.T) {
		empty, err := parsePnpmLockDependencies(strings.NewReader(`lockfileVersion: 5.4
`))

		if err != nil {
			t.Error("Parse failure: ", err)
		}

		if len(empty) != 0 {
			t.Error("No deps parse error: ", empty)
		}
	})

	t.Run("deps to workspace pkgs (lockfile v5)", func(t *testing.T) {
		wksps, err := parsePnpmLockDependencies(strings.NewReader(`lockfileVersion: 5.3
importers:
  a:
    specifiers:
      '@lib/a': workspace:*
      '@lib/b': ./lib/a
      '@lib/c': file:./lib/a
      '@lib/d': link:./lib/a
      '@lib/e': workspace:@lib/a@*
      '@lib/f': workspace:./lib/a
      '@lib/g': npm:@lib/b@*
    dependencies:
      '@lib/a': link:./lib/a
      '@lib/b': link:./lib/a
      '@lib/c': link:./lib/a
      '@lib/d': link:./lib/a
      '@lib/e': link:./lib/a
      '@lib/f': link:./lib/a
      '@lib/g': link:./lib/a
`,
		))

		if err != nil {
			t.Error("Parse failure: ", err)
		}

		if len(wksps) != 1 || wksps["a"] == nil {
			t.Error("expected 1 importers, found: ", len(wksps))
		}

		for _, lib := range []string{"a", "b", "c", "d", "e", "f", "g"} {
			if wksps["a"]["@lib/"+lib] != "link:./lib/a" {
				t.Error("expected '@lib/a' dep, found: ", wksps["a"]) //["@lib/"+lib])
			}
		}
	})

	t.Run("deps to workspace pkgs (lockfile v6)", func(t *testing.T) {
		wksps, err := parsePnpmLockDependencies(strings.NewReader(`lockfileVersion: '6.1'
importers:
  a:
    dependencies:
      '@lib/a':
        specifier: workspace:*
        version: link:./lib/a
      '@lib/b':
        specifier: workspace:*
        version: link:./lib/a
      '@lib/c':
        specifier: link:./lib/a
        version: link:./lib/a
      '@lib/d':
        specifier: ./lib/a
        version: link:./lib/a
      '@lib/e':
        specifier: npm:@lib/a@*
        version: link:./lib/a
`,
		))

		if err != nil {
			t.Error("Parse failure: ", err)
		}

		if len(wksps) != 1 || wksps["a"] == nil {
			t.Error("expected 1 importers, found: ", len(wksps))
		}

		for _, lib := range []string{"a", "b", "c", "d", "e"} {
			if wksps["a"]["@lib/"+lib] != "link:./lib/a" {
				t.Error("expected '@lib/a' dep, found: ", wksps["a"]["@lib/"+lib])
			}
		}
	})

	t.Run("workspace deps (lockfile v5)", func(t *testing.T) {
		wksps, err := parsePnpmLockDependencies(strings.NewReader(`lockfileVersion: 5.4
importers:
  .:
    specifiers:
      '@aspect-test/a': ^2.0.2
    dependencies:
      '@aspect-test/a': ^2.0.2
  gazelle/ts/tests/simple_json_import:
    specifiers: {}
  infrastructure/cdn:
    specifiers:
      '@aspect-test/c': ^2.0.2
    dependencies:
      '@aspect-test/c': ^2.0.2
packages:
  /@aspect-test/c/2.0.2:
`))

		if err != nil {
			t.Error("Parse failure: ", err)
		}

		if len(wksps) != 3 || wksps["."] == nil || wksps["gazelle/ts/tests/simple_json_import"] == nil || wksps["infrastructure/cdn"] == nil {
			t.Error("expected 3 importers, found: ", len(wksps))
		}

		if len(wksps["."]) != 1 || wksps["."]["@aspect-test/a"] == "" {
			t.Error("expected main importer to have '@aspect-test/a' dep, found: ", wksps["."])
		}

		if len(wksps["gazelle/ts/tests/simple_json_import"]) != 0 {
			t.Error("expected 'gazelle/ts/tests/simple_json_import' importer to have no deps, found ", len(wksps["gazelle/ts/tests/simple_json_import"]))
		}

		if len(wksps["infrastructure/cdn"]) != 1 || wksps["infrastructure/cdn"]["@aspect-test/c"] == "" {
			t.Error("expected 'infrastructure/cdn' importer to have '@aspect-test/c' dep, found: ", wksps["infrastructure/cdn"])
		}
	})

	t.Run("pnpm 12 multi-document lockfile", func(t *testing.T) {
		// What pnpm 12 writes: its own self-management pins as a leading document,
		// the dependency graph as a second one. The first line is a separator, and
		// BOTH documents carry an `importers:` key — the leading one listing only
		// the root, so a parser that reads the first document sees one importer
		// instead of every workspace.
		deps, err := parsePnpmLockDependencies(strings.NewReader(`---
lockfileVersion: '9.0'

importers:

  .:
    packageManagerDependencies:
      '@pnpm/exe.linux-x64': 12.0.0

---
lockfileVersion: '9.0'

importers:

  .:
    dependencies:
      '@aspect-test/a':
        specifier: ^5.0.0
        version: 5.0.2

  projects/b:
    devDependencies:
      jquery:
        specifier: 3.6.1
        version: 3.6.1
`))

		if err != nil {
			t.Fatal("Parse failure: ", err)
		}
		if len(deps) != 2 {
			t.Fatalf("Expected 2 importers, got %d: %v", len(deps), deps)
		}
		if got := deps["."]["@aspect-test/a"]; got != "5.0.2" {
			t.Errorf("Root importer came from the wrong document, got %q for @aspect-test/a: %v", got, deps["."])
		}
		if got := deps["projects/b"]["jquery"]; got != "3.6.1" {
			t.Errorf("Expected jquery 3.6.1 for projects/b, got %q", got)
		}
	})

	t.Run("multi-document order does not matter", func(t *testing.T) {
		// The same two documents with the self-management one last. Nothing about
		// the fix should depend on which document comes first.
		deps, err := parsePnpmLockDependencies(strings.NewReader(`lockfileVersion: '9.0'

importers:

  .:
    dependencies:
      '@aspect-test/a':
        specifier: ^5.0.0
        version: 5.0.2

---
lockfileVersion: '9.0'

importers:

  .:
    packageManagerDependencies:
      '@pnpm/exe.linux-x64': 12.0.0
`))

		if err != nil {
			t.Fatal("Parse failure: ", err)
		}
		if got := deps["."]["@aspect-test/a"]; got != "5.0.2" {
			t.Errorf("A document declaring no dependencies emptied one that did, got: %v", deps["."])
		}
	})

	t.Run("no lockfile version in any document", func(t *testing.T) {
		_, err := parsePnpmLockDependencies(strings.NewReader(`---
settings:
  autoInstallPeers: false
`))
		if err == nil {
			t.Error("Expected an error when no document declares a lockfileVersion")
		}
	})

	t.Run("document separator only", func(t *testing.T) {
		// `---` alone is a valid but empty YAML document, and still not a lockfile.
		// This errored before the multi-document change and continues to; only a
		// genuinely empty file is treated as "no dependencies yet".
		if _, err := parsePnpmLockDependencies(strings.NewReader("---\n")); err == nil {
			t.Error("Expected an error for a lockfile that is only a document separator")
		}
	})
}

// Decode the first YAML document of content and read its lockfileVersion, which is
// what parsePnpmLockDependencies does per document.
func parseVersionOfFirstDocument(t *testing.T, content string) (string, error) {
	t.Helper()

	var document yaml.Node
	if err := yaml.NewDecoder(strings.NewReader(content)).Decode(&document); err != nil && err != io.EOF {
		t.Fatalf("failed to decode %q: %v", content, err)
	}
	return parsePnpmLockVersion(&document)
}
