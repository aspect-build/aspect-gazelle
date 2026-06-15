package parser

import (
	"fmt"
	"regexp"
	"strings"

	BazelLog "github.com/aspect-build/aspect-gazelle/common/logger"
	treeutils "github.com/aspect-build/aspect-gazelle/common/treesitter"
	"github.com/aspect-build/aspect-gazelle/treesitter/kotlin"
)

// ParseResult holds the result of parsing a Kotlin file.
type ParseResult struct {
	File string

	// The list of parsed import statements.
	Imports []*ImportStatement

	// Identifier for the package name.
	Package *Identifier

	// True if the file defines a main function.
	HasMain bool

	// The identifiers of top level objects.
	TopLevelIdentifiers []*SimpleIdentifier

	// Parse/query errors.
	Errors []string
}

type ImportStatement struct {
	identifier   *Identifier
	isStarImport bool
	alias        *SimpleIdentifier
}

func (i *ImportStatement) Identifier() *Identifier {
	return i.identifier
}

func (i *ImportStatement) IsStarImport() bool {
	return i.isStarImport
}

func (i *ImportStatement) Alias() *SimpleIdentifier {
	return i.alias
}

func (i *ImportStatement) String() string {
	switch {
	case i.Alias() != nil:
		return i.Identifier().Literal() + " as " + i.Alias().Literal()
	case i.IsStarImport():
		return i.Identifier().Literal() + ".*"
	default:
		return i.Identifier().Literal()
	}
}

type Identifier struct {
	parts []*SimpleIdentifier
}

func (i *Identifier) Parent() *Identifier {
	if len(i.parts) <= 1 {
		return nil
	}
	return &Identifier{i.parts[0 : len(i.parts)-1]}
}

func (i *Identifier) Literal() string {
	strs := make([]string, len(i.parts))
	for idx, part := range i.parts {
		strs[idx] = part.Literal()
	}
	return strings.Join(strs, ".")
}

func (i *Identifier) Child(childComponent *SimpleIdentifier) *Identifier {
	childId := &Identifier{}
	childId.parts = append(childId.parts, i.parts...)
	childId.parts = append(childId.parts, childComponent)
	return childId
}

type SimpleIdentifier struct {
	literal string
}

func NewSimpleIdentifier(value string) (*SimpleIdentifier, error) {
	if kotlinUnquotedIdentifierRegexp.MatchString(value) {
		return &SimpleIdentifier{value}, nil
	}
	return nil, fmt.Errorf("NewSimpleIdentifier only supports identifiers that match %s; %q doesn't match", kotlinUnquotedIdentifierRegexp, value)
}

func (si *SimpleIdentifier) Literal() string {
	return si.literal
}

func (si *SimpleIdentifier) Normalize() *SimpleIdentifier {
	if !strings.HasPrefix(si.literal, "`") {
		return si
	}
	betweenQuoteMarks := si.literal[1 : len(si.literal)-1]
	if kotlinUnquotedIdentifierRegexp.MatchString(betweenQuoteMarks) {
		return &SimpleIdentifier{betweenQuoteMarks}
	}
	return si
}

func (si *SimpleIdentifier) AsIdentifier() *Identifier {
	return &Identifier{[]*SimpleIdentifier{si}}
}

var kotlinUnquotedIdentifierRegexp = regexp.MustCompile(`[\p{L}_][\p{L}_\d]*`)

type Parser interface {
	Parse(filePath string, source []byte) (*ParseResult, []error)
}

type treeSitterParser struct {
	Parser
}

func NewParser() Parser {
	p := treeSitterParser{}
	return &p
}

const parserQuery = `
	(source_file
		(package_header
			(identifier) @package
		)
	)

	(source_file
		(import_list
			(import_header
				(identifier) @import_name
				(wildcard_import)? @import_wildcard
				(import_alias (type_identifier) @import_alias)?
			)
		)
	)

	(source_file
		(function_declaration
			(simple_identifier) @equals_main
		)
		(#eq? @equals_main "main")
	)

	(source_file
		(class_declaration
			(type_identifier) @class_id
		)
	)

	(source_file
		(property_declaration
			(variable_declaration
				(simple_identifier) @property_id
			)
		)
	)

	(source_file
		(function_declaration
			(simple_identifier) @function_id
		)
	)

	(source_file
		(type_alias
			(type_identifier) @type_alias_id
		)
	)

	(source_file
		(object_declaration
			(type_identifier) @object_id
		)
	)
`

func (p *treeSitterParser) Parse(filePath string, sourceCode []byte) (*ParseResult, []error) {
	result := &ParseResult{
		File:    filePath,
		Imports: []*ImportStatement{},
	}

	var errs []error

	lang := treeutils.NewLanguage(treeutils.Kotlin, kotlin.LanguagePtr())
	tree, err := treeutils.ParseSourceCode(lang, filePath, sourceCode)
	if err != nil {
		errs = append(errs, err)
	}

	if tree == nil {
		return result, errs
	}
	defer tree.Close()

	q, err := treeutils.GetQuery(lang, parserQuery)
	if err != nil {
		BazelLog.Fatalf("Failed to create kotlin 'parserQuery': %v", err)
	}

	for caps := range tree.Query(q) {
		if pkg, ok := caps["package"]; ok {
			id, err := ParseIdentifier(pkg)
			if err != nil {
				errs = append(errs, err)
			} else {
				result.Package = id
			}
		}

		if impName, ok := caps["import_name"]; ok {
			id, err := ParseIdentifier(impName)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			isStar := false
			if _, starOk := caps["import_wildcard"]; starOk {
				isStar = true
			}
			var alias *SimpleIdentifier
			if aliasName, aliasOk := caps["import_alias"]; aliasOk {
				alias = &SimpleIdentifier{aliasName}
			}
			result.Imports = append(result.Imports, &ImportStatement{
				identifier:   id,
				isStarImport: isStar,
				alias:        alias,
			})
		}

		if _, ok := caps["equals_main"]; ok {
			result.HasMain = true
		}

		// Top-level identifiers
		var topLevelId string
		if id, ok := caps["class_id"]; ok {
			topLevelId = id
		} else if id, ok := caps["property_id"]; ok {
			topLevelId = id
		} else if id, ok := caps["function_id"]; ok {
			topLevelId = id
		} else if id, ok := caps["type_alias_id"]; ok {
			topLevelId = id
		} else if id, ok := caps["object_id"]; ok {
			topLevelId = id
		}

		if topLevelId != "" && topLevelId != "main" {
			simpleId, err := NewSimpleIdentifier(topLevelId)
			if err == nil {
				found := false
				for _, existing := range result.TopLevelIdentifiers {
					if existing.Literal() == simpleId.Literal() {
						found = true
						break
					}
				}
				if !found {
					result.TopLevelIdentifiers = append(result.TopLevelIdentifiers, simpleId)
				}
			}
		}
	}

	treeErrors := tree.QueryErrors()
	if treeErrors != nil {
		for _, e := range treeErrors {
			result.Errors = append(result.Errors, e.Error())
		}
		errs = append(errs, treeErrors...)
	}

	return result, errs
}

func ParseIdentifier(literal string) (*Identifier, error) {
	partsStr := strings.Split(literal, ".")
	parts := make([]*SimpleIdentifier, len(partsStr))
	for i, pStr := range partsStr {
		si, err := NewSimpleIdentifier(pStr)
		if err != nil {
			return nil, err
		}
		parts[i] = si
	}
	return &Identifier{parts}, nil
}
