package parser

/*
#include <stdint.h>
#include <stddef.h>

// Implemented by the Rust crate //crates/js-parser (linked via cdeps).
// Parses `src` (src_len bytes) as the file located at `path` (path_len bytes),
// returning a heap-allocated buffer of *out_len bytes holding the encoded
// ParseResult (see decodeParseResult). The caller owns the buffer and must
// release it with js_parser_free. Returns NULL on allocation failure.
extern uint8_t *js_parser_parse(const uint8_t *path, size_t path_len,
                                const uint8_t *src, size_t src_len,
                                size_t *out_len);
extern void js_parser_free(uint8_t *ptr, size_t len);
*/
import "C"

import (
	"encoding/binary"
	"fmt"
	"runtime"
	"strings"
	"unsafe"
)

// Parse and find imports in JavaScript/TypeScript source files using the oxc
// (Rust) parser, linked into this package via cgo. See //crates/js-parser.

type ParseResult struct {
	// Imports interpreted based on the format such as distinguishing relative vs absolute imports
	Imports []string

	// Imports sourced from jsx/tsx attributes
	JSXImports []string

	// Imports known to always be relative to the file no matter what format they are in
	URLImports []string

	// Defined module names via "declare module 'modname' { ... }"
	Modules []string

	// Syntax errors reported by the parser. When non-empty the lists above may
	// be incomplete: oxc has no error recovery and bails with an empty AST on a
	// syntax error, so a file with errors can yield no imports at all.
	Errors []string
}

type ParseErrors struct {
	Errors []error
}

var _ error = (*ParseErrors)(nil)

func (pe *ParseErrors) Error() string {
	s := make([]string, 0, len(pe.Errors))
	for _, err := range pe.Errors {
		s = append(s, err.Error())
	}
	return strings.Join(s, "\n")
}

// ParseSource parses the given file for import statements and module
// declarations.
//
// The result is returned from Rust as a single allocation using a compact,
// little-endian length-prefixed encoding of five string lists, in order:
// Imports, JSXImports, URLImports, Modules, Errors. Each list is encoded as:
//
//	uint32 count
//	count times: uint32 byte_len, followed by byte_len UTF-8 bytes
func ParseSource(filePath string, sourceCode []byte) (ParseResult, error) {
	pathBytes := []byte(filePath)

	var pathPtr *C.uint8_t
	if len(pathBytes) > 0 {
		pathPtr = (*C.uint8_t)(unsafe.Pointer(&pathBytes[0]))
	}
	var srcPtr *C.uint8_t
	if len(sourceCode) > 0 {
		srcPtr = (*C.uint8_t)(unsafe.Pointer(&sourceCode[0]))
	}

	var outLen C.size_t
	buf := C.js_parser_parse(pathPtr, C.size_t(len(pathBytes)), srcPtr, C.size_t(len(sourceCode)), &outLen)

	// Ensure the Go-owned input buffers outlive the synchronous C call.
	runtime.KeepAlive(pathBytes)
	runtime.KeepAlive(sourceCode)

	if buf == nil {
		return ParseResult{}, &ParseErrors{[]error{fmt.Errorf("js parser returned null for %q", filePath)}}
	}
	defer C.js_parser_free(buf, outLen)

	encoded := C.GoBytes(unsafe.Pointer(buf), C.int(outLen))

	result, err := decodeParseResult(encoded)
	if err != nil {
		return ParseResult{}, &ParseErrors{[]error{fmt.Errorf("decoding js parser result for %q: %w", filePath, err)}}
	}

	return result, nil
}

func decodeParseResult(b []byte) (ParseResult, error) {
	r := byteReader{b: b}

	imports, err := r.stringList()
	if err != nil {
		return ParseResult{}, fmt.Errorf("imports: %w", err)
	}
	jsxImports, err := r.stringList()
	if err != nil {
		return ParseResult{}, fmt.Errorf("jsx imports: %w", err)
	}
	urlImports, err := r.stringList()
	if err != nil {
		return ParseResult{}, fmt.Errorf("url imports: %w", err)
	}
	modules, err := r.stringList()
	if err != nil {
		return ParseResult{}, fmt.Errorf("modules: %w", err)
	}
	errors, err := r.stringList()
	if err != nil {
		return ParseResult{}, fmt.Errorf("errors: %w", err)
	}
	if r.off != len(r.b) {
		return ParseResult{}, fmt.Errorf("trailing bytes: %d of %d consumed", r.off, len(r.b))
	}

	return ParseResult{
		Imports:    imports,
		JSXImports: jsxImports,
		URLImports: urlImports,
		Modules:    modules,
		Errors:     errors,
	}, nil
}

type byteReader struct {
	b   []byte
	off int
}

func (r *byteReader) u32() (uint32, error) {
	if r.off+4 > len(r.b) {
		return 0, fmt.Errorf("unexpected end of buffer reading uint32 at offset %d", r.off)
	}
	v := binary.LittleEndian.Uint32(r.b[r.off:])
	r.off += 4
	return v, nil
}

func (r *byteReader) stringList() ([]string, error) {
	count, err := r.u32()
	if err != nil {
		return nil, err
	}
	if count == 0 {
		return nil, nil
	}
	out := make([]string, 0, count)
	for i := uint32(0); i < count; i++ {
		n, err := r.u32()
		if err != nil {
			return nil, err
		}
		if r.off+int(n) > len(r.b) {
			return nil, fmt.Errorf("unexpected end of buffer reading %d-byte string at offset %d", n, r.off)
		}
		out = append(out, string(r.b[r.off:r.off+int(n)]))
		r.off += int(n)
	}
	return out, nil
}
