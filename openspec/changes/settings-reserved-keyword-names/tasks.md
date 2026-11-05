# Tasks: SETTINGS Item Names MAY Be Reserved Keywords

## 1. Parser (D1)

- [x] 1.1 `parser/parser_table.go` `parseSettingsExpr`: read the name with `parseAnyKeyword`.
- [x] 1.2 Verify `SET`, dictionary settings and `ALTER TABLE ... MODIFY SETTING` delegate to
      `parseSettingsExpr`; no further change needed.

## 2. Fixtures and goldens

- [x] 2.1 Add `parser/testdata/query/select_with_reserved_keyword_settings.sql`.
- [x] 2.2 Run `make update_test`; only the new fixture and its JSON / format / beautify goldens appear.

## 3. Inline tests

- [x] 3.1 `parser/parser_test.go`: `TestParser_With_ReservedKeywordSettingNames` with subtests named as the
      spec Scenarios.

## 4. Changelog and verification

- [x] 4.1 `CHANGELOG.md`: one `**Fix**` line under "Unreleased".
- [x] 4.2 `gofmt`, `go vet`, `make lint`, `make test` pass.
- [x] 4.3 `openspec validate settings-reserved-keyword-names --strict` passes.
