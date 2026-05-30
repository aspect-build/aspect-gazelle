/*
 * Copyright 2023 Aspect Build Systems, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package treesitter

import (
	"fmt"
	"iter"
	"path"
	"sync"

	BazelLog "github.com/aspect-build/aspect-gazelle/common/logger"
	sitter "github.com/odvcencio/gotreesitter"
)

// Background tree release channel. Trees are sent here from Close() and
// released by a pool of goroutines so arena teardown does not block parsing.
var treeReleaseCh = make(chan *sitter.Tree, 128)

func init() {
	for range 3 {
		go func() {
			for t := range treeReleaseCh {
				t.Release()
			}
		}()
	}
}

type LanguageGrammar string

const (
	Kotlin      LanguageGrammar = "kotlin"
	Starlark                    = "starlark"
	Typescript                  = "typescript"
	TypescriptX                 = "tsx"
	JSON                        = "json"
	Java                        = "java"
	Go                          = "go"
	Rust                        = "rust"
	Ruby                        = "ruby"
	HCL                         = "hcl"
	Python                      = "python"
)

type Language any

// languageCache memoizes treeLanguage wrappers per *sitter.Language so the
// ParserPool inside the wrapper survives across calls. Per-language bindings
// (tsx.NewLanguage(), typescript.NewLanguage(), …) invoke NewLanguage on
// every parse, so without this dedupe the pool would be recreated empty
// every time.
var languageCache sync.Map // key: *sitter.Language → *treeLanguage

func NewLanguage(grammar LanguageGrammar, lang *sitter.Language) Language {
	if cached, ok := languageCache.Load(lang); ok {
		return cached.(*treeLanguage)
	}
	tl := &treeLanguage{
		grammar: grammar,
		lang:    lang,
		// ParserPool is concurrency-safe and scrubs request-local state
		// (reuseCursor/reuseScratch node refs, recoveryParser) on release.
		// One pool per language, held for the process lifetime.
		pool: sitter.NewParserPool(lang, sitter.WithParserPoolTimeoutMicros(parseTimeoutMicros)),
	}
	actual, _ := languageCache.LoadOrStore(lang, tl)
	return actual.(*treeLanguage)
}

type treeLanguage struct {
	grammar LanguageGrammar
	lang    *sitter.Language

	// pool amortizes gotreesitter's per-language table preprocessing
	// (buildSmallLookup, buildRecoverActionsByState, …) across parses.
	pool *sitter.ParserPool
}

func (tree *treeLanguage) String() string {
	return fmt.Sprintf("treeLanguage{grammar: %q}", tree.grammar)
}

type ASTQueryResult interface {
	Captures() map[string]string
}

type AST interface {
	Query(query TreeQuery) iter.Seq[ASTQueryResult]
	QueryErrors() []error

	// Release all resources related to this AST.
	// The AST is most likely no longer usable after this call.
	Close()
}
type treeAst struct {
	lang       *treeLanguage
	filePath   string
	sourceCode []byte

	sitterTree *sitter.Tree
}

var _ AST = (*treeAst)(nil)

func (tree *treeAst) Close() {
	t := tree.sitterTree
	tree.sitterTree = nil
	tree.sourceCode = nil
	if t != nil {
		// Pass the tree to the background deletion channel and nil out the reference here
		treeReleaseCh <- t
	}
}

func (tree *treeAst) String() string {
	return fmt.Sprintf("treeAst{\n lang: %q,\n filePath: %q,\n AST:\n  %v\n}", tree.lang.grammar, tree.filePath, tree.sitterTree.RootNode().SExpr(tree.lang.lang))
}

func PathToLanguage(p string) LanguageGrammar {
	return extensionToLanguage(path.Ext(p))
}

// Based on https://github.com/github-linguist/linguist/blob/master/lib/linguist/languages.yml
var extLanguages = map[string]LanguageGrammar{
	"go": Go,

	"rs": Rust,

	"kt":  Kotlin,
	"ktm": Kotlin,
	"kts": Kotlin,

	"bzl": Starlark,

	"ts":  Typescript,
	"cts": Typescript,
	"mts": Typescript,
	"js":  Typescript,
	"mjs": Typescript,
	"cjs": Typescript,

	"tsx": TypescriptX,
	"jsx": TypescriptX,

	"java": Java,
	"jav":  Java,
	"jsh":  Java,
	"json": JSON,

	"hcl":    HCL,
	"nomad":  HCL,
	"tf":     HCL,
	"tfvars": HCL,
	"tofu":   HCL,

	// Not commonly used, although linguist says this is HCL.
	// "workflow": HCL,

	"rb":       Ruby,
	"rake":     Ruby,
	"gemspec":  Ruby,
	"podspec":  Ruby,
	"thor":     Ruby,
	"jbuilder": Ruby,
	"rabl":     Ruby,

	"py":  Python,
	"pyw": Python,
	"pyi": Python,
}

// In theory, this is a mirror of
// https://github.com/github-linguist/linguist/blob/master/lib/linguist/languages.yml
func extensionToLanguage(ext string) LanguageGrammar {
	var lang, found = extLanguages[ext[1:]]

	// TODO: allow override or fallback language for files
	if !found {
		BazelLog.Fatalf("Unknown source file extension %q", ext)
	}

	return lang
}

// parseTimeoutMicros caps gotreesitter's per-file parse time. Without this,
// gotreesitter's retryFullParseWithDFA path can spend many seconds on a single
// file when the first parse hits a stack/iteration/node cap — the retry forks
// the GLR stacks widely and merge-dedup goes O(stacks² × depth).
const parseTimeoutMicros uint64 = 100_000

func ParseSourceCode(lang Language, filePath string, sourceCode []byte) (AST, error) {
	tl := lang.(*treeLanguage)

	tree, err := tl.pool.Parse(sourceCode)
	if err != nil {
		return nil, err
	}

	return &treeAst{lang: tl, filePath: filePath, sourceCode: sourceCode, sitterTree: tree}, nil
}
