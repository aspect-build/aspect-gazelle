package node

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

	// Exact (non-pattern) subpath exports from the 'exports' field, keyed by normalized subpath, mapped to their sorted target files.
	Exports map[string][]string

	// '*' subpath export patterns from the 'exports' field, sorted by resolution priority.
	ExportPatterns []SubpathPattern

	// Exact (non-pattern) subpath imports from the 'imports' field, mapped to their sorted targets.
	Imports map[string][]string

	// '*' subpath import patterns from the 'imports' field, sorted by resolution priority.
	ImportPatterns []SubpathPattern
}

// SubpathPattern is a '*' subpath pattern from the 'exports' or 'imports'
// field such as "#internal/*": "./src/internal/*.js", split around the '*'.
// See https://nodejs.org/api/packages.html#subpath-patterns
type SubpathPattern struct {
	Prefix, Suffix string

	// The sorted mapping targets, '*'s not yet substituted.
	Targets []string
}

// ResolveImport resolves a subpath import specifier to its mapping targets,
// matching and expanding '*' subpath patterns.
func (p *PackageJson) ResolveImport(specifier string) []string {
	return resolveSubpath(p.Imports, p.ImportPatterns, specifier)
}

// ResolveExport resolves a normalized 'exports' subpath to its mapping target files, matching and expanding '*' subpath patterns.
func (p *PackageJson) ResolveExport(subpath string) []string {
	return resolveSubpath(p.Exports, p.ExportPatterns, subpath)
}

func resolveSubpath(exact map[string][]string, patterns []SubpathPattern, specifier string) []string {
	if targets, found := exact[specifier]; found {
		return targets
	}

	for _, pat := range patterns {
		if len(specifier) <= len(pat.Prefix)+len(pat.Suffix) || !strings.HasPrefix(specifier, pat.Prefix) || !strings.HasSuffix(specifier, pat.Suffix) {
			continue
		}

		// Substitute the pattern match into the targets.
		matched := specifier[len(pat.Prefix) : len(specifier)-len(pat.Suffix)]
		targets := make([]string, len(pat.Targets))
		for i, target := range pat.Targets {
			targets[i] = strings.ReplaceAll(target, "*", matched)
		}
		return targets
	}

	return nil
}

// indexSubpaths splits raw subpath mappings into exact specifiers and sorted '*' patterns; field ("exports"/"imports") labels warnings.
func indexSubpaths(raw map[string][]string, field string) (map[string][]string, []SubpathPattern) {
	var exact map[string][]string
	var patterns []SubpathPattern

	for key, targets := range raw {
		slices.Sort(targets)

		prefix, suffix, isPattern := strings.Cut(key, "*")
		if !isPattern {
			if exact == nil {
				exact = make(map[string][]string, len(raw))
			}
			exact[key] = targets
		} else if strings.Contains(suffix, "*") {
			BazelLog.Warnf("Invalid package.json %s key %q: multiple '*'s", field, key)
		} else {
			patterns = append(patterns, SubpathPattern{Prefix: prefix, Suffix: suffix, Targets: targets})
		}
	}

	// Node resolution priority (PATTERN_KEY_COMPARE): the longest prefix,
	// then the longest suffix. Equal-length patterns can not match the same
	// specifier; order those lexicographically for determinism.
	slices.SortFunc(patterns, func(a, b SubpathPattern) int {
		if d := len(b.Prefix) - len(a.Prefix); d != 0 {
			return d
		}
		if d := len(b.Suffix) - len(a.Suffix); d != 0 {
			return d
		}
		if d := strings.Compare(a.Prefix, b.Prefix); d != 0 {
			return d
		}
		return strings.Compare(a.Suffix, b.Suffix)
	})

	return exact, patterns
}

func (p *PackageJson) addEntry(file string) {
	p.Entries = append(p.Entries, path.Clean(file))
}

// exportSubpath normalizes an 'exports' subpath so the package root "." is ""
// and subpaths have no leading "./".
func exportSubpath(subpath string) string {
	if subpath = path.Clean(subpath); subpath == "." {
		return ""
	}
	return subpath
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
		// Raw subpath -> target files, keyed by normalized subpath.
		rawExports := make(map[string][]string)
		addExport := func(subpath, file string) {
			file = path.Clean(file)
			subpath = exportSubpath(subpath)
			rawExports[subpath] = append(rawExports[subpath], file)
			pkg.addEntry(file)
		}

		switch exports := c.Exports.(type) {
		case string:
			// Single export
			addExport(".", exports)
		case map[string]any:
			// Subpath exports. Keys not starting with "." are conditions
			// ("node", "default" etc.) of the package root export.
			for exportKey, export := range exports {
				subpath := exportKey
				if !strings.HasPrefix(exportKey, ".") {
					subpath = "."
				}

				switch e := export.(type) {
				case string:
					// Regular subpath export
					addExport(subpath, e)
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
							addExport(subpath, subE)
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
					addExport(".", subE)
				default:
					BazelLog.Warnf("Unknown package.json exports[%v] type: %T", i, subE)
				}
			}
		default:
			BazelLog.Warnf("Unknown package.json exports type: %T", exports)
		}

		// Index the raw mappings for resolution: exact subpaths split from
		// '*' patterns, targets sorted and patterns ordered by priority.
		pkg.Exports, pkg.ExportPatterns = indexSubpaths(rawExports, "exports")
	}

	// https://nodejs.org/api/packages.html#subpath-imports
	if c.Imports != nil {
		if imports, ok := c.Imports.(map[string]any); ok {
			rawImports := make(map[string][]string)
			addImport := func(specifier, target string) {
				specifier = path.Clean(specifier)
				rawImports[specifier] = append(rawImports[specifier], target)

				// Only './'-relative targets are files within the package, otherwise
				// the target is an external package specifier.
				if strings.HasPrefix(target, "./") {
					pkg.addEntry(target)
				}
			}

			for importKey, imprt := range imports {
				switch i := imprt.(type) {
				case string:
					// Regular subpath import
					addImport(importKey, i)
				case nil:
					// Excluded target, same as 'exports' null targets.
					break
				case map[string]any:
					// Conditional subpath import
					for subIKey, subI := range i {
						switch subI := subI.(type) {
						case string:
							addImport(importKey, subI)
						default:
							BazelLog.Warnf("Unknown package.json imports.%s.%s type: %T", importKey, subIKey, subI)
						}
					}
				default:
					BazelLog.Warnf("Unknown package.json imports.%s type: %T", importKey, imprt)
				}
			}

			// Index the raw mappings for resolution: exact specifiers split
			// from '*' patterns, targets sorted and patterns ordered by priority.
			pkg.Imports, pkg.ImportPatterns = indexSubpaths(rawImports, "imports")
		} else {
			BazelLog.Warnf("Unknown package.json imports type: %T", c.Imports)
		}
	}

	return pkg, nil
}
