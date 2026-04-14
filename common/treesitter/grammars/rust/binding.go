package rust

//#include "tree_sitter/parser.h"
//TSLanguage *tree_sitter_rust();
import "C"
import (
	"unsafe"

	"github.com/aspect-build/aspect-gazelle/common/treesitter"
)

func NewLanguage() treesitter.Language {
	return treesitter.NewLanguage(
		treesitter.Rust,
		unsafe.Pointer(C.tree_sitter_rust()))
}
