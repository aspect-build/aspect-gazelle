package gazelle

import (
	"encoding/gob"
	"fmt"
	"path"
	"strings"

	common "github.com/aspect-build/aspect-gazelle/common"
	"github.com/aspect-build/aspect-gazelle/common/cache"
	BazelLog "github.com/aspect-build/aspect-gazelle/common/logger"
	ruleUtils "github.com/aspect-build/aspect-gazelle/common/rule"
	"github.com/aspect-build/aspect-gazelle/language/kotlin/kotlinconfig"
	"github.com/aspect-build/aspect-gazelle/language/kotlin/parser"
	"github.com/bazelbuild/bazel-gazelle/language"
	"github.com/bazelbuild/bazel-gazelle/resolve"
	"github.com/bazelbuild/bazel-gazelle/rule"
	"github.com/emirpasic/gods/v2/maps/treemap"
)

func init() {
	// TODO: don't expose 'gob' cache serialization here
	gob.Register(parser.ParseResult{})
}

func (kt *kotlinLang) GenerateRules(args language.GenerateArgs) language.GenerateResult {
	// TODO: record args.GenFiles labels?

	cfg := args.Config.Exts[LanguageName].(kotlinconfig.Configs)[args.Rel]

	// When we return empty, we mean that we don't generate anything, but this
	// still triggers the indexing for all the TypeScript targets in this package.
	if !cfg.GenerationEnabled() {
		BazelLog.Tracef("GenerateRules(%s) disabled: %s", LanguageName, args.Rel)
		return language.GenerateResult{}
	}

	BazelLog.Tracef("GenerateRules(%s): %s", LanguageName, args.Rel)

	// Collect all source files.
	sourceFiles := kt.collectSourceFiles(cfg, args)

	// TODO: multiple library targets (lib, test, ...)
	libTarget := NewKotlinLibTarget()
	binTargets := treemap.NewWith[string, *KotlinBinTarget](strings.Compare)

	// Parse all source files and group information into target(s)
	for p := range kt.parseFiles(args, sourceFiles) {
		// A nil result means the file could not be read (the error was already
		// printed by the parse worker); skip it rather than panic.
		if p == nil {
			continue
		}

		var target *KotlinTarget

		pkgName := ""
		if p.Package != nil {
			pkgName = p.Package.Literal()
		}

		if p.HasMain {
			binTarget := NewKotlinBinTarget(p.File, pkgName)
			binTargets.Put(p.File, binTarget)

			target = &binTarget.KotlinTarget
		} else {
			libTarget.Files.Add(p.File)
			if pkgName != "" {
				libTarget.Packages.Add(pkgName)
			}

			target = &libTarget.KotlinTarget
		}

		for _, impt := range p.Imports {
			target.Imports.Add(ImportStatement{
				ImportSpec: resolve.ImportSpec{
					Lang: LanguageName,
					Imp:  impt.Identifier.Literal(),
				},
				SourcePath: p.File,
			})
		}
	}

	var result language.GenerateResult

	libTargetName := toDefaultTargetName(args, "root")

	srcGenErr := kt.addLibraryRule(libTargetName, libTarget, args, false, &result)
	if srcGenErr != nil {
		common.GenerationErrorf(args.Config, "Source rule generation error: %v", srcGenErr)
	}

	for _, binTarget := range binTargets.Values() {
		binTargetName := toBinaryTargetName(binTarget.File)
		kt.addBinaryRule(binTargetName, binTarget, args, &result)
	}

	return result
}

func toDefaultTargetName(args language.GenerateArgs, defaultRootName string) string {
	// The workspace root may be the version control root and non-deterministic
	if args.Rel == "" {
		if args.Config.RepoName != "" {
			return args.Config.RepoName
		} else {
			return defaultRootName
		}
	}

	return path.Base(args.Dir)
}

