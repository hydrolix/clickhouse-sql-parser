# EXCEPT After a Star Is Always a Column Modifier

## Why

`SELECT timestamp, * EXCEPT timestamp FROM t` fails with
`expected SELECT, WITH or (, got <keyword>` although ClickHouse runs it (HDX-12821).
The modifier arm of `parseSelectItem` treats `EXCEPT` as a column modifier only when the
next token is `(` or an `<ident>` token. `timestamp`, `date`, `key`, `host`, `type` and
every other column name that the lexer classifies as a keyword fall through to the
set-operator path, which then demands `SELECT`, `WITH` or `(`. Fleet "Raw Logs"
dashboards use `* except timestamp`, and the Grafana plugin parses the query whenever an
ad hoc filter is applied, so the panel errors out (regression since plugin 0.11.0 /
parser v0.5.2).

ClickHouse resolves the ambiguity by position, not by the next token: directly after `*`,
`COLUMNS(...)` or a qualified star, `EXCEPT` is always a column transformer
(`SELECT * EXCEPT SELECT 1` is a syntax error there), and after any other select item it
is never one (`SELECT a EXCEPT b FROM t` is a syntax error). Verified on ClickHouse 26.8.

The same token-kind guard also produces two wrong trees today: `SELECT * EXCEPT STRICT ts`
parses as `EXCEPT(STRICT) AS ts`, and `SELECT 1 EXCEPT (SELECT 2)` parses as a modifier
`EXCEPT(SELECT 2)` on the item `1` instead of a set operation.

## What Changes

- **`EXCEPT` is a column modifier only after a star-like item.** When the select item is
  `*`, a qualified star (`t.*`, `db.t.*`) or `COLUMNS(...)`, `EXCEPT` starts a column modifier whatever the next token is. After
  any other item the modifier loop does not consume `EXCEPT`, so it reaches the existing
  set-operator path.
- **Keyword-named columns are accepted.** `* EXCEPT timestamp`, `* EXCEPT date`,
  `* EXCEPT key` parse as a one-column modifier, like `* EXCEPT (timestamp)`.
- **`STRICT` is recognised.** `* EXCEPT STRICT col` and `* EXCEPT STRICT (a, b)` parse with
  the mode recorded in the modifier's name (`EXCEPT STRICT`); `Format` re-emits it.
- **Set operations after a non-star item are parsed as such.** `SELECT 1 EXCEPT (SELECT 2)`
  becomes a set operation; `SELECT a EXCEPT b FROM t` becomes a parse error, as in
  ClickHouse.
- `APPLY` and `REPLACE` modifiers are not touched.

## Capabilities

### New Capabilities

- `select-star-except`: position-based resolution of `EXCEPT` after a star-like select
  item, keyword-named columns, and the `STRICT` mode.

### Modified Capabilities

- `select-except-bare-ident`: the bare-identifier form is no longer limited to `<ident>`
  tokens, and the "EXCEPT after a non-star item is a modifier" leniency is removed.

## Impact

- **AST:** no new node or field. The modifier stays a `*FunctionExpr`; with `STRICT` its
  `Name.Name` is `"EXCEPT STRICT"` and `Name.NameEnd` is the end of the `STRICT` token.
- **Goldens:** every existing JSON, format and beautify golden is byte-identical. New
  fixtures add new golden files only.
- **Behaviour change:** `SELECT <non-star> EXCEPT <ident>` used to parse as a modifier and
  now fails, matching ClickHouse. `SELECT <non-star> EXCEPT (SELECT …)` used to parse as a
  modifier and is now a set operation.
- **Consumers:** the Grafana plugin and chproxy need only a dependency bump. The
  `plain-sql/regressions.yaml` entries `HDX-12821/001..003` in the dashboards corpus turn
  green; `004..009` stay green.
- **Out of scope:** making the three-part `db.t.*` parse (separate change
  `qualified-star-path`; the star check already covers it); `APPLY`/`REPLACE` after a non-star item stays
  accepted (upstream fixture `select_item_with_modifiers.sql` relies on it).
