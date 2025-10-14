package starlark

//#include "tree_sitter/parser.h"
//TSLanguage *tree_sitter_starlark();
import "C"
import (
	"unsafe"

	"github.com/aspect-build/aspect-gazelle/common/treesitter"
)

func NewLanguage() treesitter.Language {
	return treesitter.NewLanguage(
		treesitter.Starlark,
		unsafe.Pointer(C.tree_sitter_starlark()))
}