func (kt *kotlinLang) addLibraryRule(targetName string, target *KotlinLibTarget, args language.GenerateArgs, isTestRule bool, result *language.GenerateResult) error {
	// Check for name-collisions with the rule being generated.
	colError := ruleUtils.CheckCollisionErrors(targetName, KtJvmLibrary, sourceRuleKinds, args)
	if colError != nil {
		return colError
	}

	// Generate nothing if there are no source files. Remove any existing rules.
	if target.Files.Empty() {
		if args.File == nil {
			return nil
		}

		for _, r := range args.File.Rules {
			if r.Name() == targetName && r.Kind() == KtJvmLibrary {
				emptyRule := rule.NewRule(KtJvmLibrary, targetName)
				result.Empty = append(result.Empty, emptyRule)
				return nil
			}
		}

		return nil
	}

	ktLibrary := rule.NewRule(KtJvmLibrary, targetName)
	ktLibrary.SetAttr("srcs", target.Files.Values())
	ktLibrary.SetPrivateAttr(packagesKey, target)

	if isTestRule {
		ktLibrary.SetAttr("testonly", true)
	}

	result.Gen = append(result.Gen, ktLibrary)
	result.Imports = append(result.Imports, target)

	BazelLog.Infof("add rule '%s' '%s:%s'", ktLibrary.Kind(), args.Rel, ktLibrary.Name())
	return nil
}

func (kt *kotlinLang) addBinaryRule(targetName string, target *KotlinBinTarget, args language.GenerateArgs, result *language.GenerateResult) {
	main_class := strings.TrimSuffix(target.File, ".kt")
	if target.Package != "" {
		main_class = target.Package + "." + main_class
	}

	ktBinary := rule.NewRule(KtJvmBinary, targetName)
	ktBinary.SetAttr("srcs", []string{target.File})
	ktBinary.SetAttr("main_class", main_class)
	ktBinary.SetPrivateAttr(packagesKey, target)

	result.Gen = append(result.Gen, ktBinary)
	result.Imports = append(result.Imports, target)

	BazelLog.Infof("add rule '%s' '%s:%s'", ktBinary.Kind(), args.Rel, ktBinary.Name())
}

func (kt *kotlinLang) parseFiles(args language.GenerateArgs, sources []string) chan *parser.ParseResult {
	parserCache := cache.Get(args.Config)
	rootDir := args.Config.RepoRoot
	rel := args.Rel

	return common.Parallelize(sources, func(sourcePath string) *parser.ParseResult {
		r, err := parseFile(parserCache, rootDir, rel, sourcePath)

		// Output errors to stdout
		if err != nil {
			fmt.Println(sourcePath, "parse error:", err)
		}
		if r != nil && len(r.Errors) > 0 {
			fmt.Println(sourcePath, "parse error(s):")
			for _, e := range r.Errors {
				fmt.Println(e)
			}
		}

		return r
	})
}

// Parse the passed file for import statements, caching the result
func parseFile(parserCache cache.Cache, rootDir, rel, sourcePath string) (*parser.ParseResult, error) {
	BazelLog.Tracef("ParseImports(%s): %s", LanguageName, sourcePath)

	var result *parser.ParseResult
	r, _, err := parserCache.LoadOrStoreFile(rootDir, path.Join(rel, sourcePath), "kotlin.Parse", func(_ string, content []byte) (any, error) {
		// The parse-relative file name (not the repo-relative cache path) is
		// stored on the result, matching how targets reference their srcs.
		// Parse records its errors onto result.Errors, which survives caching.
		p, _ := parser.NewParser().Parse(sourcePath, content)
		return *p, nil
	})

	if r != nil {
		p := r.(parser.ParseResult)
		result = &p
	}

	return result, err
}

func (kt *kotlinLang) collectSourceFiles(cfg *kotlinconfig.KotlinConfig, args language.GenerateArgs) []string {
	sourceFiles := []string{}

	// TODO: "module" targets similar to java?

	for _, f := range args.RegularFiles {
		// Otherwise the file is either source or potentially importable.
		if isSourceFileType(f) {
			BazelLog.Tracef("SourceFile: %s", f)

			sourceFiles = append(sourceFiles, f)
		}
	}

	return sourceFiles
}

func isSourceFileType(f string) bool {
	ext := path.Ext(f)
	return ext == ".kt" || ext == ".kts"
}
