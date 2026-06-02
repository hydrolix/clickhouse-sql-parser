## Why

ClickHouse SQL supports `REGEXP` (and its negated form `NOT REGEXP`) as a pattern-matching infix operator that sits in the same family as `LIKE` and `ILIKE`. Today the parser recognises `LIKE`, `ILIKE`, `NOT LIKE`, `NOT ILIKE`, `BETWEEN`, `IN`, and `NOT IN` — but not `REGEXP` or `NOT REGEXP`. Queries that use `REGEXP` for pattern filtering (a common shape in observability dashboards and log-analysis queries) fail to parse at the operator position.

This change closes the gap with two small, symmetric edits: one in the precedence ladder, one in the operator predicate set, plus a matching extension to the `NOT` switch that already groups the negated pattern-matching operators.

## What Changes

- `REGEXP` is accepted as an infix operator at the same precedence level as `BETWEEN`, `LIKE`, and `ILIKE` (`PrecedenceBetweenLike`). The arm consumes the operator and parses the right-hand side via the existing `parseSubExpr`, emitting a `BinaryOperation` with `Operation: TokenKind("REGEXP")`.
- `NOT REGEXP` is accepted alongside the existing `NOT IN`, `NOT LIKE`, `NOT ILIKE` cases in the `KeywordNot` arm of `parseInfix`. The resulting `BinaryOperation` carries `Operation: TokenKind("NOT REGEXP")`, mirroring how `"NOT LIKE"` and `"NOT ILIKE"` are stored today.
- The error message issued when none of `IN`, `LIKE`, `ILIKE`, or `REGEXP` follows `NOT` becomes `"expected IN, LIKE, ILIKE or REGEXP after NOT, got <token-kind>"`.
- No new AST node, no new token kind, no lexer change, no formatter change. `KeywordRegexp` already exists in the codebase (used in `JSONOption` parsing) and is reused here without modification.

## Capabilities

### New Capabilities
- `regexp-infix-operator`: Recognise `REGEXP` and `NOT REGEXP` as infix pattern-matching operators in column expressions, producing a `BinaryOperation` whose `Operation` field carries `"REGEXP"` or `"NOT REGEXP"`.

### Modified Capabilities
<!-- None. -->

## Impact

- **Code touched**: three edits in `parser/parser_column.go` — one arm extension in `getNextPrecedence`, one predicate added to the binary-operator arm of `parseInfix`, and one case + error-message update in the `KeywordNot` switch of `parseInfix`.
- **Behavioural contract — two existing inline tests** in `parser/parser_test.go`:
  - **`TestParser_REGEXP_Bare`** — three minimal SQLs exercising `REGEXP` in WHERE clauses. Currently FAILs; flips to PASS after this change. **One additional SQL covering `NOT REGEXP`** is added as part of this change so the symmetric case is locked in by a test.
  - **`TestParser_With_REGEXP_Operators`** — one large real-world query that uses `REGEXP` alongside template variables and a SETTINGS clause. Currently FAILs; flips to PASS.
- **Behavioural contract — three new `.sql` fixtures** under `parser/testdata/query/`, exercising the parse + format + beautify pipeline through `TestParser_ParseStatements`, `TestParser_Format`, and `TestParser_FormatBeautify`. Each fixture produces three golden files (one `.sql.golden.json` for the JSON AST, one `.sql` for the formatted output, one `.sql` for the beautified output) for a total of 3 inputs + 9 goldens:
  - `select_regexp.sql` — `SELECT a, b FROM t WHERE name REGEXP '^foo'`.
  - `select_case_when_regexp.sql` — `SELECT CASE WHEN col REGEXP '^[0-9]+$' THEN toInt32(col) ELSE 0 END AS num_value FROM t`.
  - `select_not_regexp.sql` — `SELECT a, b FROM t WHERE name NOT REGEXP '^foo'`.
- **Regression guards** that must stay green: `TestParser_InvalidSyntax` and every existing golden fixture under `parser/testdata/` (matching byte-for-byte without `-update`).
- **No dependencies** added, no public API change, no breaking changes.
