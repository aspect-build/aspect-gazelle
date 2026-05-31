//! Tree-sitter query bridge for the aspect-gazelle language extensions.
//!
//! This crate is built as a `staticlib` and linked into the gazelle Go binary
//! via cgo (see the `treesitter/query` Go package). A single FFI call parses a
//! source file with the requested grammar, runs a batch of tree-sitter queries
//! against the resulting tree, and returns the captures plus any parse-error
//! diagnostics. Doing parse + query + capture-extraction in one call keeps the
//! Go side allocation-free (no per-node objects cross the boundary).
//!
//! ## Wire format
//!
//! All integers are little-endian `u32`. A *string list* is encoded as:
//!
//! ```text
//! u32 count
//! count times: u32 byte_len, then byte_len UTF-8 bytes
//! ```
//!
//! The `queries` input is a single string list. The result buffer is:
//!
//! ```text
//! u32 num_queries
//! num_queries times (one per input query, same order):
//!   u32 num_matches
//!   num_matches times:
//!     u32 num_captures
//!     num_captures times: <string: capture name> <string: captured text>
//! <string list: parse-error diagnostics>
//! ```

use std::collections::HashMap;
use std::sync::{Arc, Mutex, OnceLock};

use streaming_iterator::StreamingIterator;
use tree_sitter::{Language, Node, Parser, Query, QueryCursor};

/// Maps a grammar name (matching the Go `query.Grammar` constants) to its
/// tree-sitter `Language`. Returns `None` for an unknown grammar.
fn language_for(grammar: &str) -> Option<Language> {
    let lang: Language = match grammar {
        "go" => tree_sitter_go::LANGUAGE.into(),
        "rust" => tree_sitter_rust::LANGUAGE.into(),
        "java" => tree_sitter_java::LANGUAGE.into(),
        "json" => tree_sitter_json::LANGUAGE.into(),
        "python" => tree_sitter_python::LANGUAGE.into(),
        "ruby" => tree_sitter_ruby::LANGUAGE.into(),
        "hcl" => tree_sitter_hcl::LANGUAGE.into(),
        "starlark" => tree_sitter_starlark::LANGUAGE.into(),
        "typescript" => tree_sitter_typescript::LANGUAGE_TYPESCRIPT.into(),
        "tsx" => tree_sitter_typescript::LANGUAGE_TSX.into(),
        // ast-grep's republication of the fwcd Kotlin grammar (the upstream
        // crate pins the ancient tree-sitter 0.20 and cannot be used).
        "kotlin" => tree_sitter_kotlin_sg::LANGUAGE.into(),
        _ => return None,
    };
    Some(lang)
}

/// The captures collected for a batch of queries against one parsed file.
#[derive(Default)]
struct QueryResults {
    /// One entry per input query (same order); each is a list of matches; each
    /// match is a list of (capture name, captured text) pairs.
    queries: Vec<Vec<Vec<(String, String)>>>,
    /// Human-readable parse-error diagnostics (mirrors the old Go QueryErrors).
    /// Advisory: the file still produced results.
    parse_errors: Vec<String>,
    /// Errors compiling an input query string. Unlike parse_errors these are a
    /// hard failure (a plugin bug) — the Go caller turns them into an error.
    query_errors: Vec<String>,
}

/// Formats a tree-sitter query-compile error to match the message the previous
/// go-tree-sitter binding produced (e.g. "invalid node type 'import_' at line 1
/// column 0"), so callers and snapshots see a stable diagnostic.
fn format_query_error(e: &tree_sitter::QueryError) -> String {
    use tree_sitter::QueryErrorKind::*;
    let what = match e.kind {
        NodeType => format!("invalid node type '{}'", e.message),
        Field => format!("invalid field name '{}'", e.message),
        Capture => format!("invalid capture name '{}'", e.message),
        Predicate => format!("invalid predicate: {}", e.message),
        Structure => "impossible pattern".to_string(),
        Syntax => "invalid syntax".to_string(),
        Language => "language error".to_string(),
    };
    format!("{} at line {} column {}", what, e.row + 1, e.column)
}

