# SETTINGS Item Names MAY Be Reserved Keywords

## Why

`SELECT 1 SETTINGS limit = 5`, `SETTINGS offset = 10` and `SETTINGS final = 1` fail with
`expected '<ident>', but got '<keyword>'`. `limit`, `offset` and `final` are real ClickHouse
query-level settings, and ClickHouse accepts any name in a SETTINGS list.

This regressed with the upstream v0.5.6 merge: upstream #282 split keywords into reserved and
non-reserved sets and made `LIMIT`, `OFFSET` and `FINAL` reserved, so `matchTokenKind(TokenKindIdent)`
rejects them. `parseSettingsExpr` read the item name with `parseIdent`, which goes through that check.
Before the merge every keyword matched `TokenKindIdent`.

## What Changes

- **A SETTINGS item name MAY be any identifier or keyword**, reserved or not, quoted or not.
- The value side, the `=` requirement and every other clause are unchanged.

## Capabilities

### New Capabilities

- `settings-reserved-keyword-names`: reserved keywords as SETTINGS item names.

### Modified Capabilities

None.

## Impact

- **AST:** none. `SettingExpr.Name` stays an `*Ident` carrying the token's source text.
- **Paths covered:** `parseSettingsExpr` is the single name reader for `SELECT ... SETTINGS`,
  `... FORMAT x SETTINGS`, `SET`, DDL/engine `SETTINGS`, `ALTER TABLE ... MODIFY SETTING` and
  dictionary `SETTINGS(...)`, so all of them are fixed together.
- **Goldens:** existing fixtures are byte-identical; one new fixture adds new golden files only.
