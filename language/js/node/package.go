package gazelle

import (
	"io"
	"path"
	"slices"
	"strings"

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

	// imports: https://nodejs.org/docs/latest-v22.x/api/packages.html#imports
	Imports any `json:"imports"`

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

	// Subpath imports from the 'imports' field, keyed by the '#'-prefixed
	// specifier. Values are the raw mapping targets: './'-relative files
	// within the package, or external package specifiers. Targets are
	// sorted, JSON condition order is not preserved.
	// Nil if package.json has no 'imports' field.
	Imports map[string][]string
}

func (p *PackageJson) addEntry(file string) {
	p.Entries = append(p.Entries, path.Clean(file))
}

func (p *PackageJson) addImport(specifier, target string) {
	specifier = path.Clean(specifier)
	p.Imports[specifier] = append(p.Imports[specifier], target)

	// Only './'-relative targets are files within the package, otherwise
	// the target is an external package specifier.
	if strings.HasPrefix(target, "./") {
		p.addEntry(target)
	}
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

	// https://nodejs.org/api/packages.html#subpath-imports
	if c.Imports != nil {
		if imports, ok := c.Imports.(map[string]any); ok {
			pkg.Imports = make(map[string][]string)

			for importKey, imprt := range imports {
				switch i := imprt.(type) {
				case string:
					// Regular subpath import
					pkg.addImport(importKey, i)
				case nil:
					// Excluded target, same as 'exports' null targets.
					break
				case map[string]any:
					// Conditional subpath import
					for subIKey, subI := range i {
						switch subI := subI.(type) {
						case string:
							pkg.addImport(importKey, subI)
						default:
							BazelLog.Warnf("Unknown package.json imports.%s.%s type: %T", importKey, subIKey, subI)
						}
					}
				default:
					BazelLog.Warnf("Unknown package.json imports.%s type: %T", importKey, imprt)
				}
			}

			// Conditional import targets are collected in undefined map iteration
			// order. Sort for determinism.
			for _, targets := range pkg.Imports {
				slices.Sort(targets)
			}
		} else {
			BazelLog.Warnf("Unknown package.json imports type: %T", c.Imports)
		}
	}

	return pkg, nil
}
