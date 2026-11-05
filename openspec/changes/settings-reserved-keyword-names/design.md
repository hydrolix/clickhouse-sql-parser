# Design: SETTINGS Item Names MAY Be Reserved Keywords

## Context

`parseSettingsExpr` (`parser/parser_table.go`) reads the name with `parseIdent`, i.e.
`expectTokenKind(TokenKindIdent)`, which since upstream #282 rejects reserved keywords.
`parseSettingsList`, `parseSettingsClause` (used by `SET`), the dictionary settings clause and
`ALTER TABLE ... MODIFY SETTING` all delegate to `parseSettingsExpr`; no other site reads a setting name.

## Decisions

### D1 — Read the name with `parseAnyKeyword`

```go
ident, err := p.parseAnyKeyword()
```

`parseAnyKeyword` accepts `TokenKindIdent` or `TokenKindKeyword`, consumes it, and returns an `*Ident`
with the token's raw text and quote type. No AST, formatter or walker change; the formatter prints the
source spelling (`limit`, not `LIMIT`).

**Why it is safe:** a SETTINGS list contains only `name = value` pairs. The item name is always the first
token after `SETTINGS` or a comma, and is always followed by `=`, so no keyword in that position can start
another clause. There is no ambiguity to resolve.

Non-ident, non-keyword tokens (`5`, `'limit'`, `=`) are still rejected by `parseAnyKeyword`.

## Golden footprint

New fixture `parser/testdata/query/select_with_reserved_keyword_settings.sql` with JSON, format and
beautify goldens. No existing golden changes.
