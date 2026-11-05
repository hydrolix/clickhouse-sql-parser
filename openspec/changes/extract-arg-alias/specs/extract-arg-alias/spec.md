# extract-arg-alias Specification

## ADDED Requirements

### Requirement: An extract parameter SHALL accept an alias

The parser SHALL accept `AS alias` after any parameter of `extract(...)` / `EXTRACT(...)`,
for the plain-expression form and for the `unit FROM expr` form, and SHALL represent an
aliased parameter as a `*ColumnExpr` whose `Expr` is the parameter and whose `Alias` is
the identifier.

#### Scenario: Alias on a regex extract argument
- **WHEN** `SELECT avg(toUInt32OrNull(extract(arrayJoin(splitByString('//', s)) AS part, '(\d+)'))) AS v, part FROM t`
  is parsed
- **THEN** `ParseStmts` returns no error AND the `ExtractExpr`'s first parameter is a
  `*ColumnExpr` with `Alias.Name` `part` AND its second parameter is the string literal

#### Scenario: Alias on the haystack argument
- **WHEN** `SELECT extract(s AS src, '\d+') AS n, src FROM t` is parsed
- **THEN** `ParseStmts` returns no error AND the first parameter is a `*ColumnExpr` with
  `Alias.Name` `src`

#### Scenario: Alias inside EXTRACT unit FROM expr
- **WHEN** `SELECT EXTRACT(YEAR FROM d AS day), day FROM t` is parsed
- **THEN** `ParseStmts` returns no error AND the single parameter is a `*ColumnExpr` with
  `Alias.Name` `day` whose `Expr` is an `*IntervalFrom`

### Requirement: Unaliased extract parameters SHALL parse unchanged

A parameter without `AS` SHALL produce the same node kind and positions as before this
change.

#### Scenario: Unaliased parameters unchanged
- **WHEN** `SELECT extract(s, '\d+'), EXTRACT(MONTH FROM d) FROM t` is parsed
- **THEN** `ParseStmts` returns no error AND no parameter of either `ExtractExpr` is a
  `*ColumnExpr`

### Requirement: Aliased extract parameters SHALL round-trip through Format

`Format` SHALL emit the parameter as `expr AS alias` so that parse → format is a fixed
point.

#### Scenario: Round trip keeps the alias
- **WHEN** `SELECT avg(toUInt32OrNull(extract(arrayJoin(splitByString('//', s)) AS part, '(\d+)'))) AS v, part FROM t`
  is parsed and formatted
- **THEN** the output contains `AS part` AND formatting the re-parsed output yields the
  same text

### Requirement: Existing goldens SHALL be byte-identical

This change SHALL NOT alter the lexer, SHALL NOT add or rename an AST node or field, and
SHALL NOT change how `EXTRACT` is dispatched. Every golden file that existed before the
change SHALL match without `-update`.

#### Scenario: Pre-existing goldens remain green
- **WHEN** goldens are regenerated after the change
- **THEN** no existing JSON, format or beautify golden file changes
