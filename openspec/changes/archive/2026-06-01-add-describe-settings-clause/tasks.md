## 1. Baseline

- [x] 1.1 Confirm `parser/parser_test.go` contains `TestParser_With_DescribeSettings` exercising `DESCRIBE TABLE foo SETTINGS …` and `DESCRIBE foo SETTINGS k1=v1, k2=v2`. Starting state: FAIL.
- [x] 1.2 Confirm `parser/testdata/ddl/describe_table_with_table_keyword.sql` (`DESCRIBE TABLE mytable`) and `parser/testdata/ddl/describe_table_without_table_keyword.sql` (`DESCRIBE mytable`) exist with their three goldens each.
- [x] 1.3 Capture the baseline: `go test ./parser/... -run 'TestParser_With_DescribeSettings|TestParser_InvalidSyntax|TestParser_ParseStatements|TestParser_Format|TestParser_FormatBeautify' -count=1`. `TestParser_With_DescribeSettings` must FAIL; everything else must PASS.
- [x] 1.4 Snapshot the current `describe_table_with_table_keyword.sql.golden.json` content for post-change visual comparison: `cp parser/testdata/ddl/output/describe_table_with_table_keyword.sql.golden.json /tmp/describe_table_with_table_keyword.before.json` (and the no-TABLE variant). These backups are deleted in task 8.x; they exist only to make the one-line shift visually verifiable.

## 2. AST changes in `parser/ast.go`

- [x] 2.1 Locate `type DescribeStmt struct { … }` (around line 6289). Add a new field `Settings *SettingsClause` after `Target *TableIdentifier`. Do NOT add `json:",omitempty"` or `json:"-"` tags — match the existing convention where every field renders explicitly.
- [x] 2.2 Update `func (d *DescribeStmt) End() Pos` to return `d.Settings.End()` when `d.Settings != nil`, otherwise `d.Target.End()`.
- [x] 2.3 Update `func (d *DescribeStmt) Accept(visitor ASTVisitor) error` to traverse `d.Settings` after `d.Target` when non-nil, before calling `visitor.VisitDescribeExpr(d)`. Mirror the pattern used by other statements with optional sub-trees (e.g. `SelectQuery.Accept` for its `Format` clause).
- [x] 2.4 `go build ./parser/...` to confirm the package compiles.

## 3. Parser change in `parser/parser_table.go`

- [x] 3.1 Locate `func (p *Parser) parseDescribeStmt(pos Pos) (*DescribeStmt, error)` (around line 1649). After the `tableIdent, err := p.parseTableIdentifier(p.Pos())` block, add:
  ```go
  settings, err := p.tryParseSettingsClause(p.Pos())
  if err != nil {
      return nil, err
  }
  ```
- [x] 3.2 Compute the statement end from the settings clause when present:
  ```go
  statementEnd := tableIdent.End()
  if settings != nil {
      statementEnd = settings.End()
  }
  ```
- [x] 3.3 In the returned struct literal, set `StatementEnd: statementEnd` (replacing the current `tableIdent.End()`) and add `Settings: settings` to the field list.
- [x] 3.4 `go build ./parser/...`.

## 4. Verify the two existing DESCRIBE goldens diff exactly as expected

- [x] 4.1 Run `go test ./parser/... -run 'TestParser_ParseStatements/describe_table_(with|without)_table_keyword\.sql$' -count=1`. Expected: BOTH fail because the new `Settings: nil` field is missing from the goldens.
- [x] 4.2 Re-run with `-update`: `go test ./parser/... -run 'TestParser_ParseStatements/describe_table_(with|without)_table_keyword\.sql$' -count=1 -update`. Both goldens are now updated.
- [x] 4.3 Visually diff each against the snapshot from task 1.4:
  ```bash
  diff /tmp/describe_table_with_table_keyword.before.json parser/testdata/ddl/output/describe_table_with_table_keyword.sql.golden.json
  diff /tmp/describe_table_without_table_keyword.before.json parser/testdata/ddl/output/describe_table_without_table_keyword.sql.golden.json
  ```
  The diff MUST be exactly one new line (`"Settings": null,` or similar) per file. If any other field changed (positions shifted, ordering changed, etc.) STOP and investigate.
