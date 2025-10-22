package queries

import (
	"log"

	BazelLog "github.com/aspect-build/aspect-gazelle/common/logger"
	"github.com/aspect-build/aspect-gazelle/common/treesitter"
	treeutils "github.com/aspect-build/aspect-gazelle/common/treesitter"
	"github.com/aspect-build/aspect-gazelle/common/treesitter/grammars/golang"
	"github.com/aspect-build/aspect-gazelle/common/treesitter/grammars/java"
	"github.com/aspect-build/aspect-gazelle/common/treesitter/grammars/json"
	"github.com/aspect-build/aspect-gazelle/common/treesitter/grammars/kotlin"
	"github.com/aspect-build/aspect-gazelle/common/treesitter/grammars/rust"
	"github.com/aspect-build/aspect-gazelle/common/treesitter/grammars/starlark"
	"github.com/aspect-build/aspect-gazelle/common/treesitter/grammars/tsx"
	"github.com/aspect-build/aspect-gazelle/common/treesitter/grammars/typescript"
	"github.com/aspect-build/aspect-gazelle/language/orion/plugin"
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
	if BazelLog.IsLevelEnabled(BazelLog.TraceLevel) {
		treeErrors := ast.QueryErrors()
		if treeErrors != nil {
			BazelLog.Tracef("TreeSitter query errors: %v", treeErrors)
		}
	}

	// TODO: look into running queries in parallel on the same AST
	for key, query := range queries {
		params := query.Params.(plugin.AstQueryParams)
		treeQuery, err := treeutils.GetQuery(lang, params.Query)
		if err != nil {
			return err
		}

		// TODO: delay collection from channel until first read?
		// Then it must be cached for later reads...
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
		return golang.NewLanguage()
	case treesitter.Java:
		return java.NewLanguage()
	case treesitter.JSON:
		return json.NewLanguage()
	case treesitter.Kotlin:
		return kotlin.NewLanguage()
	case treesitter.Rust:
		return rust.NewLanguage()
	case treesitter.Starlark:
		return starlark.NewLanguage()
	case treesitter.Typescript:
		return typescript.NewLanguage()
	case treesitter.TypescriptX:
		return tsx.NewLanguage()
	}

	log.Panicf("Unknown LanguageGrammar %q", lang)
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
