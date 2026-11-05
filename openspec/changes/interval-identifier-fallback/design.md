# Design: `interval` as an Identifier

## Context

`parseColumnExpr` (`parser/parser_column.go`) starts with a rule that reads any keyword
as an identifier when the next token is a comma or `AS`, then dispatches on the current
token. `case p.matchKeyword(KeywordInterval)` calls `parseInterval(true)`, which consumes
the keyword, parses an expression with `parseExpr`, then `parseIdent` for the unit and
checks it against `intervalUnits`. Any failure is returned as the expression's error.

The lexer exposes `saveState()` / `restoreState()`, and the parser already backtracks with
them in `parser_common.go:93` and `parser_query.go:869`. `Parser` holds only the lexer, so
restoring the lexer restores the whole parse position.

The two other `KeywordInterval` sites (`parseFrameInterval` in window frames, and the
window-frame lookahead at `parser_query.go:883`) are inside window frame grammar where
`interval` cannot be an identifier; they are not touched.

ClickHouse 26.8 (`clickhouse local`):

| Input | Result |
| --- | --- |
| `SELECT 1 AS interval GROUP BY interval ORDER BY interval` | ok |
| `SELECT interval FROM (SELECT 1 AS interval)` | ok |
| `SELECT interval + 1 FROM …`, `WHERE interval > 0`, `ORDER BY interval DESC` | ok |
| `SELECT (interval) FROM …`, `SELECT t.interval FROM … AS t` | ok |
| `SELECT INTERVAL 1 MINUTE`, `INTERVAL '1' MINUTE`, `INTERVAL (1) MINUTE`, `INTERVAL x MINUTE` | ok |
| `SELECT interval` (no such column) | semantic error, not a syntax error |

## Goals / Non-Goals

**Goals:**

- `interval` parses as an identifier wherever it is not a valid interval literal.
- Interval literals keep their AST and goldens.

**Non-Goals:**

- `INTERVAL '1 minute'` with the unit inside the string.
- Treating other keywords as identifiers; only `INTERVAL` is affected.
- A dedicated error message for a malformed interval literal.

## Decisions

### D1 — Try the literal, fall back to an identifier

In `parseColumnExpr`:

```go
case p.matchKeyword(KeywordInterval):
    return p.parseIntervalOrIdent()
```

`parseIntervalOrIdent` saves the lexer state, calls `parseInterval(true)`, and on error
restores the state and returns `p.parseIdent()`.

- **Why try-parse:** it mirrors ClickHouse's own resolution and needs no enumeration of
  expression starts. `INTERVAL (x + 1) MINUTE` keeps working because the attempt parses a
  full expression.
- **Why restore on any error:** `parseInterval` fails either because the expression does
  not parse (`GROUP BY interval, x`, `interval > 0`, `interval FROM`) or because the unit
  is not an interval unit (`interval DESC`, `interval LIMIT`). Both mean the token was a
  name.
- **Alternative — two-token lookahead** (literal only if the next token is a number,
  string, variable, `(` or an identifier followed by a unit): no backtracking, but it must
  list every expression start and cannot see past a parenthesised expression. Rejected.

### D2 — No change to the comma / AS rule

The existing "keyword followed by a comma or `AS` is an identifier" rule stays; it also
covers other keywords and is cheaper than a failed literal attempt.

## Risks / Trade-offs

- [Malformed literal reports a later, generic error] → `INTERVAL 1 FOO` used to say
  `unknown interval type: <"FOO">`; now the fallback reads `interval` as a name and the
  statement fails at `1`. Still an error; the message is less specific. Accepted.
- [Cost of the failed attempt] → a handful of tokens re-lexed, only when `interval` is used
  as a name.
- [A future `parseInterval` that succeeds on a non-literal] → covered by the fixture that
  pins `ORDER BY interval DESC` and `WHERE interval > 0`.

## Golden footprint

- Existing fixtures: no change in any golden family.
- New fixture `parser/testdata/query/select_interval_as_identifier.sql` with JSON, format
  and beautify goldens.
