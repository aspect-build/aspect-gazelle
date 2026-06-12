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

func runPluginTreeQueries(fileName string, sourceCode []byte, queries plugin.NamedQueries) (plugin.QueryResults, error) {
	lang := toTreeLanguage(fileName, queries)
	ast, err := treeutils.ParseSourceCode(lang, fileName, sourceCode)
	if err != nil {
		return nil, err
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
	// Tree.cachedNode uses a plain map that is not safe for concurrent access.
	// The unsafe write happens inside QueryCursor.NextMatch (it caches a *Node
	// per capture), so deferring capture collection would not make it safe.
	results := make(plugin.QueryResults, len(queries))
	for key, query := range queries {
		treeQuery, err := treeutils.GetQuery(lang, query.(*plugin.AstQuery).Query)
		if err != nil {
			return nil, err
		}

		matches := plugin.QueryMatches(nil)
		for captures := range ast.Query(treeQuery) {
			matches = append(matches, plugin.NewQueryMatch(captures, nil))
		}

		results[key] = matches
	}

	return results, nil
}

func toTreeLanguage(fileName string, queries plugin.NamedQueries) treesitter.Language {
	lang := toTreeGrammar(fileName, queries)

	switch lang {
	case treesitter.Go:
		return treesitter.NewLanguage(treesitter.Go, golang.LanguagePtr())
	case treesitter.HCL:
		return treesitter.NewLanguage(treesitter.HCL, hcl.LanguagePtr())
	case treesitter.Java:
		return treesitter.NewLanguage(treesitter.Java, java.LanguagePtr())
	case treesitter.JSON:
		return treesitter.NewLanguage(treesitter.JSON, json.LanguagePtr())
	case treesitter.Kotlin:
		return treesitter.NewLanguage(treesitter.Kotlin, kotlin.LanguagePtr())
	case treesitter.Python:
		return treesitter.NewLanguage(treesitter.Python, python.LanguagePtr())
	case treesitter.Ruby:
		return treesitter.NewLanguage(treesitter.Ruby, ruby.LanguagePtr())
	case treesitter.Rust:
		return treesitter.NewLanguage(treesitter.Rust, rust.LanguagePtr())
	case treesitter.Starlark:
		return treesitter.NewLanguage(treesitter.Starlark, starlark.LanguagePtr())
	case treesitter.Typescript:
		return treesitter.NewLanguage(treesitter.Typescript, typescript.LanguagePtr())
	case treesitter.TypescriptX:
		return treesitter.NewLanguage(treesitter.TypescriptX, tsx.LanguagePtr())
	}

	BazelLog.Fatalf("Unknown LanguageGrammar %q", lang)
	return nil
}

func toTreeGrammar(fileName string, queries plugin.NamedQueries) treeutils.LanguageGrammar {
	// TODO: fail if queries on the same file use different languages?

	for _, q := range queries {
		grammar := q.(*plugin.AstQuery).Grammar
		if grammar != "" {
			return treeutils.LanguageGrammar(grammar)
		}
	}

	return treeutils.PathToLanguage(fileName)
}
