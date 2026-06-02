## Why

ClickHouse SQL accepts `DESCRIBE` on more than a single table identifier. Per ClickHouse's `ParserDescribeTableQuery`, the argument is parsed via `ParserTableExpression`, which accepts:

- Table identifier with optional database (`foo`, `db.foo`) — already supported today
- Table function call (`numbers(10)`, `remote(host, db.foo)`) — not supported today
- Parenthesised subquery (`(SELECT col, expr FROM inner_table)`) — not supported today
- Any of the above with optional `FINAL` and optional alias (`foo AS f`, `(SELECT 1) AS subq`) — not supported today

Today the parser accepts only `DESCRIBE [TABLE] foo` plus the recently-landed optional `SETTINGS` clause. Anything else is rejected with `"expected <ident> or <string>"` or `"<EOF> or ';' was expected"`. This change broadens the target of a `DESCRIBE` statement to accept any **single table expression** the FROM clause accepts — by routing through the same `parseTableExpr` helper that already produces `SelectQuery.From.Expr` for a one-table FROM.

**Explicit non-goal:** JOIN expressions as DESCRIBE arguments. ClickHouse's grammar uses `ParserTableExpression`, NOT a join parser — `DESCRIBE foo JOIN bar ON ...` is rejected by ClickHouse itself. Users who want to describe a join result must wrap it: `DESCRIBE (SELECT * FROM foo JOIN bar ON ...)`. Routing through `parseTableExpr` (rather than the broader `parseJoinExpr`) keeps this parser aligned with the server's grammar — no "parser accepts but server rejects" gap.

## What Changes

- `DescribeStmt.Target` changes type from `*TableIdentifier` to `*TableExpr` (concrete type). `*TableExpr` is exactly what `parseTableExpr` returns and is the same node already used as `SelectQuery.From.Expr` for a one-table FROM.
- `parseDescribeStmt` replaces `p.parseTableIdentifier(p.Pos())` with `p.parseTableExpr(p.Pos())`. The existing `SETTINGS` clause path is preserved; the only thing that changes is what `Target` is parsed from.
- No new AST nodes, no new visitor methods, no new keywords, no lexer changes. The broadened acceptance comes entirely from reusing the existing `parseTableExpr` -> `parseSubQuery` / `parseTableFunctionExpr` / `parseTableIdentifier` pipeline.

This is a **breaking API change** for any consumer that today reads `stmt.Target.Database` or `stmt.Target.Table` directly. Those consumers must add a one-level unwrap: `if ti, ok := stmt.Target.Expr.(*TableIdentifier); ok { /* use ti.Database, ti.Table.Name */ }`. The pattern is the same one already used everywhere else in the AST when reading inside a `*TableExpr` wrapper.

## Capabilities

### New Capabilities
- `describe-rich-arguments`: Recognise `DESCRIBE` with a subquery target, an aliased target, or a join expression as its argument, in addition to the bare table identifier already supported.

### Modified Capabilities
<!-- None. -->

## Impact

- **Code touched**: one field type change in `parser/ast.go`, one call-site change in `parser/parser_table.go`. Existing `End()` and `Accept()` methods on `DescribeStmt` already operate through the `Expr` interface and need no body change.
- **Breaking API change**: `DescribeStmt.Target` field is now `Expr`, not `*TableIdentifier`. Callers that pattern-match the field directly will fail to compile and must adopt the standard type-assertion pattern.
- **Behavioural contract — new inline test** added to `parser/parser_test.go`:
  - `TestParser_Describe_RichArguments` exercises the new shapes (subquery, aliased table, table function, FINAL keyword), plus regression cases (`DESCRIBE TABLE foo`, `DESCRIBE foo SETTINGS k=1`, `DESCRIBE db.foo`).
- **Negative-coverage test** added to `parser/parser_test.go`:
  - `TestParser_Describe_RejectsJoin` asserts that `DESCRIBE foo JOIN bar ON foo.x = bar.x` continues to error. This locks the parser's behaviour to ClickHouse's grammar (which rejects the join form) and documents the deliberate scope decision.
- **Behavioural contract — new golden fixtures** under `parser/testdata/ddl/`:
  - `describe_subquery.sql` — `DESCRIBE (SELECT 1 AS x, 2 AS y)`
  - `describe_with_alias.sql` — `DESCRIBE foo AS f`
  - `describe_table_function.sql` — `DESCRIBE numbers(10)`
  Each fixture produces three goldens (output/, format/, format/beautify/).
- **Pre-existing goldens that WILL be regenerated** (multi-line shift, NOT a single-line shift):
  - `parser/testdata/ddl/output/describe_table_with_table_keyword.sql.golden.json`
  - `parser/testdata/ddl/output/describe_table_without_table_keyword.sql.golden.json`
  - `parser/testdata/ddl/output/desc_table_with_table_keyword.sql.golden.json`
  - `parser/testdata/ddl/output/desc_table_without_table_keyword.sql.golden.json`
  - `parser/testdata/ddl/output/describe_table_with_settings.sql.golden.json`
  - `parser/testdata/ddl/output/describe_settings_multiple.sql.golden.json`

  `Target` shifts from a bare `*TableIdentifier` JSON object to a wrapping `*TableExpr` JSON object that contains the `*TableIdentifier` inside an `Expr` field. The format and beautify goldens for the same six fixtures may also shift if the formatter renders the wrapping `*TableExpr` differently from the bare `*TableIdentifier`. Each must be visually inspected.
- **No dependencies** added, no public package change beyond the `DescribeStmt.Target` field type.
