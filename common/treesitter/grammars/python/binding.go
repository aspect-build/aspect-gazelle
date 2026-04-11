package python

import (
	"github.com/aspect-build/aspect-gazelle/common/treesitter"
	tree_sitter_python "github.com/tree-sitter/tree-sitter-python/bindings/go"
)

func NewLanguage() treesitter.Language {
	return treesitter.NewLanguage(
		treesitter.Python,
		tree_sitter_python.Language(),
	)
}
