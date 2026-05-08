package python

import (
	sitter "github.com/smacker/go-tree-sitter"
	sitter_python "github.com/smacker/go-tree-sitter/python"
)

func NewLanguage() *sitter.Language {
	return sitter_python.GetLanguage()
}
