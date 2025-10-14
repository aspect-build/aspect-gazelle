package rust

import (
	"github.com/aspect-build/aspect-gazelle/common/treesitter"

	// TODO: replace with direct use of https://github.com/tree-sitter/tree-sitter-rust
	"github.com/smacker/go-tree-sitter/rust"
)

func NewLanguage() treesitter.Language {
	return treesitter.NewLanguageFromSitter(
		treesitter.Rust,
		rust.GetLanguage())
}
