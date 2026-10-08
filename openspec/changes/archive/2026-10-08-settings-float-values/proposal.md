# Float Values in SETTINGS

## Why

The parser rejects a `SETTINGS` item whose value is a decimal number with a fractional
part or a leading dot. `SELECT 1 SETTINGS max_bytes_ratio_before_external_group_by = 0.5`
fails with `unexpected token: "0.5", expected <number>, <bool> or <string>`, although
ClickHouse runs it. The same happens for `SET x = 0.5`, `… FORMAT JSON SETTINGS x = 0.5`,
`CREATE TABLE … SETTINGS x = 0.5` and the other statements that take a settings list.
Integer values (`1`, `-1`, `1e-3`) already parse.

ClickHouse has float-typed settings (for example `max_bytes_ratio_before_external_group_by`),
and it also accepts a float for an integer setting. chproxy (the Hydrolix http-proxy) parses
every cacheable query with this parser to build cache keys, strip proxy-only settings and
detect non-deterministic functions (HDX-12521). A query with a float setting value fails
to parse, so chproxy can't do any of that for it.

Upstream `AfterShip/clickhouse-sql-parser` has the same limitation.

## What Changes

- **Float setting values parse.** A `SETTINGS` item accepts a decimal float literal:
  `0.5`, `-0.5`, `+0.5`, `.5`, `1.`, `1.5e-3`. The value is a `NumberLiteral`, the same
  node integers already produce, with `Literal` holding the source text.
- **Every settings list gets it.** `SELECT … SETTINGS`, `… FORMAT … SETTINGS`, `SET`,
  `CREATE TABLE … SETTINGS`, `ALTER TABLE … MODIFY SETTING` and dictionary
  `SETTINGS(…)` share the same item grammar, so all of them accept floats.
- **Round trip keeps the literal.** `Format` and `Beautify` re-emit the value as written,
  so parse → format stays a fixed point.

## Capabilities

### New Capabilities

- `settings-float-values`: parsing and formatting of float literals as `SETTINGS` item
  values.

### Modified Capabilities

None.

## Impact

- **AST:** no shape change. A float value is a `NumberLiteral`, which the AST already has.
  No exported field is added, renamed or reordered.
- **Goldens:** JSON, format and beautify goldens are byte-identical for every existing
  fixture (no existing fixture has a float setting value, since none of them parsed).
  New fixtures add new golden files only.
- **Consumers:** chproxy needs no code change. It reads non-string setting values through
  `Format`, and a `NumberLiteral`'s end position is already the end of the token. chproxy
  only bumps the dependency.
- **Performance:** one more token-kind comparison per settings item.
- **Rollback:** revert the commit; inputs that parsed before still parse the same way.
- **Out of scope:** `-.5` / `+.5` (sign before a leading dot), hexadecimal floats
  (`0x1p3`), and `inf` / `nan` as setting values. See design.md.
