package gazelle

import (
	"io"
	"path"

	BazelLog "github.com/aspect-build/aspect-gazelle/common/logger"
	"github.com/goexlib/jsonc"
)

type npmPackageJSON struct {
	// name: https://nodejs.org/docs/latest-v22.x/api/packages.html#name
	Name string `json:"name"`

	// main: https://nodejs.org/docs/latest-v22.x/api/packages.html#main
	Main string `json:"main"`

	// exports: https://nodejs.org/docs/latest-v22.x/api/packages.html#exports
	Exports any `json:"exports"`

	// types/typings: https://www.typescriptlang.org/docs/handbook/declaration-files/publishing.html#including-declarations-in-your-npm-package
	Types   string `json:"types"`
	Typings string `json:"typings"`
}

// PackageJson is the package.json data relevant to gazelle such as the
// package name and entry point fields ('main', 'exports' etc).
type PackageJson struct {
	// The package "name" field.
	Name string

	// All entry point files such as the 'main' and 'exports' fields.
	Entries []string
}

func (p *PackageJson) addEntry(file string) {
	p.Entries = append(p.Entries, path.Clean(file))
}

// Extract the package metadata from the package.json file such as the
// package name and the various entry point fields such as 'main' and 'exports'.
func ParsePackageJson(packageJsonReader io.Reader) (PackageJson, error) {
	pkg := PackageJson{}

	packageJsonData, err := io.ReadAll(packageJsonReader)
	if err != nil {
		return pkg, err
	}

	var c npmPackageJSON
	if err := jsonc.Unmarshal(packageJsonData, &c); err != nil {
		return pkg, err
	}

	pkg.Name = c.Name

	if c.Main != "" {
		pkg.addEntry(c.Main)
	}
	if c.Types != "" {
		pkg.addEntry(c.Types)
	}
	if c.Typings != "" {
		pkg.addEntry(c.Typings)
	}

	// https://nodejs.org/api/packages.html#exports
	if c.Exports != nil {
		switch exports := c.Exports.(type) {
		case string:
			// Single export
			pkg.addEntry(exports)
		case map[string]any:
			// Subpath exports
			for exportKey, export := range exports {
				switch e := export.(type) {
				case string:
					// Regular subpath export
					pkg.addEntry(e)
				case nil:
					// According to https://nodejs.org/api/packages.html#subpath-patterns, to exclude
					// private subfolders from patterns, null targets can be used:
					// {
					//   "exports": {
					// 	   "./features/*.js": "./src/features/*.js",
					// 	   "./features/private-internal/*": null
					// 	 }
					// }
					break
				case map[string]any:
					// Conditional subpath export
					for subEKey, subE := range e {
						switch subE := subE.(type) {
						case string:
							pkg.addEntry(subE)
						default:
							BazelLog.Warnf("Unknown package.json exports.%s.%s type: %T", exportKey, subEKey, subE)
						}
					}
				default:
					BazelLog.Warnf("Unknown package.json exports.%s type: %T", exportKey, export)
				}
			}
		case []any:
			// Array of subpath exports
			for i, subE := range exports {
				switch subE := subE.(type) {
				case string:
					pkg.addEntry(subE)
				default:
					BazelLog.Warnf("Unknown package.json exports[%v] type: %T", i, subE)
				}
			}
		default:
			BazelLog.Warnf("Unknown package.json exports type: %T", exports)
		}
	}

	return pkg, nil
}
