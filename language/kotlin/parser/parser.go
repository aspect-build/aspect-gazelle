package parser

import (
	"regexp"
	"strings"

	BazelLog "github.com/aspect-build/aspect-gazelle/common/logger"
	"github.com/aspect-build/aspect-gazelle/common/treesitter/grammars/kotlin"

	treeutils "github.com/aspect-build/aspect-gazelle/common/treesitter"
)

type ParseResult struct {
	File    string
	Imports []string
	Package string
	HasMain bool
}

type Parser interface {
	Parse(filePath string, source []byte) (*ParseResult, []error)
}

type treeSitterParser struct {
	Parser
}

func NewParser() Parser {
	p := treeSitterParser{}

	return &p
}

const importsQuery = `
	(import_header
		(identifier) @from
		(wildcard_import)? @from-wild
	)

	(package_header
		(identifier) @package
	)

	(source_file
		(function_declaration
			(simple_identifier) @equals-main
		)

		(#eq? @equals-main "main")
	)
`

var (
	kotlinPackageRe = regexp.MustCompile(`(?m)^\s*package\s+([A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)*)`)
	kotlinImportRe  = regexp.MustCompile(`(?m)^\s*import\s+([A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)*(?:\.\*)?)(?:\s+as\s+[A-Za-z_][A-Za-z0-9_]*)?(?:\s*//.*)?$`)
	kotlinMainRe    = regexp.MustCompile(`(?m)\bfun\s+main\s*\(`)
)

func (p *treeSitterParser) Parse(filePath string, sourceCode []byte) (*ParseResult, []error) {
	var result = &ParseResult{
		File:    filePath,
		Imports: []string{},
	}

	var errs []error
	seenImports := make(map[string]bool)
	addImport := func(from string) {
		if from == "" || seenImports[from] {
			return
		}
		seenImports[from] = true
		result.Imports = append(result.Imports, from)
	}

	lang := kotlin.NewLanguage()
	tree, err := treeutils.ParseSourceCode(lang, filePath, sourceCode)
	if err != nil {
		errs = append(errs, err)
	}

	if tree != nil {
		defer tree.Close()

		q, err := treeutils.GetQuery(lang, importsQuery)
		if err != nil {
			BazelLog.Fatalf("Failed to create kotlin 'importsQuery': %v", err)
		}
		for queryResult := range tree.Query(q) {
			BazelLog.Tracef("Kotlin AST Query %q: %v", filePath, queryResult)

			caps := queryResult.Captures()
			if from, isFrom := caps["from"]; isFrom {
				if _, isFromWild := caps["from-wild"]; !isFromWild {
					if lastDot := strings.LastIndex(from, "."); lastDot != -1 {
						from = from[:lastDot]
					}
				}
				addImport(from)
			} else if pkg, isPackage := caps["package"]; isPackage {
				if result.Package != "" {
					BazelLog.Fatalf("Multiple package declarations found in %q: %s and %s", filePath, result.Package, pkg)
				}

				result.Package = pkg
			} else if _, isMain := caps["equals-main"]; isMain {
				result.HasMain = true
			} else {
				BazelLog.Fatalf("Unexpected query result for %q: %v", filePath, queryResult)
			}
		}

		if BazelLog.IsTraceEnabled() {
			treeErrors := tree.QueryErrors()
			if treeErrors != nil {
				BazelLog.Tracef("Kotlin TreeSitter query errors: %v", treeErrors)
			}
		}
	}

	applyTextFallbacks(result, sourceCode, addImport)

	return result, errs
}

func applyTextFallbacks(result *ParseResult, sourceCode []byte, addImport func(string)) {
	source := string(sourceCode)
	if result.Package == "" {
		if m := kotlinPackageRe.FindStringSubmatch(source); len(m) == 2 {
			result.Package = m[1]
		}
	}
	if !result.HasMain {
		result.HasMain = kotlinMainRe.MatchString(source)
	}
	for _, m := range kotlinImportRe.FindAllStringSubmatch(source, -1) {
		if len(m) != 2 {
			continue
		}
		from := m[1]
		if strings.HasSuffix(from, ".*") {
			from = strings.TrimSuffix(from, ".*")
		} else if lastDot := strings.LastIndex(from, "."); lastDot != -1 {
			from = from[:lastDot]
		}
		addImport(from)
	}
}
