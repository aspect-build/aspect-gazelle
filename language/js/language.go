package gazelle

import (
	pnpm "github.com/aspect-build/aspect-gazelle/language/js/pnpm"
	"github.com/aspect-build/aspect-gazelle/language/js/typescript"
	"github.com/bazelbuild/bazel-gazelle/label"
	"github.com/bazelbuild/bazel-gazelle/language"
)

const LanguageName = "js"

var _ language.Language = (*typeScriptLang)(nil)

// The Gazelle extension for TypeScript rules.
// TypeScript satisfies the language.Language interface including the
// Configurer and Resolver types.
type typeScriptLang struct {
	// Importable files and the generating label.
	fileLabels map[string]*label.Label

	// Importable type definitions and the generating labels.
	// Multiple labels may define/extend the same type definition, potentially also extending packages.
	moduleTypes map[string][]*label.Label

	// Importable npm-like packages. Each pnpm project has its own set
	// of importable npm packages.
	// BUILDs alongside pnpm project roots have a map. BUILDs within a project contain a reference
	// to the parent pnpm project map.
	pnpmProjects *pnpm.PnpmProjectMap

	// Directories containing a package.json file.
	// Possibly pnpm projects, possibly just package.json files.
	packageJsonDirs map[string]struct{}

	// TypeScript configuration across the workspace
	tsconfig *typescript.TsWorkspace
}

var _ language.Language = (*typeScriptLang)(nil)
var _ language.ModuleAwareLanguage = (*typeScriptLang)(nil)

// NewLanguage initializes a new TypeScript that satisfies the language.Language
// interface. This is the entrypoint for the extension initialization.
func NewLanguage() language.Language {
	pnpmProjects := pnpm.NewPnpmProjectMap()
	packageJsonDirs := make(map[string]struct{})

	return &typeScriptLang{
		fileLabels:      make(map[string]*label.Label),
		moduleTypes:     make(map[string][]*label.Label),
		pnpmProjects:    pnpmProjects,
		packageJsonDirs: packageJsonDirs,
		tsconfig: typescript.NewTsWorkspace(pnpmProjects, func(rel string) bool {
			_, found := packageJsonDirs[rel]
			return found
		}),
	}
}
