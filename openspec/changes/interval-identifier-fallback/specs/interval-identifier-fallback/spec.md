# interval-identifier-fallback Specification

## ADDED Requirements

### Requirement: INTERVAL SHALL fall back to an identifier when it is not a literal

The parser SHALL read the keyword `INTERVAL` as an identifier named `interval` in any
expression position where it does not start a well-formed `INTERVAL <expr> <unit>`
literal. The identifier SHALL be an ordinary `*Ident`.

#### Scenario: interval alias in GROUP BY and ORDER BY
- **WHEN** `SELECT toStartOfInterval(ts, INTERVAL 1 MINUTE) AS interval, count() FROM t GROUP BY interval ORDER BY interval`
  is parsed
- **THEN** `ParseStmts` returns no error AND the GROUP BY and ORDER BY expressions are
  `*Ident` with `Name` `interval`

#### Scenario: interval in WHERE and ORDER BY DESC
- **WHEN** `SELECT interval FROM t WHERE interval > 0 ORDER BY interval DESC` is parsed
- **THEN** `ParseStmts` returns no error AND the select item is an `*Ident` `interval`

#### Scenario: interval in select list and parentheses
- **WHEN** `SELECT (interval), interval + 1 FROM t` is parsed
- **THEN** `ParseStmts` returns no error

### Requirement: Interval literals SHALL parse unchanged

`INTERVAL <expr> <unit>` SHALL still produce an `*IntervalExpr` with the same fields as
before, for a number, string, parenthesised or identifier expression and for interval
arithmetic.

#### Scenario: Interval literals unchanged
- **WHEN** `SELECT now() - INTERVAL 1 HOUR, INTERVAL (1) MINUTE, INTERVAL x DAY FROM t`
  is parsed
- **THEN** `ParseStmts` returns no error AND each `INTERVAL …` is an `*IntervalExpr`

#### Scenario: Malformed literal still errors
- **WHEN** `SELECT INTERVAL 1 FOO` is parsed
- **THEN** `ParseStmts` returns an error

### Requirement: The alias SHALL round-trip through Format

`Format` SHALL emit the identifier as `interval` so that parse → format is a fixed point.

#### Scenario: Round trip keeps the alias
- **WHEN** `SELECT 1 AS interval FROM t GROUP BY interval ORDER BY interval` is parsed and
  formatted
- **THEN** the output is `SELECT 1 AS interval FROM t GROUP BY interval ORDER BY interval`
  AND formatting the re-parsed output yields the same text

### Requirement: Existing goldens SHALL be byte-identical

This change SHALL NOT alter the lexer, SHALL NOT add or rename an AST node or field, and
SHALL NOT touch window-frame `INTERVAL` parsing. Every golden file that existed before the
change SHALL match without `-update`.

#### Scenario: Pre-existing goldens remain green
- **WHEN** goldens are regenerated after the change
- **THEN** no existing JSON, format or beautify golden file changes
