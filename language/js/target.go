package gazelle

import (
	"path"
	"strings"

	"github.com/bazelbuild/bazel-gazelle/label"
	"github.com/bazelbuild/bazel-gazelle/resolve"
	"github.com/emirpasic/gods/v2/sets/treeset"
)

// ImportStatement represents an ImportSpec imported from a source file.
// Imports can be of any form (es6, cjs, amd, ...).
// Imports may be relative ot the source, absolute, workspace, named modules etc.
type ImportStatement struct {
	resolve.ImportSpec

	// The path of the file containing the import
	SourcePath string

	// The path as written in the import statement
	ImportPath string

	// The type of import which produced this statement
	Kind ImportKind

	// If the import is optional and failure to resolve should not be an error
	Optional bool

	// If the import is explicitly for types, in which case prefer @types package
	// dependencies when types are shipped separately
	TypesOnly bool
}

type ImportKind string

const (
	ImportKindImport ImportKind = "import"
	ImportKindJsx    ImportKind = "jsx"
	ImportKindURL    ImportKind = "url"
)

// Npm link-all rule import data
type LinkAllPackagesImports struct{}

func newLinkAllPackagesImports() *LinkAllPackagesImports {
	return &LinkAllPackagesImports{}
}

type TsPackageInfo struct {
	TsProjectInfo

	source *label.Label
}

func newTsPackageInfo(source *label.Label) *TsPackageInfo {
	return &TsPackageInfo{
		TsProjectInfo: TsProjectInfo{
			imports: treeset.NewWith(importStatementComparator),
			sources: treeset.NewWith(strings.Compare),
		},
		source: source,
	}
}

// TsProject rule import data
type TsProjectInfo struct {
	// `ImportStatement`s in ths project
	imports *treeset.Set[ImportStatement]

	// The 'srcs' of this project
	sources *treeset.Set[string]
}

func newTsProjectInfo() *TsProjectInfo {
	return &TsProjectInfo{
		imports: treeset.NewWith(importStatementComparator),
		sources: treeset.NewWith(strings.Compare),
	}
}
func (i *TsProjectInfo) AddImport(impt ImportStatement) {
	i.imports.Add(impt)
}

func (i *TsProjectInfo) HasTsx() bool {
	if i.sources != nil {
		for it := i.sources.Iterator(); it.Next(); {
			if isTsxFileExt(path.Ext(it.Value())) {
				return true
			}
		}
	}

	return false
}

// importStatementComparator compares modules by name.
func importStatementComparator(a, b ImportStatement) int {
	return strings.Compare(a.Imp, b.Imp)
}
