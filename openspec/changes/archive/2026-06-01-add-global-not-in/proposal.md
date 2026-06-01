## Why

ClickHouse SQL supports `GLOBAL NOT IN (<subquery>)` as the negated form of `GLOBAL IN`, used to filter against a distributed `remote()` subquery. The parser already accepts `GLOBAL IN`, but `GLOBAL NOT IN` is rejected with `"expected IN after GLOBAL"`. Every other negated IN-family operator — bare `NOT IN`, `NOT LIKE`, `NOT ILIKE` — is supported, so the gap is a single missing arm rather than a missing feature class.

This change closes the gap with a small extension to the same parser arm that already handles `GLOBAL IN`.

## What Changes

- The `KeywordGlobal` arm of `parseInfix` (`parser/parser_column.go`) recognises an optional `NOT` between `GLOBAL` and `IN`. When present, the produced `BinaryOperation` carries `Operation: "GLOBAL NOT IN"`; when absent, it remains `Operation: "GLOBAL IN"` exactly as today.
- The error message issued when neither `IN` nor `NOT IN` follows `GLOBAL` becomes `"expected IN or NOT IN after GLOBAL, got <token-kind>"`, so the diagnostic stays accurate.
- No new AST nodes or fields. No formatter change. No lexer change.

## Capabilities

### New Capabilities
- `global-not-in`: Recognise `GLOBAL NOT IN <subquery>` as the negated form of `GLOBAL IN`, producing a `BinaryOperation` whose `Operation` field carries the verbatim `"GLOBAL NOT IN"` text.

### Modified Capabilities
<!-- None. -->

## Impact

- **Code touched**: the `KeywordGlobal` arm of `func (p *Parser) parseInfix(...)` in `parser/parser_column.go` (around line 139). One arm grows by a handful of lines.
- **Behavioural contract**: `TestParser_With_GlobalNotIn` in `parser/parser_test.go` exercises both `GLOBAL NOT IN` and `GLOBAL IN` (the latter as a smoke test that the existing case still works). It currently FAILs on the first SQL and flips to PASS after this change.
- **Regression guards** that must stay green: `TestParser_InvalidSyntax` (in particular, `GLOBAL` followed by anything other than `IN`/`NOT IN` must still produce a parser error), the full golden suite (`TestParser_ParseStatements`, `TestParser_Format`, `TestParser_FormatBeautify`), and every other parser test in the package. No `.sql` fixture under `parser/testdata/` exercises `GLOBAL NOT IN` today, so golden output is expected to remain byte-identical.
- **No dependencies** added, no public API change, no breaking changes.
