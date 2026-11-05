# WITH-Prefixed Subqueries in Expressions

## Why

`SELECT (WITH 1000 AS latest SELECT max(x) FROM t WHERE y > latest) AS m` fails with
`expected the last token kind is: ), but got <int>`. Five queries in the dashboard SQL
corpus (DS2 Web Security and its copies) open with
`WITH ( WITH 1000 AS LATEST SELECT [...] FROM (...) ) AS min_max SELECT ...`, and
ClickHouse runs them.

In expression position `parseColumnExpr` recognises a subquery only when the token after
`(` is `SELECT`; a `(` followed by `WITH` is sent to `parseFunctionParams`, which reads
`WITH` as an identifier and then fails on the next token. The bare form has the same gap:
`exists(WITH 1 AS a SELECT a)` and `x IN (WITH 1 AS a SELECT a)` fail, while
`FROM (WITH 1 AS a SELECT a)` and a top-level `WITH 1000 AS latest SELECT 1` already parse
because those paths check for `WITH` too. ClickHouse accepts a WITH-prefixed subquery in
every expression position (verified on ClickHouse 26.8).

## What Changes

- **`(WITH … SELECT …)` is a parenthesised subquery in expressions.** It produces the same
  `*SubQuery` node as `(SELECT …)`.
- **A bare `WITH … SELECT …` in expression position is a select query**, the same way a
  bare `SELECT …` is today (function parameters such as `exists(…)`).
- Covered positions: scalar subquery in the select list, the right side of `IN`, a
  function argument, the value of a top-level `WITH … AS name` item.
- No change to `FROM (…)`, CTEs (`WITH name AS (…)`), or set operations.

## Capabilities

### New Capabilities

- `with-prefixed-subquery-expr`: WITH-prefixed subqueries in expression position.

### Modified Capabilities

None.

## Impact

- **AST:** no new node or field. `(WITH …)` yields `*SubQuery{HasParen: true}`; a bare
  `WITH …` yields `*SelectQuery`, matching the existing `SELECT` behaviour in each spot.
- **Goldens:** every existing JSON, format and beautify golden is byte-identical. New
  fixtures add new golden files only.
- **Consumers:** chproxy and the Grafana plugin need only a dependency bump. The five corpus
  queries in `clickhouse-sql-parser-dashboards/plain-sql` turn green.
- **Behaviour change:** an expression that starts with the keyword `WITH` used to be read as
  an identifier named `with` (and then failed on the next token in every real input); it is
  now parsed as a query. A column literally named `with` needs backticks, as in ClickHouse.