/// Compiled-query cache keyed by (grammar, query text), mirroring the old Go
/// `queryCache`. `Query` is immutable and `Send + Sync` once built.
static QUERY_CACHE: OnceLock<Mutex<HashMap<(String, String), Arc<Query>>>> = OnceLock::new();

fn get_query(
    grammar: &str,
    language: &Language,
    text: &str,
) -> Result<Arc<Query>, tree_sitter::QueryError> {
    let cache = QUERY_CACHE.get_or_init(|| Mutex::new(HashMap::new()));
    let key = (grammar.to_string(), text.to_string());
    if let Some(q) = cache.lock().unwrap().get(&key) {
        return Ok(q.clone());
    }
    let q = Arc::new(Query::new(language, text)?);
    cache.lock().unwrap().insert(key, q.clone());
    Ok(q)
}

fn run(grammar: &str, source: &str, queries: &[String]) -> QueryResults {
    let mut out = QueryResults::default();
    // Always emit one (possibly empty) result slot per input query so the Go
    // side can map results back to query keys by index.
    out.queries = Vec::with_capacity(queries.len());

    let Some(language) = language_for(grammar) else {
        out.parse_errors.push(format!("unknown tree-sitter grammar {grammar:?}"));
        out.queries.extend(queries.iter().map(|_| Vec::new()));
        return out;
    };

    let mut parser = Parser::new();
    if parser.set_language(&language).is_err() {
        out.parse_errors
            .push(format!("failed to set tree-sitter language {grammar:?}"));
        out.queries.extend(queries.iter().map(|_| Vec::new()));
        return out;
    }

    let Some(tree) = parser.parse(source, None) else {
        out.parse_errors.push("tree-sitter failed to parse source".to_string());
        out.queries.extend(queries.iter().map(|_| Vec::new()));
        return out;
    };

    let root = tree.root_node();
    let src = source.as_bytes();

    for q in queries {
        let mut matches_out: Vec<Vec<(String, String)>> = Vec::new();
        match get_query(grammar, &language, q) {
            Ok(query) => {
                let names = query.capture_names();
                let mut cursor = QueryCursor::new();
                // `matches` filters out matches failing standard text predicates
                // (#eq?/#not-eq?/#match?/#not-match?/#any-of? and friends), so
                // the hand-rolled Go `filters.go` is no longer needed.
                let mut it = cursor.matches(&query, root, src);
                while let Some(m) = it.next() {
                    let mut caps: Vec<(String, String)> = Vec::with_capacity(m.captures.len());
                    for c in m.captures {
                        let name = names[c.index as usize].to_string();
                        let text = c.node.utf8_text(src).unwrap_or("").to_string();
                        caps.push((name, text));
                    }
                    matches_out.push(caps);
                }
            }
            Err(e) => {
                out.query_errors.push(format_query_error(&e));
            }
        }
        out.queries.push(matches_out);
    }

    if root.has_error() {
        let lines: Vec<&str> = source.split('\n').collect();
        collect_parse_errors(root, &lines, &mut out.parse_errors);
    }

    out
}

/// Walks the tree collecting a diagnostic for each ERROR / MISSING node,
/// formatted like the previous Go `QueryErrors` (a source line + a caret).
fn collect_parse_errors(node: Node, lines: &[&str], out: &mut Vec<String>) {
    if node.is_error() || node.is_missing() {
        let start = node.start_position();
        let line = lines.get(start.row).copied().unwrap_or("");
        let prefix = format!("     {}: ", start.row + 1);
        let caret = format!("{}^", " ".repeat(prefix.len() + start.column));
        out.push(format!("{prefix}{line}\n{caret}"));
        return; // don't descend into an error subtree; one diagnostic per error
    }
    let mut cursor = node.walk();
    for child in node.children(&mut cursor) {
        collect_parse_errors(child, lines, out);
    }
}

// ---- FFI boundary -----------------------------------------------------------

unsafe fn as_str<'a>(ptr: *const u8, len: usize) -> std::borrow::Cow<'a, str> {
    if ptr.is_null() || len == 0 {
        return std::borrow::Cow::Borrowed("");
    }
    String::from_utf8_lossy(std::slice::from_raw_parts(ptr, len))
}

