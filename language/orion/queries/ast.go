package queries

import (
	BazelLog "github.com/aspect-build/aspect-gazelle/common/logger"
	"github.com/aspect-build/aspect-gazelle/common/treesitter"
	treeutils "github.com/aspect-build/aspect-gazelle/common/treesitter"
	"github.com/aspect-build/aspect-gazelle/language/orion/plugin"
	"github.com/aspect-build/aspect-gazelle/treesitter/golang"
	"github.com/aspect-build/aspect-gazelle/treesitter/hcl"
	"github.com/aspect-build/aspect-gazelle/treesitter/java"
	"github.com/aspect-build/aspect-gazelle/treesitter/json"
	"github.com/aspect-build/aspect-gazelle/treesitter/kotlin"
	"github.com/aspect-build/aspect-gazelle/treesitter/python"
	"github.com/aspect-build/aspect-gazelle/treesitter/ruby"
	"github.com/aspect-build/aspect-gazelle/treesitter/rust"
	"github.com/aspect-build/aspect-gazelle/treesitter/starlark"
	"github.com/aspect-build/aspect-gazelle/treesitter/tsx"
	"github.com/aspect-build/aspect-gazelle/treesitter/typescript"
)

func runPluginTreeQueries(fileName string, sourceCode []byte, queries plugin.NamedQueries, queryResults chan *plugin.QueryProcessorResult) error {
	lang := toTreeLanguage(fileName, queries)
	ast, err := treeutils.ParseSourceCode(lang, fileName, sourceCode)
	if err != nil {
		return err
	}
	defer ast.Close()

	// Parse errors. Only log them due to many false positives.
	// TODO: what false positives? See js plugin where this is from
	if BazelLog.IsTraceEnabled() {
		treeErrors := ast.QueryErrors()
		if treeErrors != nil {
			BazelLog.Tracef("TreeSitter query errors: %v", treeErrors)
		}
	}

	// Queries must run sequentially on the same AST because go-tree-sitter's
	// Tree.cachedNode uses a plain map that is not safe for concurrent access
	// when collecting AST nodes for captures.
	// NOTE: could potentially split initial query execution vs capture collection?
	for key, query := range queries {
		params := query.Params.(plugin.AstQueryParams)
		treeQuery, err := treeutils.GetQuery(lang, params.Query)
		if err != nil {
			return err
		}

		matches := plugin.QueryMatches(nil)
		for r := range ast.Query(treeQuery) {
			matches = append(matches, plugin.NewQueryMatch(r.Captures(), nil))
		}

		queryResults <- &plugin.QueryProcessorResult{
			Key:    key,
			Result: matches,
		}
	}

	return nil
}

func toTreeLanguage(fileName string, queries plugin.NamedQueries) treesitter.Language {
	lang := toTreeGrammar(fileName, queries)

	switch lang {
	case treesitter.Go:
		return treesitter.NewLanguageFromSitter(treesitter.Go, golang.NewLanguage())
	case treesitter.HCL:
		return treesitter.NewLanguageFromSitter(treesitter.HCL, hcl.NewLanguage())
	case treesitter.Java:
		return treesitter.NewLanguageFromSitter(treesitter.Java, java.NewLanguage())
	case treesitter.JSON:
		return treesitter.NewLanguageFromSitter(treesitter.JSON, json.NewLanguage())
	case treesitter.Kotlin:
		return treesitter.NewLanguageFromSitter(treesitter.Kotlin, kotlin.NewLanguage())
	case treesitter.Python:
		return treesitter.NewLanguageFromSitter(treesitter.Python, python.NewLanguage())
	case treesitter.Ruby:
		return treesitter.NewLanguageFromSitter(treesitter.Ruby, ruby.NewLanguage())
	case treesitter.Rust:
		return treesitter.NewLanguageFromSitter(treesitter.Rust, rust.NewLanguage())
	case treesitter.Starlark:
		return treesitter.NewLanguageFromSitter(treesitter.Starlark, starlark.NewLanguage())
	case treesitter.Typescript:
		return treesitter.NewLanguageFromSitter(treesitter.Typescript, typescript.NewLanguage())
	case treesitter.TypescriptX:
		return treesitter.NewLanguageFromSitter(treesitter.TypescriptX, tsx.NewLanguage())
	}

	BazelLog.Fatalf("Unknown LanguageGrammar %q", lang)
	return nil
}

func toTreeGrammar(fileName string, queries plugin.NamedQueries) treeutils.LanguageGrammar {
	// TODO: fail if queries on the same file use different languages?

	for _, q := range queries {
		grammar := q.Params.(plugin.AstQueryParams).Grammar
		if grammar != "" {
			return treeutils.LanguageGrammar(grammar)
		}
	}

	return treeutils.PathToLanguage(fileName)
}
