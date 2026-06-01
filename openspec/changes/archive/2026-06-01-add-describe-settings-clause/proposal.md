## Why

ClickHouse SQL accepts a trailing `SETTINGS` clause on `DESCRIBE`/`DESC` statements — for example `DESCRIBE TABLE foo SETTINGS describe_compact_output=1` — to tune the columns returned and their representation. Dashboard tools and observability layers that introspect ClickHouse schemas commonly use these settings to ask for compact output, to include sub-columns, or to control type display. Today the parser stops after the table identifier and rejects the trailing `SETTINGS …` clause with `"<EOF> or ';' was expected, but got: SETTINGS"`.

This change extends the existing `DescribeStmt` AST node with an optional `Settings *SettingsClause` field and teaches the parser to consume the trailing clause. The same `SettingsClause` node already used by `SelectQuery`, `AlterTable`, and other statements is reused — no new AST machinery.

## What Changes

- A new optional `Settings *SettingsClause` field is added to `DescribeStmt` in `parser/ast.go`. When present, `End()` returns the settings' end position; when nil, `End()` falls back to the target table identifier's end (today's behaviour).
- `DescribeStmt.Accept()` traverses the new settings sub-tree when it's non-nil, exactly as the same pattern is used elsewhere for optional clauses.
- `parseDescribeStmt` in `parser/parser_table.go` calls the existing `tryParseSettingsClause(p.Pos())` helper after the table identifier. The helper returns `(nil, nil)` when SETTINGS is absent — so the success path is `DESCRIBE TABLE foo` (Settings stays nil, no behavioural change) or `DESCRIBE TABLE foo SETTINGS k=v` (Settings populated).
- No new token kind, no new visitor method, no new keyword. `KeywordSettings` is already used by other statements; `SettingsClause` and `tryParseSettingsClause` are existing infrastructure.

## Capabilities

### New Capabilities
- `describe-settings-clause`: Recognise a trailing `SETTINGS key1=value1, key2=value2, …` clause on `DESCRIBE`/`DESC` statements, storing the parsed clause as `DescribeStmt.Settings`.

### Modified Capabilities
<!-- None. -->

## Impact

- **Code touched**: one new struct field in `parser/ast.go`, three small method-body changes in `parser/ast.go` (`End`, `Accept` of `DescribeStmt`), one call insertion in `parser/parser_table.go` (`parseDescribeStmt`). No public API removed; the new struct field is additive.
- **AST snapshot footprint**: the existing JSON goldens at `parser/testdata/ddl/output/describe_table_with_table_keyword.sql.golden.json` and `describe_table_without_table_keyword.sql.golden.json` do NOT use `omitempty` and currently render every field explicitly (including nil pointers as `null`). Adding `Settings *SettingsClause` will add a single new `"Settings": null` line to both goldens. This is a controlled, locally-reviewable diff — the change MUST regenerate these two goldens, the diff MUST be visually verified to be exactly one new line per file, and the new golden state MUST be committed.
- **Behavioural contract — one existing inline test** in `parser/parser_test.go`:
  - **`TestParser_With_DescribeSettings`** — two SQLs: `DESCRIBE TABLE foo SETTINGS describe_compact_output=1` and `DESCRIBE foo SETTINGS describe_compact_output=1, describe_include_subcolumns=1`. Currently FAILs; flips to PASS after this change.
- **Behavioural contract — two new `.sql` fixtures** under `parser/testdata/ddl/`, exercising parse + format + beautify through `TestParser_ParseStatements`, `TestParser_Format`, `TestParser_FormatBeautify`. Each fixture produces three golden files (one `.sql.golden.json`, one `.sql` formatted, one `.sql` beautified) for a total of 2 inputs + 6 new goldens:
  - `describe_table_with_settings.sql` — `DESCRIBE TABLE foo SETTINGS describe_compact_output=1`.
  - `describe_settings_multiple.sql` — `DESCRIBE foo SETTINGS describe_compact_output=1, describe_include_subcolumns=1`.
- **No dependencies** added, no public API change, no breaking changes for callers that don't construct or pattern-match exhaustively on `DescribeStmt`.
