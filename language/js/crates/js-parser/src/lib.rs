//! JavaScript/TypeScript import extractor for the aspect-gazelle `language/js`
//! extension, backed by [oxc](https://oxc.rs/).
//!
//! This crate is built as a `staticlib` and linked into the Go gazelle binary
//! via cgo. Given a file path and its contents it returns the imports, JSX
//! asset references, `new URL(...)` references and ambient module declarations
//! found in the source.
//!
//! The result is returned across the FFI boundary as a single allocation using
//! a compact little-endian length-prefixed encoding of five string lists, in
//! order: imports, jsx imports, url imports, modules, errors. Each list is
//! encoded as:
//!
//! ```text
//! u32 count
//! count times: u32 byte_len, then byte_len UTF-8 bytes
//! ```

use std::sync::OnceLock;

use oxc_allocator::Allocator;
use oxc_ast::ast::*;
use oxc_ast_visit::{walk, Visit};
use oxc_parser::Parser;
use oxc_span::{GetSpan, SourceType};
use regex::Regex;

/// The references extracted from a source file, collected while walking the
/// AST. Mirrors the Go `ParseResult` this is encoded into. Entries are appended
/// in AST-traversal order; downstream gazelle dedupes and sorts them by import
/// path, so the relative order here is not significant.
#[derive(Default)]
struct ParseResult {
    imports: Vec<String>,
    jsx_imports: Vec<String>,
    url_imports: Vec<String>,
    modules: Vec<String>,
    errors: Vec<String>,
}

fn is_asset_tag(name: &str) -> bool {
    matches!(name, "img" | "video" | "source" | "audio" | "track")
}

/// The string literal of an argument, if it is a plain string literal.
fn arg_string<'a, 'b>(arg: &'b Argument<'a>) -> Option<&'b StringLiteral<'a>> {
    match arg.as_expression()? {
        Expression::StringLiteral(s) => Some(s),
        _ => None,
    }
}

fn expr_string<'a, 'b>(expr: &'b Expression<'a>) -> Option<&'b StringLiteral<'a>> {
    match expr {
        Expression::StringLiteral(s) => Some(s),
        _ => None,
    }
}

/// Whether an argument is `import.meta.url`.
fn is_import_meta_url(arg: &Argument) -> bool {
    match arg.as_expression() {
        Some(Expression::StaticMemberExpression(m)) => {
            m.property.name.as_str() == "url" && matches!(m.object, Expression::MetaProperty(_))
        }
        _ => false,
    }
}

impl<'a> Visit<'a> for ParseResult {
    fn visit_import_declaration(&mut self, it: &ImportDeclaration<'a>) {
        // Covers `import x from "y"`, `import type ... from "y"`,
        // `import "y"` (side-effect) and imports nested in ambient modules.
        self.imports.push(it.source.value.as_str().to_string());
        walk::walk_import_declaration(self, it);
    }

    fn visit_export_named_declaration(&mut self, it: &ExportNamedDeclaration<'a>) {
        if let Some(source) = &it.source {
            self.imports.push(source.value.as_str().to_string());
        }
        walk::walk_export_named_declaration(self, it);
    }

    fn visit_export_all_declaration(&mut self, it: &ExportAllDeclaration<'a>) {
        self.imports.push(it.source.value.as_str().to_string());
        walk::walk_export_all_declaration(self, it);
    }

    fn visit_call_expression(&mut self, it: &CallExpression<'a>) {
        // require("y")
        if let Expression::Identifier(id) = &it.callee {
            if id.name.as_str() == "require" {
                if let Some(s) = it.arguments.first().and_then(arg_string) {
                    self.imports.push(s.value.as_str().to_string());
                }
            }
        }
        walk::walk_call_expression(self, it);
    }

    fn visit_import_expression(&mut self, it: &ImportExpression<'a>) {
        // dynamic import("y")
        if let Some(s) = expr_string(&it.source) {
            self.imports.push(s.value.as_str().to_string());
        }
        walk::walk_import_expression(self, it);
    }

