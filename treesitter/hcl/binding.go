package hcl

//#include "tree_sitter/parser.h"
//TSLanguage *tree_sitter_hcl();
import "C"
import "unsafe"

// LanguagePtr returns the raw tree-sitter grammar (`const TSLanguage *`),
// for use with a parsing backend such as common/treesitter NewLanguage().
func LanguagePtr() unsafe.Pointer {
	return unsafe.Pointer(C.tree_sitter_hcl())
}
