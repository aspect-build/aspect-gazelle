package parser

import "strings"

// ParseResult contains the metadata extracted from a Kotlin source file.
type ParseResult struct {
	Imports []string
	Package string
	HasMain bool
}

// Parse extracts package, imports, and main function presence from Kotlin source.
//
// It tokenizes the source while skipping comments and string literals, then
// walks the token stream at brace depth 0 to match package, import, and
// top-level fun main() declarations. Brace depth tracking correctly excludes
// declarations nested inside classes, objects, or lambda expressions.
func Parse(src []byte) *ParseResult {
	result := &ParseResult{
		Imports: []string{},
	}

	tokens := tokenize(src)
	braceDepth := 0

	for i, tok := range tokens {
		if tok.kind == symbolTok {
			switch tok.sym {
			case '{':
				braceDepth++
			case '}':
				if braceDepth > 0 {
					braceDepth--
				}
			}
			continue
		}

		if braceDepth != 0 || tok.kind != identTok {
			continue
		}

		rest := tokens[i+1:]
		switch tok.text {
		case "package":
			if result.Package == "" {
				result.Package = parsePackageName(rest)
			}
		case "import":
			if imp := parseImportPath(rest); imp != "" {
				result.Imports = append(result.Imports, imp)
			}
		case "fun":
			if !result.HasMain && isMainFunction(rest) {
				result.HasMain = true
			}
		}
	}

	return result
}

// --- Tokens ---

type tokenKind uint8

const (
	identTok        tokenKind = iota // identifier or keyword
	escapedIdentTok                  // backtick-escaped identifier
	symbolTok                        // punctuation: { } ( ) . * ;
	newlineTok                       // statement boundary marker
)

type token struct {
	kind tokenKind
	text string // set for identTok, escapedIdentTok
	sym  byte   // set for symbolTok
}

// --- Lexer ---

// lexer tokenizes Kotlin source into identifiers, symbols, and newlines.
// Comments and string literal contents are discarded; newlines inside
// multi-line constructs are preserved for statement boundary detection.
type lexer struct {
	src []byte
	pos int
	out []token
}

func tokenize(src []byte) []token {
	l := &lexer{src: src, out: make([]token, 0, len(src)/4)}
	l.scan()
	return l.out
}

func (l *lexer) remaining() int      { return len(l.src) - l.pos }
func (l *lexer) peek() byte          { return l.src[l.pos] }
func (l *lexer) peekAt(off int) byte { return l.src[l.pos+off] }
func (l *lexer) advance()            { l.pos++ }

func (l *lexer) emit(tok token) {
	l.out = append(l.out, tok)
}

func (l *lexer) emitNewline() {
	l.out = append(l.out, token{kind: newlineTok})
}

func (l *lexer) scan() {
	for l.pos < len(l.src) {
		ch := l.peek()
		switch ch {
		case ' ', '\t', '\r', '\f':
			l.advance()

		case '\n':
			l.emitNewline()
			l.advance()

		case '/':
			l.scanSlash()

		case '"':
			l.scanString()

		case '\'':
			l.advance()
			l.skipQuoted('\'')

		case '`':
			l.advance()
			l.scanBacktickIdent()

		default:
			if isIdentStart(ch) {
				l.scanIdent()
			} else if isRelevantSymbol(ch) {
				l.emit(token{kind: symbolTok, sym: ch})
				l.advance()
			} else {
				l.advance()
			}
		}
	}
}

func (l *lexer) scanSlash() {
	if l.remaining() >= 2 && l.peekAt(1) == '/' {
		l.pos += 2
		l.skipLineComment()
	} else if l.remaining() >= 2 && l.peekAt(1) == '*' {
		l.pos += 2
		l.skipBlockComment()
	} else {
		l.emit(token{kind: symbolTok, sym: '/'})
		l.advance()
	}
}

func (l *lexer) scanString() {
	if l.remaining() >= 3 && l.peekAt(1) == '"' && l.peekAt(2) == '"' {
		l.pos += 3
		l.skipRawString()
	} else {
		l.advance()
		l.skipQuoted('"')
	}
}

// skipLineComment advances past a // comment (excluding the trailing newline).
func (l *lexer) skipLineComment() {
	for l.pos < len(l.src) && l.peek() != '\n' {
		l.advance()
	}
}

// skipBlockComment advances past a /* */ comment, supporting Kotlin's nested
// block comments. Newlines within the comment are emitted as tokens.
func (l *lexer) skipBlockComment() {
	depth := 1
	for l.pos < len(l.src) && depth > 0 {
		switch {
		case l.peek() == '\n':
			l.emitNewline()
			l.advance()
		case l.remaining() >= 2 && l.peek() == '/' && l.peekAt(1) == '*':
			depth++
			l.pos += 2
		case l.remaining() >= 2 && l.peek() == '*' && l.peekAt(1) == '/':
			depth--
			l.pos += 2
		default:
			l.advance()
		}
	}
}