    fn visit_ts_import_equals_declaration(&mut self, it: &TSImportEqualsDeclaration<'a>) {
        // TypeScript `import x = require("y")` (a distinct AST node from a
        // CallExpression). Also fires when nested in a `namespace`.
        if let TSModuleReference::ExternalModuleReference(ext) = &it.module_reference {
            self.imports.push(ext.expression.value.as_str().to_string());
        }
        walk::walk_ts_import_equals_declaration(self, it);
    }

    fn visit_ts_import_type(&mut self, it: &TSImportType<'a>) {
        // import("y") in a type position, e.g. `typeof import("y")`.
        self.imports.push(it.source.value.as_str().to_string());
        walk::walk_ts_import_type(self, it);
    }

    fn visit_new_expression(&mut self, it: &NewExpression<'a>) {
        // new URL("y", import.meta.url)
        if let Expression::Identifier(id) = &it.callee {
            if id.name.as_str() == "URL" && it.arguments.len() >= 2 {
                if let Some(s) = arg_string(&it.arguments[0]) {
                    if is_import_meta_url(&it.arguments[1]) {
                        self.url_imports.push(s.value.as_str().to_string());
                    }
                }
            }
        }
        walk::walk_new_expression(self, it);
    }

    fn visit_ts_module_declaration(&mut self, it: &TSModuleDeclaration<'a>) {
        // declare module "y" { ... }
        if let TSModuleDeclarationName::StringLiteral(s) = &it.id {
            self.modules.push(s.value.as_str().to_string());
        }
        walk::walk_ts_module_declaration(self, it);
    }

    fn visit_jsx_opening_element(&mut self, it: &JSXOpeningElement<'a>) {
        // <img src="y" />, <video poster="y" />, etc.
        if let JSXElementName::Identifier(tag) = &it.name {
            if is_asset_tag(tag.name.as_str()) {
                for item in &it.attributes {
                    if let JSXAttributeItem::Attribute(attr) = item {
                        if let JSXAttributeName::Identifier(name) = &attr.name {
                            let n = name.name.as_str();
                            if n == "src" || n == "poster" {
                                if let Some(JSXAttributeValue::StringLiteral(s)) = &attr.value {
                                    self.jsx_imports.push(s.value.as_str().to_string());
                                }
                            }
                        }
                    }
                }
            }
        }
        walk::walk_jsx_opening_element(self, it);
    }
}

