package parser

import (
	"fmt"
	"regexp"
	"strings"

	BazelLog "github.com/aspect-build/aspect-gazelle/common/logger"
	treeutils "github.com/aspect-build/aspect-gazelle/common/treesitter"
	"github.com/aspect-build/aspect-gazelle/treesitter/kotlin"
)

// ParseResult holds the result of parsing a Kotlin source file.
type ParseResult struct {
	// File is the Bazel package-relative path to the Kotlin source file (e.g. "Greeter.kt"),
	// matching how files are referenced in Bazel target srcs attributes.
	File string

	// Imports is the list of parsed import statements found in the file.
	Imports []*ImportStatement

	// Package is the structured package identifier, or nil for the default package.
	Package *Identifier

	// HasMain is true if the file defines a top-level 'main' function (a binary entry point).
	HasMain bool

	// TopLevelIdentifiers is the list of unique top-level declarations defined in this file.
	TopLevelIdentifiers []*SimpleIdentifier

	// Errors is the list of parse or query errors, formatted as strings so they survive caching.
	Errors []string
}

// ImportStatement represents a single parsed Kotlin import header, which may
// optionally contain a wildcard (star import) or an import alias name.
type ImportStatement struct {
	identifier   *Identifier
	isStarImport bool
	alias        *SimpleIdentifier
}

func (i *ImportStatement) Identifier() *Identifier {
	return i.identifier
}

// IsStarImport reports whether the import is a wildcard (e.g. import com.example.*).
func (i *ImportStatement) IsStarImport() bool {
	return i.isStarImport
}

// Alias returns the local name alias (e.g. Baz in "import Foo as Baz"), or nil.
func (i *ImportStatement) Alias() *SimpleIdentifier {
	return i.alias
}

// String returns a human-readable representation, including any alias or star suffix.
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

// Identifier represents a structured dot-separated identifier path (e.g. com.example.utils).
type Identifier struct {
	parts []*SimpleIdentifier
}

// Parent returns the parent identifier path by stripping the last segment.
// Returns nil if the identifier has 1 or fewer segments.
func (i *Identifier) Parent() *Identifier {
	if len(i.parts) <= 1 {
		return nil
	}
	return &Identifier{i.parts[0 : len(i.parts)-1]}
}

// Literal returns the raw, dot-separated string representation of the identifier path.
func (i *Identifier) Literal() string {
	strs := make([]string, len(i.parts))
	for idx, part := range i.parts {
		strs[idx] = part.Literal()
	}
	return strings.Join(strs, ".")
}

// Child constructs a new Identifier by appending a child identifier component to the path.
func (i *Identifier) Child(childComponent *SimpleIdentifier) *Identifier {
	childId := &Identifier{}
	childId.parts = append(childId.parts, i.parts...)
	childId.parts = append(childId.parts, childComponent)
	return childId
}

// SimpleIdentifier represents a single valid segment of an identifier.
type SimpleIdentifier struct {
	literal string
}

// NewSimpleIdentifier validates value as a Kotlin identifier segment (stripping any
// surrounding backticks), returning an error if it is not valid.
func NewSimpleIdentifier(value string) (*SimpleIdentifier, error) {
	normalized := (&SimpleIdentifier{value}).Normalize()
	if kotlinUnquotedIdentifierRegexp.MatchString(normalized.literal) {
		return normalized, nil
	}
	return nil, fmt.Errorf("NewSimpleIdentifier only supports identifiers that match %s; %q doesn't match", kotlinUnquotedIdentifierRegexp, value)
}

func (si *SimpleIdentifier) Literal() string {
	return si.literal
}

// Normalize strips surrounding backticks if the inner literal is a valid unquoted segment.
func (si *SimpleIdentifier) Normalize() *SimpleIdentifier {
	if len(si.literal) < 2 || !strings.HasPrefix(si.literal, "`") || !strings.HasSuffix(si.literal, "`") {
		return si
	}
	betweenQuoteMarks := si.literal[1 : len(si.literal)-1]
	if kotlinUnquotedIdentifierRegexp.MatchString(betweenQuoteMarks) {
		return &SimpleIdentifier{betweenQuoteMarks}
	}
	return si
}

// AsIdentifier wraps the SimpleIdentifier into a single-segment Identifier path.
func (si *SimpleIdentifier) AsIdentifier() *Identifier {
	return &Identifier{[]*SimpleIdentifier{si}}
}

var kotlinUnquotedIdentifierRegexp = regexp.MustCompile(`^[\p{L}_][\p{L}_\d]*$`)

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

// parserQuery contains AST queries used to extract Kotlin package names, imports,
// main entry points, and top-level declarations.
//
// These nodes and rules map to the official tree-sitter-kotlin grammar definitions:
//   - [package_header]
//   - [import_header]
//   - [function_declaration]
//   - [class_declaration]
//   - [property_declaration]
//   - [type_alias]
//   - [object_declaration]
//
// [package_header]: https://github.com/fwcd/tree-sitter-kotlin/blob/main/grammar.js#L73
// [import_header]: https://github.com/fwcd/tree-sitter-kotlin/blob/main/grammar.js#L81
// [function_declaration]: https://github.com/fwcd/tree-sitter-kotlin/blob/main/grammar.js#L210
// [class_declaration]: https://github.com/fwcd/tree-sitter-kotlin/blob/main/grammar.js#L131
// [property_declaration]: https://github.com/fwcd/tree-sitter-kotlin/blob/main/grammar.js#L268
// [type_alias]: https://github.com/fwcd/tree-sitter-kotlin/blob/main/grammar.js#L123
// [object_declaration]: https://github.com/fwcd/tree-sitter-kotlin/blob/main/grammar.js#L182
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
				if aliasId, aliasErr := NewSimpleIdentifier(aliasName); aliasErr != nil {
					errs = append(errs, aliasErr)
				} else {
					alias = aliasId
				}
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

// ParseIdentifier parses a dot-separated string representation of a Kotlin identifier
// path (e.g. "com.example.hello"), validating and constructing a structured Identifier.
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