- [x] 4.4 Run `go test ./parser/... -run 'TestParser_Format/describe_table_(with|without)_table_keyword\.sql$|TestParser_FormatBeautify/describe_table_(with|without)_table_keyword\.sql$' -count=1`. Both must PASS without `-update` — the formatter does not emit anything for nil `Settings`, so these goldens remain byte-identical.

## 5. Add new `.sql` fixture inputs

- [x] 5.1 Create `parser/testdata/ddl/describe_table_with_settings.sql` with the single line:
  ```
  DESCRIBE TABLE foo SETTINGS describe_compact_output=1
  ```
- [x] 5.2 Create `parser/testdata/ddl/describe_settings_multiple.sql` with the single line:
  ```
  DESCRIBE foo SETTINGS describe_compact_output=1, describe_include_subcolumns=1
  ```

## 6. Generate and inspect the new goldens

- [x] 6.1 Run `go test ./parser/... -run 'TestParser_ParseStatements/(describe_table_with_settings\.sql|describe_settings_multiple\.sql)$' -count=1 -update`. **Visually inspect each generated JSON** to confirm `DescribeStmt.Settings` is populated with the expected `SettingsClause` structure (an `Items` array with one or two entries, each having a `Name` and `Expr`).
- [x] 6.2 Run `go test ./parser/... -run 'TestParser_Format/(describe_table_with_settings\.sql|describe_settings_multiple\.sql)$' -count=1 -update`. **Visually inspect each generated `.sql`** to confirm the formatter renders the SETTINGS clause correctly (e.g. `DESCRIBE TABLE foo SETTINGS describe_compact_output = 1;`).
- [x] 6.3 Run `go test ./parser/... -run 'TestParser_FormatBeautify/(describe_table_with_settings\.sql|describe_settings_multiple\.sql)$' -count=1 -update`. **Visually inspect** each beautified file.
- [x] 6.4 Re-run all three commands without `-update`: `go test ./parser/... -run 'TestParser_ParseStatements|TestParser_Format|TestParser_FormatBeautify' -count=1`. All goldens (regenerated + new) must pass.

## 7. Verify the feature contract and regression guards

- [x] 7.1 `go test ./parser/... -run 'TestParser_With_DescribeSettings' -v -count=1` → expect PASS for both SQLs in the test.
- [x] 7.2 `go test ./parser/... -run 'TestParser_InvalidSyntax' -v -count=1` → must PASS.
- [x] 7.3 `go test ./parser/... -run 'TestParser_ParseStatements' -count=1` → all goldens match (including the two regenerated and two new fixtures).
- [x] 7.4 `go test ./parser/... -run 'TestParser_Format' -count=1` → all formatter goldens match.
- [x] 7.5 `go test ./parser/... -run 'TestParser_FormatBeautify' -count=1` → all beautify goldens match.

## 8. Close out

- [x] 8.1 `go test ./parser/... -count=1`. Confirm the deltas from the previous full-suite run: `TestParser_With_DescribeSettings` transitions FAIL → PASS; two pre-existing DESCRIBE JSON goldens shift by one line each (regenerated); two new fixtures × 3 goldens = 6 new golden sub-tests appear and PASS. Nothing previously passing moves to fail.
- [x] 8.2 `go vet ./parser/...` produces no new warnings (the pre-existing `WriteByte` notice in `parser/format.go` is acceptable).
- [x] 8.3 `openspec validate add-describe-settings-clause` reports the change as valid.
- [x] 8.4 Delete the temporary `/tmp/describe_table_*.before.json` snapshots from task 1.4.
