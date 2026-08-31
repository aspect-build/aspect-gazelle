package pnpm

import "gopkg.in/yaml.v3"

func parsePnpmLockDependenciesV9(document *yaml.Node) (WorkspacePackageVersionMap, error) {
	// The top-level lockfile object is the same as v6 for the WorkspacePackageVersionMap requirements
	return parsePnpmLockDependenciesV6(document)
}
