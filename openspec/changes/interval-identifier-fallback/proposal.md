# `interval` as an Identifier

## Why

`SELECT toStartOfInterval(ts, INTERVAL 1 MINUTE) AS interval, count() FROM t GROUP BY interval ORDER BY interval`
fails with `unknown interval type: <"BY">` (or `unexpected token kind: <eof>` when the clause
is last). `interval` is a popular alias for a time bucket in dashboards; seven queries in the
dashboard SQL corpus (Edge Workers, DNS Info) use it this way, and ClickHouse runs them.

`parseColumnExpr` turns the keyword `INTERVAL` into an identifier only when the next token
is a comma or `AS`. In every other position it calls `parseInterval`, which demands
`<expr> <unit>`, so `GROUP BY interval`, `ORDER BY interval DESC`, `WHERE interval > 0`,
`SELECT interval FROM t` and `(interval)` all fail. ClickHouse tries the `INTERVAL <expr>
<unit>` literal first and otherwise reads `interval` as a plain identifier (verified on
ClickHouse 26.8).

## What Changes

- **Fallback to identifier.** When `INTERVAL` does not start a well-formed interval
  literal, the parser SHALL read it as an identifier named `interval`, in any expression
  position.
- **Interval literals are unchanged.** `INTERVAL 1 MINUTE`, `INTERVAL '1' MINUTE`,
  `INTERVAL (1) MINUTE`, `INTERVAL x DAY` and arithmetic with intervals parse as today and
  produce the same `IntervalExpr`.
- **Error wording for a malformed literal changes.** `INTERVAL 1 FOO` no longer reports
  `unknown interval type`; it falls back to the identifier and fails at the next token.

## Capabilities

### New Capabilities

- `interval-identifier-fallback`: `interval` parses as an identifier wherever it is not a
  valid interval literal.

### Modified Capabilities

None.

## Impact

- **AST:** no new node or field. The identifier is an ordinary `*Ident`.
- **Goldens:** every existing JSON, format and beautify golden is byte-identical. New
  fixtures add new golden files only.
- **Consumers:** chproxy and the Grafana plugin need only a dependency bump. The seven
  corpus queries in `clickhouse-sql-parser-dashboards/plain-sql` turn green.
- **Performance:** a failed literal attempt re-lexes a few tokens; only inputs that use
  `interval` as a name pay it.
- **Out of scope:** `INTERVAL '1 minute'` (unit inside the string), which ClickHouse
  accepts and the parser rejects today; it stays rejected (now as a fallback error).