unsafe fn as_bytes<'a>(ptr: *const u8, len: usize) -> &'a [u8] {
    if ptr.is_null() || len == 0 {
        return &[];
    }
    std::slice::from_raw_parts(ptr, len)
}

fn read_u32(b: &[u8], off: &mut usize) -> Option<u32> {
    if *off + 4 > b.len() {
        return None;
    }
    let v = u32::from_le_bytes([b[*off], b[*off + 1], b[*off + 2], b[*off + 3]]);
    *off += 4;
    Some(v)
}

/// Decodes a length-prefixed string list (the `queries` input).
fn decode_string_list(b: &[u8]) -> Vec<String> {
    let mut out = Vec::new();
    let mut off = 0usize;
    let Some(count) = read_u32(b, &mut off) else {
        return out;
    };
    for _ in 0..count {
        let Some(n) = read_u32(b, &mut off) else { break };
        let n = n as usize;
        if off + n > b.len() {
            break;
        }
        out.push(String::from_utf8_lossy(&b[off..off + n]).into_owned());
        off += n;
    }
    out
}

fn push_str(buf: &mut Vec<u8>, s: &str) {
    buf.extend_from_slice(&(s.len() as u32).to_le_bytes());
    buf.extend_from_slice(s.as_bytes());
}

fn encode(r: &QueryResults) -> Vec<u8> {
    let mut buf = Vec::new();
    buf.extend_from_slice(&(r.queries.len() as u32).to_le_bytes());
    for matches in &r.queries {
        buf.extend_from_slice(&(matches.len() as u32).to_le_bytes());
        for caps in matches {
            buf.extend_from_slice(&(caps.len() as u32).to_le_bytes());
            for (name, value) in caps {
                push_str(&mut buf, name);
                push_str(&mut buf, value);
            }
        }
    }
    buf.extend_from_slice(&(r.parse_errors.len() as u32).to_le_bytes());
    for e in &r.parse_errors {
        push_str(&mut buf, e);
    }
    buf.extend_from_slice(&(r.query_errors.len() as u32).to_le_bytes());
    for e in &r.query_errors {
        push_str(&mut buf, e);
    }
    buf
}

/// Runs `parse`, recovering from a panic with a diagnostic instead of unwinding
/// across the FFI boundary (which is undefined behavior).
fn catch(parse: impl FnOnce() -> QueryResults + std::panic::UnwindSafe) -> QueryResults {
    std::panic::catch_unwind(parse).unwrap_or_else(|_| QueryResults {
        parse_errors: vec!["internal error: the tree-sitter query bridge panicked".to_string()],
        ..Default::default()
    })
}

/// Parse `src` with `grammar`, run each query in the `queries` string list, and
/// return a heap-allocated buffer of `*out_len` bytes holding the encoded
/// result (see the module docs). The caller owns the buffer and must release it
/// with [`ts_query_free`].
///
/// # Safety
/// Each `*_ptr`/`*_len` pair must describe `*_len` readable bytes (or be null
/// with a zero length). `out_len` must be a valid writable pointer.
#[no_mangle]
pub unsafe extern "C" fn ts_query_run(
    grammar: *const u8,
    grammar_len: usize,
    _path: *const u8,
    _path_len: usize,
    src: *const u8,
    src_len: usize,
    queries: *const u8,
    queries_len: usize,
    out_len: *mut usize,
) -> *mut u8 {
    let grammar = as_str(grammar, grammar_len).into_owned();
    let source = as_str(src, src_len).into_owned();
    let query_list = decode_string_list(as_bytes(queries, queries_len));

    let result = catch(move || run(&grammar, &source, &query_list));

    let boxed = encode(&result).into_boxed_slice();
    let len = boxed.len();
    if !out_len.is_null() {
        *out_len = len;
    }
    Box::into_raw(boxed) as *mut u8
}

/// Release a buffer previously returned by [`ts_query_run`].
///
/// # Safety
/// `ptr`/`len` must be exactly the values produced by a single prior
/// `ts_query_run` call, and must not be freed more than once.
#[no_mangle]
pub unsafe extern "C" fn ts_query_free(ptr: *mut u8, len: usize) {
    if ptr.is_null() {
        return;
    }
    let slice = std::slice::from_raw_parts_mut(ptr, len);
    drop(Box::from_raw(slice as *mut [u8]));
}

