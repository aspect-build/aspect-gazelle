package gazelle

import (
	"encoding/gob"
	"fmt"
	"path"
	"sort"
	"strings"

	common "github.com/aspect-build/aspect-gazelle/common"
	"github.com/aspect-build/aspect-gazelle/common/cache"
	BazelLog "github.com/aspect-build/aspect-gazelle/common/logger"
	ruleUtils "github.com/aspect-build/aspect-gazelle/common/rule"
	"github.com/aspect-build/aspect-gazelle/language/kotlin/git"
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

	libTargets := newLibTargetsForPackage(cfg, sourceFiles, args.File)
	binTargets := treemap.NewWith[string, *KotlinBinTarget](strings.Compare)
	testTargets := treemap.NewWith[string, *KotlinTestTarget](strings.Compare)

	// Parse all source files and group information into target(s)
	for p := range kt.parseFiles(args, sourceFiles) {
		// A nil result means the file could not be read (the error was already
		// printed by the parse worker); skip it rather than panic.
		if p == nil {
			continue
		}

		pkgName := ""
		if p.Package != nil {
			pkgName = p.Package.Literal()
		}

		if cfg.IsTestBaseName(path.Base(p.File)) {
			testTarget := NewKotlinTestTarget([]string{p.File}, pkgName, guessClassName(p))
			testTargets.Put(p.File, testTarget)

			for _, impt := range p.Imports {
				testTarget.Imports.Add(ImportStatement{
					ImportSpec: resolve.ImportSpec{
						Lang: LanguageName,
						Imp:  impt.Identifier().Literal(),
					},
					SourcePath: p.File,
				})
			}
		} else if p.HasMain {
			binTarget := NewKotlinBinTarget(p.File, pkgName)
			binTargets.Put(p.File, binTarget)

			for _, impt := range p.Imports {
				binTarget.Imports.Add(ImportStatement{
					ImportSpec: resolve.ImportSpec{
						Lang: LanguageName,
						Imp:  impt.Identifier().Literal(),
					},
					SourcePath: p.File,
				})
			}
		} else {
			if err := libTargets.collectSourceFile(cfg.ExportGranularity(), p); err != nil {
				common.GenerationErrorf(args.Config, "failed to collect library file: %v", err)
			}
		}
	}

	var result language.GenerateResult

	for _, libTarget := range libTargets.allTargets() {
		libTargetName := toDefaultTargetName(args, "root")
		if libTarget.ExistingName != "" {
			libTargetName = libTarget.ExistingName
		}

		srcGenErr := kt.addLibraryRule(libTargetName, libTarget, args, false, &result)
		if srcGenErr != nil {
			common.GenerationErrorf(args.Config, "Source rule generation error: %v", srcGenErr)
		}
	}

	for _, binTarget := range binTargets.Values() {
		binTargetName := toBinaryTargetName(binTarget.File)
		kt.addBinaryRule(binTargetName, binTarget, args, &result)
	}

	for _, testTarget := range testTargets.Values() {
		testTargetName := toTestTargetName(testTarget.Files[0])
		kt.addTestRule(testTargetName, testTarget, args, &result)
	}

	return result
}

type libTargetsForPackage struct {
	cfg                   *kotlinconfig.KotlinConfig
	defaultTarget         *KotlinLibTarget
	existingFileToTargets map[string][]*KotlinLibTarget
}

func newLibTargetsForPackage(cfg *kotlinconfig.KotlinConfig, sourceFiles []string, buildFile *rule.File) *libTargetsForPackage {
	defaultTarget := NewKotlinLibTarget()
	fileToTargets := map[string][]*KotlinLibTarget{}

	if cfg.OnlyUseExistingLibraryTargets() && buildFile != nil {
		for _, rule := range buildFile.Rules {
			if rule.Kind() != KtJvmLibrary {
				continue
			}
			target := NewKotlinLibTarget()
			target.ExistingName = rule.Name()
			for _, file := range rule.AttrStrings("srcs") {
				target.Files.Add(file)
				fileToTargets[file] = append(fileToTargets[file], target)
			}
		}
		defaultTarget = nil
	}

	return &libTargetsForPackage{
		cfg:                   cfg,
		defaultTarget:         defaultTarget,
		existingFileToTargets: fileToTargets,
	}
}

func (lts *libTargetsForPackage) collectSourceFile(exportGranularity kotlinconfig.ExportGranularity, pr *parser.ParseResult) error {
	targets := lts.existingFileToTargets[pr.File]
	if len(targets) == 0 {
		if lts.cfg.OnlyUseExistingLibraryTargets() {
			return fmt.Errorf("failed to process source file %q: OnlyUseExistingLibraryTargets is specified, yet %q doesn't appear in srcs of any %s target", pr.File, pr.File, KtJvmLibrary)
		}
		targets = append(targets, lts.defaultTarget)
	}
	for _, target := range targets {
		target.Files.Add(pr.File)
		switch exportGranularity {
		case kotlinconfig.ExportGranularityPackage:
			if pr.Package != nil && pr.Package.Literal() != "" {
				target.Packages.Add(pr.Package.Literal())
			}
		case kotlinconfig.ExportGranularityTopLevelObjects:
			for _, id := range pr.TopLevelIdentifiers {
				fullyQualifiedId := ""
				if pr.Package == nil || pr.Package.Literal() == "" {
					fullyQualifiedId = id.Literal()
				} else {
					fullyQualifiedId = pr.Package.Literal() + "." + id.Literal()
				}
				target.Packages.Add(fullyQualifiedId)
			}
		default:
		}

		for _, impt := range pr.Imports {
			target.Imports.Add(ImportStatement{
				ImportSpec: resolve.ImportSpec{
					Lang: LanguageName,
					Imp:  impt.Identifier().Literal(),
				},
				SourcePath: pr.File,
			})
		}
	}
	return nil
}

func (lts *libTargetsForPackage) allTargets() []*KotlinLibTarget {
	set := map[*KotlinLibTarget]struct{}{}
	if lts.defaultTarget != nil {
		set[lts.defaultTarget] = struct{}{}
	}
	for _, targets := range lts.existingFileToTargets {
		for _, target := range targets {
			set[target] = struct{}{}
		}
	}
	out := make([]*KotlinLibTarget, 0, len(set))
	for t := range set {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ExistingName < out[j].ExistingName
	})
	return out
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

	// Check for name-collisions with the rule being generated.
	colError := ruleUtils.CheckCollisionErrors(targetName, KtJvmLibrary, sourceRuleKinds, args)
	if colError != nil {
		return colError
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

func (kt *kotlinLang) addTestRule(targetName string, target *KotlinTestTarget, args language.GenerateArgs, result *language.GenerateResult) {
	ktTest := rule.NewRule(KtJvmTest, targetName)
	ktTest.SetAttr("srcs", target.Files)
	ktTest.SetAttr("test_class", target.TestClass)
	ktTest.SetPrivateAttr(packagesKey, target)

	result.Gen = append(result.Gen, ktTest)
	result.Imports = append(result.Imports, target)

	BazelLog.Infof("add rule '%s' '%s:%s'", ktTest.Kind(), args.Rel, ktTest.Name())
}

func guessClassName(p *parser.ParseResult) string {
	lit := strings.TrimSuffix(path.Base(p.File), ".kt")
	if p.Package == nil || p.Package.Literal() == "" {
		return lit
	}
	return p.Package.Literal() + "." + lit
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

	isIgnored := git.GetIgnoreFunction(args.Config)

	for _, f := range args.RegularFiles {
		if isIgnored(path.Join(args.Rel, f)) {
			continue
		}
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
