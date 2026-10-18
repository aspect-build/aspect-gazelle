package pnpm

import (
	"fmt"
	"io"
	"maps"

	"gopkg.in/yaml.v3"
)

// Lockfile v9 shares the v6 structure, but pnpm 12+ may write multiple yaml
// documents such as the package manager environment followed by the project dependencies.
// See https://github.com/pnpm/pnpm/issues/13805
func parsePnpmLockDependenciesV9(yamlReader io.Reader) (WorkspacePackageVersionMap, error) {
	decoder := yaml.NewDecoder(yamlReader)

	var result WorkspacePackageVersionMap

	for {
		lockfile := PnpmLockfileV6{}
		unmarshalErr := decoder.Decode(&lockfile)
		if unmarshalErr == io.EOF {
			return result, nil
		}
		if unmarshalErr != nil {
			return nil, fmt.Errorf("parse error: %v", unmarshalErr)
		}

		if result == nil {
			result = make(WorkspacePackageVersionMap)
		}

		if lockfile.Importers != nil {
			for pkg, pkgDeps := range lockfile.Importers {
				addDependenciesV9(result, pkg, mergeDependenciesV6(pkgDeps.Dependencies, pkgDeps.DevDependencies, pkgDeps.PeerDependencies, pkgDeps.OptionalDependencies))
			}
		} else {
			addDependenciesV9(result, ".", mergeDependenciesV6(lockfile.Dependencies, lockfile.DevDependencies, lockfile.PeerDependencies, lockfile.OptionalDependencies))
		}
	}
}

func addDependenciesV9(result WorkspacePackageVersionMap, pkg string, deps map[string]string) {
	if existing, exists := result[pkg]; exists {
		maps.Copy(existing, deps)
	} else {
		result[pkg] = deps
	}
}
