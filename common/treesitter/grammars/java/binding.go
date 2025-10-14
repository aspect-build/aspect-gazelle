package java

//#include "tree_sitter/parser.h"
//TSLanguage *tree_sitter_java();
import "C"
import (
	"unsafe"

	"github.com/aspect-build/aspect-gazelle/common/treesitter"
)

func NewLanguage() treesitter.Language {
	return treesitter.NewLanguage(
		treesitter.Java,
		unsafe.Pointer(C.tree_sitter_java()),
	)
}
