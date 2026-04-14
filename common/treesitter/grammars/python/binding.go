package python

import (
	"github.com/aspect-build/aspect-gazelle/common/treesitter"
	sitter_python "github.com/smacker/go-tree-sitter/python"
)

func NewLanguage() treesitter.Language {
	return treesitter.NewLanguageFromSitter(
		treesitter.Python,
		sitter_python.GetLanguage(),
	)
}
