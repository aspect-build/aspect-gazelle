package node

import (
	"strings"
)

func ParseImportPath(imp string) (string, string) {
	// Imports of local files are never packages
	if imp == "" || imp[0] == '/' || imp[0] == '.' {
		return "", imp
	}

	// Imports of @scoped-package-like paths
	if imp[0] == '@' {
		scopeEnd := strings.IndexByte(imp, '/')
		if scopeEnd == -1 {
			return "", imp
		}
		subPkg := imp[scopeEnd+1:]
		subPkgEnd := strings.IndexByte(subPkg, '/')
		if subPkgEnd == -1 {
			return imp, ""
		}
		return imp[:scopeEnd+subPkgEnd+1], imp[scopeEnd+subPkgEnd+2:]
	}

	// Imports of package-like paths
	before, after, ok := strings.Cut(imp, "/")
	if !ok {
		return imp, ""
	}

	return before, after
}

// IsSubpathImport returns true if the import is a node-style '#' subpath
// import, mapped by the 'imports' field of the importing package.
// See https://nodejs.org/api/packages.html#subpath-imports
func IsSubpathImport(imp string) bool {
	return strings.HasPrefix(imp, "#")
}

func ToAtTypesPackage(pkg string) string {
	// @scoped packages
	if pkg[0] == '@' {
		if i := strings.IndexRune(pkg, '/'); i != -1 {
			return "@types/" + pkg[1:i] + "__" + pkg[i+1:]
		}
		return ""
	}

	// packages with trailing 0
	if i := strings.IndexRune(pkg, '/'); i != -1 {
		return "@types/" + pkg[:i]
	}

	return "@types/" + pkg
}
