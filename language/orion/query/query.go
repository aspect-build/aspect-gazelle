// Package query runs tree-sitter queries against source files via a Rust
// bridge (//crates/ts-query) linked in through cgo. A single call parses the
// file with the requested grammar, runs a batch of queries, and returns the
// per-query capture maps plus any parse-error diagnostics — no per-node objects
// cross the FFI boundary. See //crates/ts-query for the wire format.
package query

/*
#include <stdint.h>
#include <stddef.h>

// Implemented by the Rust crate //crates/ts-query (linked via cdeps). Parses
// `src` as `grammar`, runs each query in the `queries` string list, and returns
// a heap-allocated buffer of *out_len bytes holding the encoded result. The
// caller owns the buffer and must release it with ts_query_free.
extern uint8_t *ts_query_run(const uint8_t *grammar, size_t grammar_len,
                             const uint8_t *path, size_t path_len,
                             const uint8_t *src, size_t src_len,
                             const uint8_t *queries, size_t queries_len,
                             size_t *out_len);
extern void ts_query_free(uint8_t *ptr, size_t len);
*/
import "C"

import (
	"encoding/binary"
	"errors"
	"fmt"
	"path"
	"runtime"
	"strings"
	"unsafe"
)

// Grammar selects the tree-sitter grammar used to parse a file. Values must
// match the grammar names recognized by //crates/ts-query.
type Grammar string

const (
	Kotlin      Grammar = "kotlin"
	Starlark    Grammar = "starlark"
	Typescript  Grammar = "typescript"
	TypescriptX Grammar = "tsx"
	JSON        Grammar = "json"
	Java        Grammar = "java"
	Go          Grammar = "go"
	Rust        Grammar = "rust"
	Ruby        Grammar = "ruby"
	HCL         Grammar = "hcl"
	Python      Grammar = "python"
)

// Match is the set of captures of a single query match: capture name -> the
// matched node's source text. Duplicate capture names within a match keep the
// last value (matching the previous go-tree-sitter behavior).
type Match = map[string]string

// Based on https://github.com/github-linguist/linguist/blob/master/lib/linguist/languages.yml
var extGrammars = map[string]Grammar{
	"go": Go,

	"rs": Rust,

	"kt":  Kotlin,
	"ktm": Kotlin,
	"kts": Kotlin,

	"bzl": Starlark,

	"ts":  Typescript,
	"cts": Typescript,
	"mts": Typescript,
	"js":  Typescript,
	"mjs": Typescript,
	"cjs": Typescript,

	"tsx": TypescriptX,
	"jsx": TypescriptX,

	"java": Java,
	"jav":  Java,
	"jsh":  Java,
	"json": JSON,

	"hcl":    HCL,
	"nomad":  HCL,
	"tf":     HCL,
	"tfvars": HCL,
	"tofu":   HCL,

	"rb":       Ruby,
	"rake":     Ruby,
	"gemspec":  Ruby,
	"podspec":  Ruby,
	"thor":     Ruby,
	"jbuilder": Ruby,
	"rabl":     Ruby,

	"py":  Python,
	"pyw": Python,
	"pyi": Python,
}

// PathToGrammar returns the grammar for a file path's extension, or "" if the
// extension is unknown.
func PathToGrammar(p string) Grammar {
	ext := path.Ext(p)
	if ext == "" {
		return ""
	}
	return extGrammars[ext[1:]]
}