/// Matches a `/// <reference path|types="..." />` directive and captures the
/// referenced module. "lib" references are intentionally excluded as they do
/// not create a dependency.
fn triple_slash_lib(comment_text: &str) -> Option<String> {
    static RE: OnceLock<Regex> = OnceLock::new();
    let re = RE.get_or_init(|| {
        Regex::new(r#"^///\s*<reference\s+(?:path|types)\s*=\s*"([^"]+)""#).unwrap()
    });
    re.captures(comment_text)
        .map(|caps| caps[1].to_string())
}

fn extract_imports(path: &str, source: &str) -> ParseResult {
    let allocator = Allocator::default();
    let source_type = SourceType::from_path(path)
        .unwrap_or_else(|_| SourceType::default().with_typescript(true).with_jsx(true));

    let ret = Parser::new(&allocator, source, source_type).parse();

    let mut result = ParseResult::default();
    result.visit_program(&ret.program);

    // Triple-slash directives are comments, collected separately by oxc. Only
    // top-level comments count: a comment nested inside a statement (e.g. a
    // function body) is contained within that top-level statement's span and is
    // ignored.
    let top_spans: Vec<(u32, u32)> = ret
        .program
        .body
        .iter()
        .map(|stmt| {
            let span = stmt.span();
            (span.start, span.end)
        })
        .collect();
    for comment in &ret.program.comments {
        let start = comment.span.start;
        let nested = top_spans
            .iter()
            .any(|(lo, hi)| start >= *lo && start < *hi);
        if nested {
            continue;
        }
        let text = comment.span.source_text(source);
        if let Some(lib) = triple_slash_lib(text) {
            result.imports.push(lib);
        }
    }

    // Surface parser diagnostics so the caller can report that a file failed to
    // parse cleanly (and that its extracted imports may therefore be partial).
    // oxc has no error recovery: on a syntax error it bails with an empty AST,
    // so without this the dropped imports would be silent.
    result.errors = ret.errors.iter().map(|d| d.message.to_string()).collect();

    result
}

fn encode(result: &ParseResult) -> Vec<u8> {
    let mut buf = Vec::new();
    for list in [
        &result.imports,
        &result.jsx_imports,
        &result.url_imports,
        &result.modules,
        &result.errors,
    ] {
        buf.extend_from_slice(&(list.len() as u32).to_le_bytes());
        for s in list {
            buf.extend_from_slice(&(s.len() as u32).to_le_bytes());
            buf.extend_from_slice(s.as_bytes());
        }
    }
    buf
}

unsafe fn as_str<'a>(ptr: *const u8, len: usize) -> std::borrow::Cow<'a, str> {
    if ptr.is_null() || len == 0 {
        return std::borrow::Cow::Borrowed("");
    }
    String::from_utf8_lossy(std::slice::from_raw_parts(ptr, len))
}

/// Run `parse` (which may panic), recovering from a panic with an empty result
/// that carries a diagnostic in `errors`. A panic must not unwind across the
/// FFI boundary (that is undefined behavior), and degrading silently would make
/// a crashed parse look like a clean file with no imports — so the dropped
/// imports are surfaced to the caller (reported like a syntax error) instead.
fn catch_parse(parse: impl FnOnce() -> ParseResult + std::panic::UnwindSafe) -> ParseResult {
    std::panic::catch_unwind(parse).unwrap_or_else(|_| ParseResult {
        errors: vec!["internal error: the js parser panicked".to_string()],
        ..Default::default()
    })
}

/// Parse `src` as the file located at `path`, returning a heap-allocated buffer
/// of `*out_len` bytes holding the encoded result (see the module docs). The
/// caller owns the buffer and must release it with [`js_parser_free`]. Never
/// returns null for valid (possibly empty) input.
///
/// # Safety
/// `path`/`src` must point to `path_len`/`src_len` readable bytes (or be null
/// with a zero length). `out_len` must be a valid writable pointer.
#[no_mangle]
pub unsafe extern "C" fn js_parser_parse(
    path: *const u8,
    path_len: usize,
    src: *const u8,
    src_len: usize,
    out_len: *mut usize,
) -> *mut u8 {
    let path = as_str(path, path_len).into_owned();
    let source = as_str(src, src_len).into_owned();

    let result = catch_parse(|| extract_imports(&path, &source));

    let boxed = encode(&result).into_boxed_slice();
    let len = boxed.len();
    if !out_len.is_null() {
        *out_len = len;
    }
    Box::into_raw(boxed) as *mut u8
}

/// Release a buffer previously returned by [`js_parser_parse`].
///
/// # Safety
/// `ptr`/`len` must be exactly the values produced by a single prior
/// `js_parser_parse` call, and must not be freed more than once.
#[no_mangle]
pub unsafe extern "C" fn js_parser_free(ptr: *mut u8, len: usize) {
    if ptr.is_null() {
        return;
    }
    let slice = std::slice::from_raw_parts_mut(ptr, len);
    drop(Box::from_raw(slice as *mut [u8]));
}

#[cfg(test)]
mod tests {
    use super::*;

    fn imports(path: &str, src: &str) -> Vec<String> {
        extract_imports(path, src).imports
    }

    #[test]
    fn esm_import() {
        assert_eq!(imports("a.ts", "import x from 'date-fns';"), vec!["date-fns"]);
    }

    #[test]
    fn require_call() {
        assert_eq!(imports("a.ts", "const x = require('foo');"), vec!["foo"]);
    }

    #[test]
    fn dynamic_import() {
        assert_eq!(imports("a.ts", "const x = import('foo');"), vec!["foo"]);
    }

    #[test]
    fn import_equals_require() {
        assert_eq!(
            imports("a.ts", "import a = require(\"date-fns\");"),
            vec!["date-fns"]
        );
    }

    #[test]
    fn import_equals_require_nested_in_namespace() {
        let src = "namespace N {\n  import a = require(\"date-fns\");\n  export const x = a;\n}\n";
        assert_eq!(imports("a.ts", src), vec!["date-fns"]);
    }

    #[test]
    fn export_from() {
        assert_eq!(imports("a.ts", "export {x} from 'foo';"), vec!["foo"]);
        assert_eq!(imports("a.ts", "export * from 'bar';"), vec!["bar"]);
    }

    #[test]
    fn triple_slash() {
        assert_eq!(
            imports("a.ts", "/// <reference types=\"node\" />\n"),
            vec!["node"]
        );
    }

    #[test]
    fn ambient_module() {
        let e = extract_imports("a.d.ts", "declare module 'foo' { export const x: number; }");
        assert_eq!(e.modules, vec!["foo"]);
    }

    #[test]
    fn url_import() {
        let e = extract_imports("a.ts", "new URL('./x.png', import.meta.url);");
        assert_eq!(e.url_imports, vec!["./x.png"]);
    }

    #[test]
    fn jsx_asset() {
        let e = extract_imports("a.tsx", "const x = <img src=\"./logo.png\" />;");
        assert_eq!(e.jsx_imports, vec!["./logo.png"]);
    }

    #[test]
    fn order_preserved() {
        assert_eq!(
            imports("a.ts", "import a from 'a';\nimport b from 'b';"),
            vec!["a", "b"]
        );
    }

    #[test]
    fn triple_slash_top_level_only() {
        let src = "/// <reference types=\"node\" />\nfunction f() {\n/// <reference types=\"jest\" />\n}\n";
        assert_eq!(imports("a.ts", src), vec!["node"]);
    }

    #[test]
    fn type_import_forms() {
        let src = r#"
            function x<T>(a: typeof import('jquery')): T { return a as T; }
            export const F: typeof import('@aspect-test/a') = null as any
            const g = (null as any) as typeof import('@aspect-test/c')
            export type * as Foo from '@aspect-test/f'
            import type * as Bar from '@aspect-test/g'
        "#;
        assert_eq!(
            imports("a.ts", src),
            vec![
                "jquery",
                "@aspect-test/a",
                "@aspect-test/c",
                "@aspect-test/f",
                "@aspect-test/g",
            ]
        );
    }

    #[test]
    fn malformed_tsx_no_recovery() {
        // On syntactically invalid input where oxc's recursive-descent parser
        // fatally bails (ParserReturn::panicked, empty body), no imports are
        // recovered, but the syntax error is reported.
        let src = "import React from \"react\";\nexport const a = () => (<></>)\n})\n";
        let e = extract_imports("a.tsx", src);
        assert_eq!(e.imports, Vec::<String>::new());
        assert!(!e.errors.is_empty(), "expected a reported syntax error");
    }

    #[test]
    fn valid_source_has_no_errors() {
        let e = extract_imports("a.ts", "import x from 'foo';");
        assert_eq!(e.imports, vec!["foo"]);
        assert!(e.errors.is_empty());
    }

    #[test]
    fn catch_parse_passes_through_success() {
        let e = catch_parse(|| extract_imports("a.ts", "import x from 'foo';"));
        assert_eq!(e.imports, vec!["foo"]);
        assert!(e.errors.is_empty());
    }

    #[test]
    fn catch_parse_recovers_from_panic_with_diagnostic() {
        // Silence the default panic hook so the deliberate panic below doesn't
        // print a scary backtrace during an otherwise-passing test.
        let prev = std::panic::take_hook();
        std::panic::set_hook(Box::new(|_| {}));
        let e = catch_parse(|| panic!("boom"));
        std::panic::set_hook(prev);

        assert!(e.imports.is_empty());
        assert!(e.jsx_imports.is_empty());
        assert!(e.url_imports.is_empty());
        assert!(e.modules.is_empty());
        assert_eq!(e.errors, vec!["internal error: the js parser panicked"]);
    }
}
