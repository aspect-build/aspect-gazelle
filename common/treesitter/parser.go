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
	"context"
	"fmt"
	"iter"
	"log"
	"path"
	"unsafe"

	sitter "github.com/smacker/go-tree-sitter"
)

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
)

type Language interface {
}

func NewLanguage(grammar LanguageGrammar, langPtr unsafe.Pointer) Language {
	return &treeLanguage{
		grammar: grammar,
		lang:    sitter.NewLanguage(langPtr),
	}
}
func NewLanguageFromSitter(grammar LanguageGrammar, lang *sitter.Language) Language {
	return &treeLanguage{
		grammar: grammar,
		lang:    lang,
	}
}

type treeLanguage struct {
	grammar LanguageGrammar
	lang    *sitter.Language
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
	tree.sitterTree.Close()
	tree.sitterTree = nil
	tree.sourceCode = nil
}

func (tree *treeAst) String() string {
	return fmt.Sprintf("treeAst{\n lang: %q,\n filePath: %q,\n AST:\n  %v\n}", tree.lang.grammar, tree.filePath, tree.sitterTree.RootNode().String())
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
}

// In theory, this is a mirror of
// https://github.com/github-linguist/linguist/blob/master/lib/linguist/languages.yml
func extensionToLanguage(ext string) LanguageGrammar {
	var lang, found = extLanguages[ext[1:]]

	// TODO: allow override or fallback language for files
	if !found {
		log.Panicf("Unknown source file extension %q", ext)
	}

	return lang
}

func ParseSourceCode(lang Language, filePath string, sourceCode []byte) (AST, error) {
	ctx := context.Background()

	parser := sitter.NewParser()
	parser.SetLanguage(lang.(*treeLanguage).lang)

	tree, err := parser.ParseCtx(ctx, nil, sourceCode)
	if err != nil {
		return nil, err
	}

	return &treeAst{lang: lang.(*treeLanguage), filePath: filePath, sourceCode: sourceCode, sitterTree: tree}, nil
}
