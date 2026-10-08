# Tasks: Float Values in SETTINGS

## 1. Baseline

- [x] 1.1 Record the current `go test ./... -count=1` result on the branch base
      (`feat/HDX-12552_settings-after-format` tip `50fc147`): all packages pass.
- [x] 1.2 Check with `clickhouse-local` which float forms ClickHouse accepts as setting
      values. Result (26.8): `0.5`, `.5`, `-.5`, `1.`, `1.5e-1`, and `0.5` after
      `FORMAT … SETTINGS` are accepted; `inf` / `nan` are rejected as values. Recorded in
      design.md Context.

## 2. Parser (D1, D2)

- [x] 2.1 `parser/parser_table.go` `parseSettingsExpr`: extend the number case to
      `p.matchTokenKind(TokenKindInt), p.matchTokenKind(TokenKindFloat),
      p.matchTokenKind(TokenKindDot)`. Body unchanged (`p.parseNumber(p.Pos())`).

## 3. Fixtures and goldens

- [x] 3.1 Add `parser/testdata/query/select_with_float_settings.sql`: every float form
      (`0.5`, `-0.5`, `+0.5`, `.5`, `1.`, `1.5e-3`), a mix with int/string values, and
      `FORMAT JSON SETTINGS x = 0.5`.
- [x] 3.2 Add `parser/testdata/ddl/create_table_with_float_settings.sql`
      (`CREATE TABLE … SETTINGS x = 0.5`) and
      `parser/testdata/ddl/alter_table_modify_float_setting.sql`
      (`ALTER TABLE t MODIFY SETTING x = 0.5`).
- [x] 3.3 Add `parser/testdata/basic/set_float_statement.sql` (`SET x = 0.5`).
- [x] 3.4 Run `make update_test`.
- [x] 3.5 Spot-check with `git status --porcelain parser/testdata`: only new files (the
      fixtures above and their JSON / format / beautify goldens); no existing golden
      modified. Check the new format goldens keep each literal verbatim (`.5`, `1.`,
      `1.5e-3`).

## 4. Inline tests

- [x] 4.1 `parser/parser_test.go`: table-driven `TestParser_With_FloatSettings` covering
      "Float value in SELECT SETTINGS", "Float forms keep their source text", "Float
      mixed with other values", "Float after FORMAT", "Float in SET", "Float in DDL
      settings lists", "Positions cover the float token", "Leading-dot position starts
      at the dot", "Integer value is unchanged" and "Non-number value still errors".
- [x] 4.2 In the same test, a round-trip case for "Round trip keeps the literal":
      `Format(Parse(sql))` equals the expected text and is a fixed point.
- [x] 4.3 "Existing goldens are byte-identical" is pinned by task 3.5.

## 5. Verification

- [x] 5.1 `go fmt ./...`, `go vet ./...`, `golangci-lint run` and `go test -race ./...
      -count=1` all pass.
- [x] 5.2 `openspec validate settings-float-values --strict` passes.
