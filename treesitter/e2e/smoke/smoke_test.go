package smoke_test

import (
	"context"
	"testing"
	"unsafe"

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
	sitter "github.com/smacker/go-tree-sitter"
)

func TestParse(t *testing.T) {
	grammars := map[string]struct {
		lang func() unsafe.Pointer
		src  string
	}{
		"golang":     {golang.LanguagePtr, "package main\n"},
		"hcl":        {hcl.LanguagePtr, "a = 1\n"},
		"java":       {java.LanguagePtr, "class A {}\n"},
		"json":       {json.LanguagePtr, "{}\n"},
		"kotlin":     {kotlin.LanguagePtr, "class A\n"},
		"python":     {python.LanguagePtr, "a = 1\n"},
		"ruby":       {ruby.LanguagePtr, "a = 1\n"},
		"rust":       {rust.LanguagePtr, "fn main() {}\n"},
		"starlark":   {starlark.LanguagePtr, "a = 1\n"},
		"tsx":        {tsx.LanguagePtr, "const a = <b />;\n"},
		"typescript": {typescript.LanguagePtr, "const a: number = 1;\n"},
	}
	for name, g := range grammars {
		t.Run(name, func(t *testing.T) {
			parser := sitter.NewParser()
			parser.SetLanguage(sitter.NewLanguage(g.lang()))
			tree, err := parser.ParseCtx(context.Background(), nil, []byte(g.src))
			if err != nil {
				t.Fatal(err)
			}
			if tree == nil || tree.RootNode().HasError() {
				t.Fatalf("failed to parse %q", g.src)
			}
		})
	}
}
