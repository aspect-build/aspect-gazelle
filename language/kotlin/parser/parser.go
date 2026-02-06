package parser

import (
	"bufio"
	"bytes"
	"strings"
)

// ParseResult contains the metadata extracted from a Kotlin source file.
type ParseResult struct {
	File    string
	Imports []string
	Package string
	HasMain bool
}

// mainModifiers are Kotlin keywords that can precede a top-level fun main().
var mainModifiers = [...]string{"public ", "internal ", "private ", "suspend "}

// Parse extracts package, imports, and main function presence from Kotlin source.
//
// It performs a single pass, skipping lines inside multi-line block comments
// and raw strings, then matching each line using string prefix checks. For
// non-wildcard imports the last segment (class name) is stripped to produce
// the package path (e.g., "import a.B" yields "a").
//
// Main detection requires fun main() at column 0 (no leading whitespace),
// which naturally excludes methods nested inside classes or objects.
func Parse(filePath string, sourceCode []byte) (*ParseResult, []error) {
	result := &ParseResult{
		File:    filePath,
		Imports: []string{},
	}

	scanner := bufio.NewScanner(bytes.NewReader(sourceCode))
	commentDepth := 0
	inRawString := false

	for scanner.Scan() {
		line := scanner.Text()

		// Skip lines inside multi-line block comments or raw strings.
		if skipMultiLine(line, &commentDepth, &inRawString) {
			continue
		}

		// Strip line comment.
		if i := strings.Index(line, "//"); i >= 0 {
			line = line[:i]
		}

		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		if result.Package == "" {
			if after, ok := strings.CutPrefix(trimmed, "package "); ok {
				result.Package = firstWord(after)
			}
		}

		if after, ok := strings.CutPrefix(trimmed, "import "); ok {
			imp := firstWord(after)
			if pkg, ok := strings.CutSuffix(imp, ".*"); ok {
				result.Imports = append(result.Imports, pkg)
			} else if i := strings.LastIndexByte(imp, '.'); i >= 0 {
				result.Imports = append(result.Imports, imp[:i])
			}
			continue
		}

		// Main detection uses the untrimmed line — leading whitespace
		// distinguishes top-level declarations from nested ones.
		if !result.HasMain && isTopLevelMain(line) {
			result.HasMain = true
		}
	}

	if err := scanner.Err(); err != nil {
		return result, []error{err}
	}

	return result, nil
}

// skipMultiLine tracks multi-line block comment and raw string state,
// returning true if the current line should be skipped.
//
// Lines that open or close a multi-line region are skipped entirely. This is
// a deliberate trade-off: package/import/main declarations never share a line
// with block comment or raw string boundaries in well-formatted Kotlin.
func skipMultiLine(line string, commentDepth *int, inRawString *bool) bool {
	if *inRawString {
		if strings.Contains(line, `"""`) {
			*inRawString = false
		}
		return true
	}

	if *commentDepth > 0 {
		*commentDepth += strings.Count(line, "/*") - strings.Count(line, "*/")
		return true
	}

	// Odd number of triple-quotes means a raw string opened without closing.
	if strings.Count(line, `"""`)%2 == 1 {
		*inRawString = true
		return true
	}

	// Block comment that opens but doesn't fully close on this line.
	opens := strings.Count(line, "/*")
	closes := strings.Count(line, "*/")
	if opens > closes {
		*commentDepth = opens - closes
		return true
	}

	return false
}

// isTopLevelMain reports whether line declares a top-level fun main().
// Only matches at column 0 (no leading whitespace), rejecting main() nested
// inside classes or objects.
func isTopLevelMain(line string) bool {
	for {
		stripped := false
		for _, mod := range mainModifiers {
			if strings.HasPrefix(line, mod) {
				line = line[len(mod):]
				stripped = true
			}
		}
		if !stripped {
			break
		}
	}
	return strings.HasPrefix(line, "fun main(")
}

// firstWord returns the first whitespace-delimited token in s.
func firstWord(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, ' '); i >= 0 {
		return s[:i]
	}
	return s
}
