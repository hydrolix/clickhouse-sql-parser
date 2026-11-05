# Aliases in `extract(...)` Arguments

## Why

`SELECT avg(toUInt32OrNull(extract(arrayJoin(splitByString('//', s)) AS part, '(\d+)'))) FROM t`
fails with `expected the last token kind is: ), but got <keyword>` at the `AS`. Four
queries in the dashboard SQL corpus (Edge Workers and its copy) alias an argument of the
ClickHouse regex function `extract(haystack, pattern)` and reuse the alias elsewhere in the
query; ClickHouse runs them.

Aliases inside ordinary function arguments already parse (`f(x AS y, 1)`), because
function parameters go through an alias-aware path that yields a `*ColumnExpr`. The
keyword `EXTRACT` is routed to `parseColumnExtractExpr` instead, so that the SQL-standard
`EXTRACT(unit FROM date)` form is recognised, and that routine parses each parameter with
plain `parseExpr`, which stops at `AS`. ClickHouse accepts an alias in both forms:
`extract(arrayJoin([...]) AS p, '(\d+)')` and `EXTRACT(YEAR FROM d AS alias)` (verified on
ClickHouse 26.8).

## What Changes

- **An `extract(...)` parameter MAY carry `AS alias`.** After each parameter, whether the
  `unit FROM expr` form or a plain expression, the parser accepts an optional alias and
  wraps the parameter in a `*ColumnExpr{Expr, Alias}`, the node ordinary function
  arguments already produce.
- Parameters without an alias are unchanged (same node, same positions).
- `Format` re-emits `… AS alias` through the existing `ColumnExpr` formatter.

## Capabilities

### New Capabilities

- `extract-arg-alias`: aliases on `extract(...)` parameters.

### Modified Capabilities

None.

## Impact

- **AST:** no new node or field. An aliased parameter is a `*ColumnExpr`; an unaliased one
  stays what it is today.
- **Goldens:** every existing JSON, format and beautify golden is byte-identical (no
  existing fixture aliases an `extract` parameter). New fixtures add new golden files only.
- **Consumers:** chproxy and the Grafana plugin need only a dependency bump. The four
  corpus queries in `clickhouse-sql-parser-dashboards/plain-sql` turn green.
