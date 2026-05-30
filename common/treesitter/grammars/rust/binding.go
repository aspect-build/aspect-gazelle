package rust

import (
	"github.com/aspect-build/aspect-gazelle/common/treesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

func NewLanguage() treesitter.Language {
	return treesitter.NewLanguage(treesitter.Rust, grammars.RustLanguage())
}
