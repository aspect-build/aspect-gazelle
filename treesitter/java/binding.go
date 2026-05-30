package java

//#include "tree_sitter/parser.h"
//TSLanguage *tree_sitter_java();
import "C"
import (
	"unsafe"

	sitter "github.com/smacker/go-tree-sitter"
)

func NewLanguage() *sitter.Language {
	return sitter.NewLanguage(unsafe.Pointer(C.tree_sitter_java()))
}