// skipRawString advances past a """ raw string. Newlines are emitted.
func (l *lexer) skipRawString() {
	for l.pos < len(l.src) {
		switch {
		case l.peek() == '\n':
			l.emitNewline()
			l.advance()
		case l.remaining() >= 3 && l.peek() == '"' && l.peekAt(1) == '"' && l.peekAt(2) == '"':
			l.pos += 3
			return
		default:
			l.advance()
		}
	}
}

// skipQuoted advances past a string or character literal delimited by quote,
// handling backslash escapes. Newlines terminate the literal.
func (l *lexer) skipQuoted(quote byte) {
	for l.pos < len(l.src) {
		ch := l.peek()
		switch ch {
		case '\\':
			l.advance()
			if l.pos < len(l.src) {
				if l.peek() == '\n' {
					l.emitNewline()
				}
				l.advance()
			}
		case quote:
			l.advance()
			return
		case '\n':
			l.emitNewline()
			l.advance()
			return
		default:
			l.advance()
		}
	}
}

// scanIdent reads an identifier and emits it.
func (l *lexer) scanIdent() {
	start := l.pos
	for l.pos < len(l.src) && isIdentPart(l.peek()) {
		l.advance()
	}
	l.emit(token{kind: identTok, text: string(l.src[start:l.pos])})
}

// scanBacktickIdent reads a backtick-quoted identifier (e.g. `fun`).
// On failure, emits "`" as a symbol and rewinds for re-scanning.
func (l *lexer) scanBacktickIdent() {
	start := l.pos
	for l.pos < len(l.src) && l.peek() != '`' && l.peek() != '\n' {
		l.advance()
	}
	if l.pos < len(l.src) && l.peek() == '`' && l.pos > start {
		l.emit(token{kind: escapedIdentTok, text: string(l.src[start:l.pos])})
		l.advance() // skip closing backtick
	} else {
		l.emit(token{kind: symbolTok, sym: '`'})
		l.pos = start // rewind
	}
}

// --- Token stream helpers ---

// parsePackageName reads a qualified name (a.b.c) from the token stream.
func parsePackageName(tokens []token) string {
	parts, _ := readQualifiedName(tokens, false)
	return strings.Join(parts, ".")
}

// parseImportPath extracts the package path from an import declaration.
// For wildcard imports (a.b.*) it returns the prefix (a.b).
// For class imports (a.b.C) it strips the last segment (a.b).
// Returns "" for single-segment imports where no package can be inferred.
func parseImportPath(tokens []token) string {
	parts, wildcard := readQualifiedName(tokens, true)
	switch {
	case len(parts) == 0, len(parts) == 1 && !wildcard:
		return ""
	case wildcard:
		return strings.Join(parts, ".")
	default:
		return strings.Join(parts[:len(parts)-1], ".")
	}
}

// readQualifiedName reads a dot-separated identifier sequence (a.b.c) from
// the front of tokens. If allowWildcard is true and a trailing .* is found,
// wildcard is set and the parts before .* are returned.
func readQualifiedName(tokens []token, allowWildcard bool) (parts []string, wildcard bool) {
	for i := 0; i < len(tokens); {
		// Expect an identifier.
		if !isNameToken(tokens[i]) {
			break
		}
		parts = append(parts, tokens[i].text)
		i++

		// Expect a dot separator; anything else ends the name.
		if i >= len(tokens) || !isSymbol(tokens[i], '.') {
			break
		}

		// Check for trailing wildcard (.*).
		if allowWildcard && i+1 < len(tokens) && isSymbol(tokens[i+1], '*') {
			return parts, true
		}
		i++ // consume dot
	}
	return parts, false
}

// isMainFunction reports whether the tokens (after "fun") declare a function
// named "main". It scans forward to the first '(', tracking the last
// identifier seen and whether a dot preceded it, to reject extension
// receivers like fun String.main().
func isMainFunction(tokens []token) bool {
	name, hasReceiver, ok := readFunctionName(tokens)
	return ok && name == "main" && !hasReceiver
}

func readFunctionName(tokens []token) (name string, hasReceiver bool, ok bool) {
	sawDot := false
	for _, tok := range tokens {
		switch {
		case isStatementBoundary(tok):
			return "", false, false
		case isSymbol(tok, '('):
			if name == "" {
				return "", false, false
			}
			return name, hasReceiver, true
		case isNameToken(tok):
			name = tok.text
			if sawDot {
				hasReceiver = true
			}
			sawDot = false
		case isSymbol(tok, '.'):
			sawDot = true
		default:
			sawDot = false
		}
	}
	return "", false, false
}

func isStatementBoundary(tok token) bool {
	return tok.kind == newlineTok || isSymbol(tok, ';') || isSymbol(tok, '{')
}

// --- Predicates ---

func isSymbol(tok token, sym byte) bool {
	return tok.kind == symbolTok && tok.sym == sym
}

func isNameToken(tok token) bool {
	return tok.kind == identTok || tok.kind == escapedIdentTok
}

func isIdentStart(ch byte) bool {
	return ch == '_' || (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
}

func isIdentPart(ch byte) bool {
	return isIdentStart(ch) || (ch >= '0' && ch <= '9')
}

func isRelevantSymbol(ch byte) bool {
	switch ch {
	case '{', '}', '(', ')', '.', '*', ';':
		return true
	default:
		return false
	}
}
