#include <stdlib.h>

#include "shim.h"

TSQuery *tsb_compile(const TSLanguage *lang, const char *query_src, uint32_t query_len,
                     int32_t *err_offset, uint32_t *err_type) {
    uint32_t offset = 0;
    TSQueryError type = TSQueryErrorNone;
    TSQuery *query = ts_query_new(lang, query_src, query_len, &offset, &type);
    if (query == NULL) {
        *err_offset = (int32_t)offset;
        *err_type = (uint32_t)type;
        return NULL;
    }
    *err_offset = -1;
    return query;
}

TSTree *tsb_parse(const TSLanguage *lang, const char *src, uint32_t src_len) {
    TSParser *parser = ts_parser_new();
    ts_parser_set_language(parser, lang);
    TSTree *tree = ts_parser_parse_string(parser, NULL, src, src_len);
    ts_parser_delete(parser);
    return tree;
}

TSBCaptures tsb_query(TSQuery *query, TSTree *tree) {
    TSBCaptures res = {NULL, 0};
    if (tree == NULL) {
        return res;
    }

    TSQueryCursor *cursor = ts_query_cursor_new();
    ts_query_cursor_exec(cursor, query, ts_tree_root_node(tree));

    uint32_t cap = 0, count = 0, match_ordinal = 0;
    TSBCapture *out = NULL;
    TSQueryMatch match;
    while (ts_query_cursor_next_match(cursor, &match)) {
        // match.captures is reused by the cursor on the next call, so copy out
        // the byte ranges now.
        for (uint16_t i = 0; i < match.capture_count; i++) {
            TSQueryCapture c = match.captures[i];
            if (count == cap) {
                cap = cap ? cap * 2 : 16;
                out = (TSBCapture *)realloc(out, cap * sizeof(TSBCapture));
            }
            out[count].match = match_ordinal;
            out[count].pattern = match.pattern_index;
            out[count].capture = c.index;
            out[count].start_byte = ts_node_start_byte(c.node);
            out[count].end_byte = ts_node_end_byte(c.node);
            count++;
        }
        match_ordinal++;
    }

    ts_query_cursor_delete(cursor);

    res.captures = out;
    res.count = count;
    return res;
}

void tsb_tree_delete(TSTree *tree) {
    if (tree != NULL) {
        ts_tree_delete(tree);
    }
}

void tsb_free(TSBCapture *captures) { free(captures); }
