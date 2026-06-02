## 1. Baseline

- [x] 1.1 Confirm the three rejected shapes currently FAIL on HEAD. Quick probe:
  ```
  go run -gcflags='all=-N -l' /tmp/probe.go   # or any one-off main that parses each SQL
  ```
  Expected errors: `DESCRIBE (SELECT 1)` → `expected <ident> or <string>`; `DESCRIBE foo AS f` → `<EOF> or ';' was expected`; `DESCRIBE foo JOIN bar ON foo.x = bar.x` → `<EOF> or ';' was expected`.
- [x] 1.2 Confirm all six pre-existing DESCRIBE/DESC JSON goldens PASS today: `go test ./parser/... -run 'TestParser_ParseStatements/desc(_| ?_table)' -count=1`.
- [x] 1.3 Snapshot all six affected JSON goldens to `/tmp/` for post-change diff inspection:
  ```bash
  for f in describe_table_with_table_keyword describe_table_without_table_keyword \
           desc_table_with_table_keyword desc_table_without_table_keyword \
           describe_table_with_settings describe_settings_multiple; do
    cp parser/testdata/ddl/output/${f}.sql.golden.json /tmp/${f}.before.json
  done
  ls /tmp/*.before.json  # confirm 6 files
  ```
- [x] 1.4 Capture the baseline: `go test ./parser/... -run 'TestParser_With_DescribeSettings|TestParser_InvalidSyntax|TestParser_ParseStatements|TestParser_Format|TestParser_FormatBeautify' -count=1`. Existing tests must PASS. Inline `TestParser_Describe_RichArguments` does not yet exist; that's expected.

## 2. AST change in `parser/ast.go`

