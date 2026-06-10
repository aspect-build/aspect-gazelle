#ifndef TSBATCH_SHIM_H
#define TSBATCH_SHIM_H

#include <stdint.h>
#include "tree_sitter/api.h"

// One capture of one match. `match` is a per-call ordinal that groups the
// captures belonging to the same match (predicates are evaluated per match);
// `pattern` is the query pattern index; `capture` is the capture id (mapped to
// a name by the caller). Node text is the source slice [start_byte, end_byte).
typedef struct {
    uint32_t match;
    uint32_t pattern;
    uint32_t capture;
    uint32_t start_byte;
    uint32_t end_byte;
} TSBCapture;

typedef struct {
    TSBCapture *captures;  // malloc'd; free with tsb_free. NULL when count==0.
    uint32_t count;
} TSBCaptures;

// Compile a query. The returned TSQuery is immutable and safe to reuse across
// threads/files, so callers cache it (ts_query_new is the most expensive
// tree-sitter op and must not run per-file). NULL on failure, with *err_offset
// set to the byte offset and *err_type to the TSQueryError; *err_offset is set
// to -1 on success.
TSQuery *tsb_compile(const TSLanguage *lang, const char *query_src, uint32_t query_len,
                     int32_t *err_offset, uint32_t *err_type);

// Parse source into a tree. Caller frees with tsb_tree_delete. May return NULL.
TSTree *tsb_parse(const TSLanguage *lang, const char *src, uint32_t src_len);

// Run a precompiled query against a parsed tree, collecting every capture of
// every match in a single call.
TSBCaptures tsb_query(TSQuery *query, TSTree *tree);

void tsb_tree_delete(TSTree *tree);
void tsb_free(TSBCapture *captures);

#endif