#[cfg(test)]
mod tests {
    use super::*;

    /// Runs a single query and returns the matches as maps, for convenience.
    fn query_one(grammar: &str, source: &str, query: &str) -> Vec<HashMap<String, String>> {
        let r = run(grammar, source, &[query.to_string()]);
        r.queries
            .into_iter()
            .next()
            .unwrap_or_default()
            .into_iter()
            .map(|caps| caps.into_iter().collect())
            .collect()
    }

    #[test]
    fn go_imports() {
        let src = "package main\nimport \"fmt\"\n";
        let matches = query_one("go", src, "(import_spec (interpreted_string_literal) @path)");
        assert_eq!(matches.len(), 1);
        assert_eq!(matches[0]["path"], "\"fmt\"");
    }

    #[test]
    fn starlark_load_predicate() {
        // Exercises a #eq? text predicate: only `load(...)` calls match.
        let src = "load(\"//foo:bar.bzl\", \"sym\")\nother(\"x\", \"y\")\n";
        let q = r#"(module (expression_statement (call
            function: (identifier) @id
            arguments: (argument_list (string) @path (string)))
            (#eq? @id "load")))"#;
        let matches = query_one("starlark", src, q);
        assert_eq!(matches.len(), 1, "only the load() call should match");
        assert_eq!(matches[0]["id"], "load");
    }

    #[test]
    fn kotlin_package_and_imports() {
        let src = "package com.example.app\nimport a.b.C\nfun main() {}\n";
        let q = r#"
            (source_file (package_header (identifier) @package))
            (source_file (import_list (import_header (identifier) @from)))
            (source_file (function_declaration (simple_identifier) @main) (#eq? @main "main"))
        "#;
        let r = run("kotlin", src, &[q.to_string()]);
        let caps: Vec<(String, String)> = r.queries[0].iter().flatten().cloned().collect();
        assert!(caps.iter().any(|(n, v)| n == "package" && v == "com.example.app"));
        assert!(caps.iter().any(|(n, v)| n == "from" && v == "a.b.C"));
        assert!(caps.iter().any(|(n, _)| n == "main"));
    }

    #[test]
    fn unknown_grammar_yields_empty_with_diagnostic() {
        let r = run("nope", "x", &["(x) @y".to_string()]);
        assert_eq!(r.queries.len(), 1);
        assert!(r.queries[0].is_empty());
        assert!(!r.parse_errors.is_empty());
    }

    #[test]
    fn parse_error_reported() {
        // Unterminated string -> ERROR node in the tree.
        let r = run("go", "package main\nvar x = \"unterminated\n", &[]);
        assert!(!r.parse_errors.is_empty(), "expected a parse-error diagnostic");
    }

    #[test]
    fn encode_round_trips_empty() {
        let r = QueryResults {
            queries: vec![Vec::new(), Vec::new()],
            parse_errors: Vec::new(),
            query_errors: Vec::new(),
        };
        let buf = encode(&r);
        // num_queries=2, each [num_matches=0], parse_errors=0, query_errors=0
        // => 4 + 4 + 4 + 4 + 4
        assert_eq!(buf.len(), 20);
    }

    #[test]
    fn invalid_query_node_type_reported_as_query_error() {
        // Mirrors the old go-tree-sitter message exactly so snapshots are stable.
        let r = run("go", "package main\n", &["(import_) @x".to_string()]);
        assert_eq!(r.query_errors.len(), 1);
        assert_eq!(
            r.query_errors[0],
            "invalid node type 'import_' at line 1 column 1"
        );
        // The query still gets an (empty) result slot.
        assert_eq!(r.queries.len(), 1);
        assert!(r.queries[0].is_empty());
    }

    #[test]
    fn decode_string_list_round_trip() {
        let mut buf = Vec::new();
        buf.extend_from_slice(&2u32.to_le_bytes());
        push_str(&mut buf, "alpha");
        push_str(&mut buf, "");
        assert_eq!(decode_string_list(&buf), vec!["alpha".to_string(), "".to_string()]);
    }
}
