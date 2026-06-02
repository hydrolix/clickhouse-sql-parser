## Why

Grafana injects template variables into ClickHouse SQL using the syntax `${name}` and `${name:format}`. Dashboard queries that contain these placeholders cannot currently be parsed — the lexer encounters `{` after the dollar sign and bails, so the parser never sees them. This makes the parser unusable for any tool in the Grafana data path that wants to inspect, validate, or transform SQL before substitution happens (linters, query rewriters, observability layers, schema-aware autocompletion).

Adding Grafana variable parsing closes this gap with a small, contained set of changes to the lexer and a few parser hot spots, while leaving the AST shape, formatter output, and existing golden tests untouched.

## What Changes

- The lexer accepts `${name}` and `${name:format}` (e.g. `${y:sqlstring}`) as a single identifier token. Inside the braces, `:` is permitted so format suffixes parse cleanly; an unclosed brace produces an explicit `unclosed variable:` error.
- The parser accepts a `$`-prefixed identifier as a first-class operand: legal in the select list, as a table name in `FROM`, as an argument inside a function call, as either side of a `WHERE` predicate, and as a value in a `SETTINGS` clause.
- The parser accepts `${VAR}` appearing between two operands as an infix operator placeholder, at a precedence positioned so the parse tree groups correctly regardless of which real operator Grafana later substitutes in.
- A small helper `matchVariable()` centralises the "current token is a `$`-prefixed identifier" predicate so the parser callsites read cleanly.
- Bare `$ident` (no braces, no format suffix) continues to lex and parse exactly as it does today. This is not a new capability — it is named explicitly as a behaviour that must be preserved.

## Capabilities

### New Capabilities
- `grafana-variable-parsing`: Recognise Grafana template variables (`${name}`, `${name:format}`, and bare `$ident`) at the lexer and parser level so SQL containing variable placeholders can be parsed without prior substitution.

### Modified Capabilities
<!-- None. This change introduces the first openspec capability for this repo. -->

## Impact

- **Code touched**, all under `parser/`: `lexer.go` (the `consumeIdent` path that recognises `${…}`), `parser_common.go` (the new `matchVariable()` helper), `parser_column.go` (a new `PrecedenceIndent` slot plus two `matchVariable()` branches in `getNextPrecedence` and `parseInfix`), `parser_table.go` (a new `matchVariable()` branch in `parseSettingsExprList`).
- **AST**: no new node types. Variables surface as ordinary identifier-shaped expressions whose `String` field carries the literal `${…}` source text. Downstream consumers — formatter, JSON marshaller, visitor implementations — require no edits.
- **Behavioural contract**: a set of bare per-feature tests defines the exact behaviour required (one test per parser entry point that needs to accept variables, plus a regression test for bare `$ident` and another for the `EXTRACT(unit FROM expr)` special form). These tests are part of this change and are the gating signal for the implementation.
- **No dependencies** added, no public API removed, no breaking changes.
