// Package tsbatch runs a tree-sitter query against a parsed tree in a single
// cgo crossing. Instead of returning cached *Node wrappers (the smacker path
// walks matches from Go, one cgo call per match, caching a *Node per capture in
// a tree-wide map that is not concurrency-safe), the C shim runs the whole
// cursor loop and returns capture byte-ranges. Callers slice the source for text
// and apply predicates in Go.
//
// Compilation (ts_query_new — the most expensive tree-sitter op) is separated
// from execution so callers can compile once and reuse a *Query across files and
// goroutines (the compiled query is immutable). Parsing is likewise separate so
// callers parse a file once and run many queries against the one *Tree.
package tsbatch

// #include <stdlib.h>
// #include "shim.h"
import "C"

import (
	"fmt"
	"unsafe"

	// Anchor a Go-level reference to smacker so the linker retains its archive,
	// which bundles the tree-sitter runtime C objects this package calls into.
	// cgo C-symbol calls alone are not tracked by Go's dead-code elimination.
	sitter "github.com/smacker/go-tree-sitter"
)

var _ = sitter.NewParser

// Capture is one capture within a match: the capture id (mapped to a name by
// the caller via the query) and the source byte range of the captured node.
type Capture struct {
	ID        uint32
	StartByte uint32
	EndByte   uint32
}

// Match is the set of captures produced by one query match, tagged with the
// query pattern index so callers can apply that pattern's predicates.
type Match struct {
	Pattern  uint32
	Captures []Capture
}

// Query is a compiled, immutable tree-sitter query. Safe to reuse across files
// and goroutines; cache it (do not compile per file).
type Query struct {
	q *C.TSQuery
}

// Tree is a parsed source tree. Not safe for concurrent use; one per file. Call
// Close to release it.
type Tree struct {
	t *C.TSTree
}

// QueryError reports a tree-sitter query compilation failure.
type QueryError struct {
	Offset int32
	Type   uint32
}

func (e *QueryError) Error() string {
	return fmt.Sprintf("tree-sitter query error (type %d) at offset %d", e.Type, e.Offset)
}

// Compile compiles query for lang (an opaque TSLanguage*). The result is
// immutable and meant to be cached and reused.
func Compile(lang unsafe.Pointer, query string) (*Query, error) {
	var qPtr *C.char
	if len(query) > 0 {
		qPtr = (*C.char)(unsafe.Pointer(unsafe.StringData(query)))
	}
	var errOffset C.int32_t
	var errType C.uint32_t
	q := C.tsb_compile((*C.TSLanguage)(lang), qPtr, C.uint32_t(len(query)), &errOffset, &errType)
	if q == nil {
		return nil, &QueryError{Offset: int32(errOffset), Type: uint32(errType)}
	}
	return &Query{q: q}, nil
}

// Parse parses source for lang. Close the result when done.
func Parse(lang unsafe.Pointer, source []byte) *Tree {
	var srcPtr *C.char
	if len(source) > 0 {
		srcPtr = (*C.char)(unsafe.Pointer(&source[0]))
	}
	return &Tree{t: C.tsb_parse((*C.TSLanguage)(lang), srcPtr, C.uint32_t(len(source)))}
}

// Close releases the tree.
func (t *Tree) Close() {
	if t.t != nil {
		C.tsb_tree_delete(t.t)
		t.t = nil
	}
}

// RunQuery runs a compiled query against a parsed tree and returns the matches
// in document order. A nil/failed tree yields no matches.
func RunQuery(query *Query, tree *Tree) []Match {
	res := C.tsb_query(query.q, tree.t)
	n := int(res.count)
	if n == 0 {
		return nil
	}
	defer C.tsb_free(res.captures)

	flat := unsafe.Slice((*C.TSBCapture)(unsafe.Pointer(res.captures)), n)

	// Group consecutive captures by their match ordinal.
	var matches []Match
	for i := 0; i < n; {
		ord := uint32(flat[i].match)
		m := Match{Pattern: uint32(flat[i].pattern)}
		for i < n && uint32(flat[i].match) == ord {
			c := flat[i]
			m.Captures = append(m.Captures, Capture{
				ID:        uint32(c.capture),
				StartByte: uint32(c.start_byte),
				EndByte:   uint32(c.end_byte),
			})
			i++
		}
		matches = append(matches, m)
	}

	return matches
}
