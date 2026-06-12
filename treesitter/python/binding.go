package python

//typedef struct TSLanguage TSLanguage;
//TSLanguage *tree_sitter_python();
import "C"
import (
	"unsafe"

	// Linked only for its C grammar symbols (tree_sitter_python). A standalone
	// @tree-sitter-python archive would duplicate parser.c/scanner.c symbols in
	// binaries that also link go-tree-sitter's python package (see BUILD note).
	_ "github.com/smacker/go-tree-sitter/python"
)

// LanguagePtr returns the raw tree-sitter grammar (`const TSLanguage *`),
// for use with a parsing backend such as common/treesitter NewLanguage().
func LanguagePtr() unsafe.Pointer {
	return unsafe.Pointer(C.tree_sitter_python())
}