- [x] 2.1 Locate `type DescribeStmt struct { … }`. Change the `Target` field type from `*TableIdentifier` to `*TableExpr`.
- [x] 2.2 Verify `DescribeStmt.End()` and `DescribeStmt.Accept()` already call methods (`.End()`, `.Accept(visitor)`) that `*TableExpr` satisfies — they require no body change.
- [x] 2.3 `go build ./parser/...` — expect compile errors at any callsite that does field access like `stmt.Target.Database` directly. There should be none in this repo (DESCRIBE consumers within the parser package don't read these fields). If any appear, document and fix.

## 3. Parser change in `parser/parser_table.go`

- [x] 3.1 Locate `parseDescribeStmt`. Replace `tableIdent, err := p.parseTableIdentifier(p.Pos())` with `target, err := p.parseTableExpr(p.Pos())`. **Do NOT use `parseJoinExpr` — ClickHouse rejects JOIN at the DESCRIBE position; staying with `parseTableExpr` keeps the parser aligned with the server's grammar.**
- [x] 3.2 Update `statementEnd` computation:
  ```go
  statementEnd := target.End()
  if settings != nil {
      statementEnd = settings.End()
  }
  ```
- [x] 3.3 Update the returned struct literal to use `Target: target,` (matching the new field type).
- [x] 3.4 `go build ./parser/...`.

## 4. Add inline `TestParser_Describe_RichArguments` and `TestParser_Describe_RejectsJoin`

- [x] 4.1 In `parser/parser_test.go`, add a new positive test function:
  ```go
  func TestParser_Describe_RichArguments(t *testing.T) {
      validSQLs := []string{
          "DESCRIBE (SELECT 1)",
          "DESCRIBE (SELECT a, b FROM inner_table) AS subq",
          "DESCRIBE foo AS f",
          "DESCRIBE db.foo AS f",
          "DESCRIBE numbers(10)",
          "DESCRIBE remote('host', db.foo)",
          "DESCRIBE foo FINAL",
          "DESCRIBE foo SETTINGS describe_compact_output=1",
          "DESCRIBE TABLE foo",
          "DESCRIBE db.foo",
          "DESCRIBE (SELECT 1) SETTINGS describe_compact_output=1",
          "DESCRIBE (SELECT * FROM foo JOIN bar ON foo.x = bar.x)",   // JOIN allowed only when wrapped in a subquery
      }
      for _, sql := range validSQLs {
          _, err := NewParser(sql).ParseStmts()
          require.NoError(t, err, "Failed to parse: %s", sql)
      }
  }
  ```
- [x] 4.2 In `parser/parser_test.go`, add a new negative test function to lock the parser-vs-server alignment:
  ```go
  func TestParser_Describe_RejectsJoin(t *testing.T) {
      // ClickHouse uses ParserTableExpression (not a join parser) in
      // ParserDescribeTableQuery; the bare-JOIN form is rejected at the
      // server. Our parser matches that grammar — this test prevents a
      // future "let's just use parseJoinExpr" refactor from silently
      // reintroducing a parser-vs-server gap.
      invalidSQLs := []string{
          "DESCRIBE foo JOIN bar ON foo.x = bar.x",
          "DESCRIBE foo LEFT JOIN bar ON foo.x = bar.x",
      }
      for _, sql := range invalidSQLs {
          _, err := NewParser(sql).ParseStmts()
          require.Error(t, err, "Expected DESCRIBE-with-JOIN to fail: %s", sql)
      }
  }
  ```
- [x] 4.3 `go test ./parser/... -run 'TestParser_Describe_RichArguments|TestParser_Describe_RejectsJoin' -v -count=1` → expect PASS for both. If any positive SQL fails, STOP and investigate (most likely culprit is a quirk of `parseTableExpr` around alias/FINAL/SAMPLE precedence). If any negative SQL accidentally parses, STOP and investigate — that means `parseTableExpr` is consuming JOIN tokens it shouldn't.

## 5. Add new `.sql` fixtures

- [x] 5.1 Create `parser/testdata/ddl/describe_subquery.sql` with:
  ```
  DESCRIBE (SELECT 1 AS x, 2 AS y)
  ```
- [x] 5.2 Create `parser/testdata/ddl/describe_with_alias.sql` with:
  ```
  DESCRIBE foo AS f
  ```
- [x] 5.3 Create `parser/testdata/ddl/describe_table_function.sql` with:
  ```
  DESCRIBE numbers(10)
  ```
  (No `describe_join.sql` fixture — JOIN at the DESCRIBE position is explicitly rejected per Decisions 1 and 6 in design.md, covered by `TestParser_Describe_RejectsJoin` instead.)

## 6. Generate and inspect new fixture goldens

- [x] 6.1 Run `go test ./parser/... -run 'TestParser_ParseStatements/(describe_subquery\.sql|describe_with_alias\.sql|describe_table_function\.sql)$' -count=1 -update`. Visually inspect each generated `.golden.json`: `Target` should be a `*TableExpr` wrapping the expected concrete sub-node (`*SubQuery` for the subquery case, `*TableIdentifier` with non-nil `Alias` for the aliased case, `*TableFunctionExpr` for the table-function case).
- [x] 6.2 Run `go test ./parser/... -run 'TestParser_Format/(describe_subquery\.sql|describe_with_alias\.sql|describe_table_function\.sql)$' -count=1 -update`. Visually inspect each formatted output for sensible whitespace and clause ordering.
- [x] 6.3 Run `go test ./parser/... -run 'TestParser_FormatBeautify/(describe_subquery\.sql|describe_with_alias\.sql|describe_table_function\.sql)$' -count=1 -update`. Visually inspect each beautified file.

## 7. Regenerate the six pre-existing affected goldens

- [x] 7.1 Run `go test ./parser/... -run 'TestParser_ParseStatements/(desc|describe)_table_(with|without)_table_keyword\.sql$|TestParser_ParseStatements/describe_table_with_settings\.sql$|TestParser_ParseStatements/describe_settings_multiple\.sql$' -count=1 -update`.
- [x] 7.2 For each of the six regenerated files, diff against the `/tmp/` snapshot from task 1.3:
  ```bash
  for f in describe_table_with_table_keyword describe_table_without_table_keyword \
           desc_table_with_table_keyword desc_table_without_table_keyword \
           describe_table_with_settings describe_settings_multiple; do
    echo "=== $f ==="
    diff /tmp/${f}.before.json parser/testdata/ddl/output/${f}.sql.golden.json
  done
  ```
  Confirm each diff is the same structural shift: `Target` bare `*TableIdentifier` JSON becomes a `*TableExpr` JSON object with `TablePos`, `TableEnd`, `Alias: null`, `Expr: { …the same TableIdentifier… }`, `HasFinal: false`. No positional drift in the inner TableIdentifier. No missing or extra fields.
- [x] 7.3 Run `go test ./parser/... -run 'TestParser_Format|TestParser_FormatBeautify' -count=1`. If any of the six DESCRIBE/DESC format/beautify goldens fail, regenerate via `-update` and visually inspect — the formatter may render `*TableExpr` with subtly different whitespace than bare `*TableIdentifier`; that's acceptable as long as the new output is sensible. If any other (non-DESCRIBE) golden fails, STOP and investigate.

## 8. Verify the feature contract and regression guards

- [x] 8.1 `go test ./parser/... -run 'TestParser_Describe_RichArguments|TestParser_Describe_RejectsJoin' -v -count=1` → expect both to PASS.
- [x] 8.2 `go test ./parser/... -run 'TestParser_With_DescribeSettings' -v -count=1` → must remain PASS (existing inline test from the previous DESCRIBE-SETTINGS change).
- [x] 8.3 `go test ./parser/... -run 'TestParser_InvalidSyntax' -v -count=1` → must remain PASS.
- [x] 8.4 `go test ./parser/... -run 'TestParser_ParseStatements|TestParser_Format|TestParser_FormatBeautify' -count=1` → all goldens match (the six regenerated + three new + all unaffected fixtures).
- [x] 8.5 `go test ./parser/... -count=1` → full suite passes. Confirm the only deltas from the previous full-suite run are: nine new sub-tests under `TestParser_ParseStatements`/`TestParser_Format`/`TestParser_FormatBeautify` for the three new fixtures (all PASS), six regenerated goldens (their sub-tests still PASS against the new shape), and a new top-level `TestParser_Describe_RichArguments` (PASS). Nothing previously passing moves to fail.

## 9. Close out

- [x] 9.1 `go vet ./parser/...` produces no new warnings (the pre-existing `WriteByte` notice in `parser/format.go` is acceptable).
- [x] 9.2 `openspec validate broaden-describe-arguments` reports the change as valid.
- [x] 9.3 Delete the `/tmp/*.before.json` snapshots from task 1.3.