// Query parses source with the given grammar and runs each query in order.
// results[i] holds the matches for queries[i]. parseErrors carries advisory
// diagnostics for any ERROR/MISSING nodes in the parse (results may still be
// populated). A query that fails to compile (a plugin bug) is returned as a
// non-nil err, matching the previous behavior of failing the whole file.
func Query(grammar Grammar, filePath string, source []byte, queries []string) (results [][]Match, parseErrors []string, err error) {
	grammarBytes := []byte(grammar)
	pathBytes := []byte(filePath)
	queriesBlob := encodeStringList(queries)

	var outLen C.size_t
	buf := C.ts_query_run(
		bytePtr(grammarBytes), C.size_t(len(grammarBytes)),
		bytePtr(pathBytes), C.size_t(len(pathBytes)),
		bytePtr(source), C.size_t(len(source)),
		bytePtr(queriesBlob), C.size_t(len(queriesBlob)),
		&outLen,
	)

	// Ensure the Go-owned input buffers outlive the synchronous C call.
	runtime.KeepAlive(grammarBytes)
	runtime.KeepAlive(pathBytes)
	runtime.KeepAlive(source)
	runtime.KeepAlive(queriesBlob)

	if buf == nil {
		return nil, nil, fmt.Errorf("ts_query_run returned null for %q", filePath)
	}
	defer C.ts_query_free(buf, outLen)

	encoded := C.GoBytes(unsafe.Pointer(buf), C.int(outLen))
	results, parseErrors, queryErrors, err := decodeResults(encoded)
	if err != nil {
		return nil, nil, err
	}
	if len(queryErrors) > 0 {
		return nil, nil, errors.New(strings.Join(queryErrors, "\n"))
	}
	return results, parseErrors, nil
}

func bytePtr(b []byte) *C.uint8_t {
	if len(b) == 0 {
		return nil
	}
	return (*C.uint8_t)(unsafe.Pointer(&b[0]))
}

func encodeStringList(list []string) []byte {
	var u [4]byte
	buf := make([]byte, 0, 4+len(list)*8)
	binary.LittleEndian.PutUint32(u[:], uint32(len(list)))
	buf = append(buf, u[:]...)
	for _, s := range list {
		binary.LittleEndian.PutUint32(u[:], uint32(len(s)))
		buf = append(buf, u[:]...)
		buf = append(buf, s...)
	}
	return buf
}

func decodeResults(b []byte) (results [][]Match, parseErrors []string, queryErrors []string, err error) {
	r := byteReader{b: b}

	numQueries, err := r.u32()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("query count: %w", err)
	}
	results = make([][]Match, 0, numQueries)
	for qi := uint32(0); qi < numQueries; qi++ {
		numMatches, err := r.u32()
		if err != nil {
			return nil, nil, nil, fmt.Errorf("query %d match count: %w", qi, err)
		}
		matches := make([]Match, 0, numMatches)
		for mi := uint32(0); mi < numMatches; mi++ {
			numCaps, err := r.u32()
			if err != nil {
				return nil, nil, nil, fmt.Errorf("query %d match %d capture count: %w", qi, mi, err)
			}
			caps := make(Match, numCaps)
			for ci := uint32(0); ci < numCaps; ci++ {
				name, err := r.str()
				if err != nil {
					return nil, nil, nil, fmt.Errorf("query %d match %d capture %d name: %w", qi, mi, ci, err)
				}
				value, err := r.str()
				if err != nil {
					return nil, nil, nil, fmt.Errorf("query %d match %d capture %d value: %w", qi, mi, ci, err)
				}
				caps[name] = value
			}
			matches = append(matches, caps)
		}
		results = append(results, matches)
	}

	parseErrors, err = r.stringList()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("parse errors: %w", err)
	}

	queryErrors, err = r.stringList()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("query errors: %w", err)
	}

	if r.off != len(r.b) {
		return nil, nil, nil, fmt.Errorf("trailing bytes: %d of %d consumed", r.off, len(r.b))
	}

	return results, parseErrors, queryErrors, nil
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

func (r *byteReader) str() (string, error) {
	n, err := r.u32()
	if err != nil {
		return "", err
	}
	if r.off+int(n) > len(r.b) {
		return "", fmt.Errorf("unexpected end of buffer reading %d-byte string at offset %d", n, r.off)
	}
	s := string(r.b[r.off : r.off+int(n)])
	r.off += int(n)
	return s, nil
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
		s, err := r.str()
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}
