package hcl

import (
	sitter "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

func NewLanguage() *sitter.Language {
	return grammars.HclLanguage()
}
