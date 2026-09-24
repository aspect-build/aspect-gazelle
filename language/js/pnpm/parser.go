package pnpm

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"io"

	semver "github.com/Masterminds/semver/v3"
	"gopkg.in/yaml.v3"
)

type WorkspacePackageVersionMap map[string]map[string]string

func init() {
	gob.Register(WorkspacePackageVersionMap{})
}

/* Parse a lockfile and return a map of workspace projects to a map of dependency name to version.
 */
func ParsePnpmLockFileDependencies(lockfileContent []byte) (WorkspacePackageVersionMap, error) {
	return parsePnpmLockDependencies(bytes.NewReader(lockfileContent))
}

// A lockfile may hold more than one YAML document. pnpm 12 writes its own
// self-management pins (`packageManagerDependencies`) as a leading document and the
// dependency graph as a second one, so the first line of the file is `---` and the
// first document is not the lockfile we care about.
//
// Only the version is read here; the yaml document is handed to the versioned
// parser so it decodes the document the version came from rather than whichever
// one a fresh decoder happens to reach first.
type lockfileVersionProbe struct {
	// A yaml.Node rather than a string: the version is quoted from v6 on ('6.0')
	// but bare in v5 (5.4), and Node.Value carries the scalar text either way.
	LockfileVersion yaml.Node `yaml:"lockfileVersion"`
}

func parsePnpmLockVersion(document *yaml.Node) (string, error) {
	probe := lockfileVersionProbe{}
	if err := document.Decode(&probe); err != nil {
		return "", fmt.Errorf("failed to read lockfile version: %v", err)
	}
	return probe.LockfileVersion.Value, nil
}

func parsePnpmLockDocument(document *yaml.Node, versionStr string) (WorkspacePackageVersionMap, error) {
	version, versionErr := semver.NewVersion(versionStr)
	if versionErr != nil {
		return nil, fmt.Errorf("failed to parse semver %q: %v", versionStr, versionErr)
	}

	switch version.Major() {
	case 5:
		return parsePnpmLockDependenciesV5(document)
	case 6:
		return parsePnpmLockDependenciesV6(document)
	case 9:
		return parsePnpmLockDependenciesV9(document)
	}

	return nil, fmt.Errorf("unsupported version: %v", versionStr)
}

func parsePnpmLockDependencies(yamlFileReader io.Reader) (WorkspacePackageVersionMap, error) {
	decoder := yaml.NewDecoder(yamlFileReader)

	var result WorkspacePackageVersionMap
	versioned := 0
	documents := 0

	for {
		var document yaml.Node
		decodeErr := decoder.Decode(&document)
		if decodeErr == io.EOF {
			break
		}
		if decodeErr != nil {
			return nil, fmt.Errorf("parse error: %v", decodeErr)
		}
		documents++

		// A document declaring no lockfileVersion carries no dependency graph.
		versionStr, versionErr := parsePnpmLockVersion(&document)
		if versionErr != nil {
			return nil, versionErr
		}
		if versionStr == "" {
			continue
		}
		versioned++

		documentResult, documentErr := parsePnpmLockDocument(&document, versionStr)
		if documentErr != nil {
			return nil, documentErr
		}
		if documentResult == nil {
			continue
		}

		if result == nil {
			result = documentResult
			continue
		}
		mergeDocumentDependencies(result, documentResult)
	}

	if documents == 0 {
		return nil, nil
	}
	if versioned == 0 {
		return nil, fmt.Errorf("failed to find lockfile version in any of the %d document(s) in the lockfile", documents)
	}

	return result, nil
}

// Fold one document's dependencies into the accumulated result.
//
// An importer can appear in more than one document - pnpm 12 lists the root
// importer in its self-management document as well as in the lockfile proper - so
// we union them. An entry carrying a version will win.
func mergeDocumentDependencies(into, from WorkspacePackageVersionMap) {
	for importer, deps := range from {
		existing, seen := into[importer]
		if !seen {
			into[importer] = deps
			continue
		}
		for name, version := range deps {
			if _, ok := existing[name]; !ok {
				existing[name] = version
			}
		}
	}
}
