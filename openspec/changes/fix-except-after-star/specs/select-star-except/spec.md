# select-star-except Specification

## ADDED Requirements

### Requirement: EXCEPT after a star-like select item SHALL be a column modifier

When the select item expression is `*` or `COLUMNS(...)`, the parser SHALL treat a following
`EXCEPT` as a column modifier regardless of the kind of the token after it. The modifier SHALL
be a `*FunctionExpr` whose `Name.Name` is `EXCEPT` and whose `Params` hold the column list.

#### Scenario: Keyword-named column
- **WHEN** `SELECT timestamp, * EXCEPT timestamp FROM t` is parsed
- **THEN** `ParseStmts` returns no error AND the second select item has one modifier
  `EXCEPT` with the single parameter `timestamp`

#### Scenario: Other keyword-named columns
- **WHEN** `SELECT * EXCEPT date FROM t` and `SELECT * EXCEPT key FROM t` are parsed
- **THEN** both return no error AND each star item has one `EXCEPT` modifier with one
  parameter

#### Scenario: COLUMNS then EXCEPT
- **WHEN** `SELECT COLUMNS('^a') EXCEPT timestamp FROM t` is parsed
- **THEN** `ParseStmts` returns no error AND the item has one `EXCEPT` modifier

#### Scenario: Chained EXCEPT modifiers
- **WHEN** `SELECT * EXCEPT timestamp EXCEPT b FROM t` is parsed
- **THEN** `ParseStmts` returns no error AND the star item has two `EXCEPT` modifiers

### Requirement: EXCEPT STRICT SHALL be recognised after a star-like item

The parser SHALL consume an identifier spelled `STRICT` (case-insensitive) as the mode
when it follows `EXCEPT` after a star-like item, and SHALL then parse either one
identifier or a parenthesised column list. The modifier's `Name.Name` SHALL be `EXCEPT STRICT` and
`Name.NameEnd` SHALL be the end of the `STRICT` token.

#### Scenario: STRICT bare and list
- **WHEN** `SELECT * EXCEPT STRICT timestamp FROM t` and
  `SELECT * EXCEPT STRICT (timestamp, key) FROM t` are parsed
- **THEN** both return no error AND the modifier `Name.Name` is `EXCEPT STRICT` AND the
  parameters are `timestamp`, respectively `timestamp, key` AND the item has no alias

#### Scenario: STRICT round-trips through Format
- **WHEN** `SELECT * EXCEPT STRICT timestamp FROM t` is parsed and formatted
- **THEN** the output is `SELECT * EXCEPT STRICT(timestamp) FROM t` AND formatting the
  re-parsed output yields the same text

### Requirement: EXCEPT after a non-star select item SHALL be a set operator

When the select item expression is not star-like, the modifier loop SHALL NOT consume
`EXCEPT`; the select query parser SHALL then parse `EXCEPT [ALL | DISTINCT]` followed by a
`SELECT`, `WITH` or parenthesised query as a set operation.

#### Scenario: Set operator matrix still parses as set operation
- **WHEN** `SELECT 1 EXCEPT SELECT 2`, `SELECT 1 EXCEPT ALL SELECT 2` and
  `SELECT 1 EXCEPT DISTINCT SELECT 2` are parsed
- **THEN** each returns no error AND the first `SelectQuery` has `Except` set AND its
  select item has no modifiers

#### Scenario: Parenthesised subquery after a non-star item is a set operation
- **WHEN** `SELECT 1 EXCEPT (SELECT 2)` is parsed
- **THEN** `ParseStmts` returns no error AND the first `SelectQuery` has `Except` set AND
  the item `1` has no modifiers

#### Scenario: Modifier then set operation after FROM
- **WHEN** `SELECT * EXCEPT timestamp FROM t EXCEPT ALL SELECT 3` is parsed
- **THEN** `ParseStmts` returns no error AND the star item has one `EXCEPT` modifier AND
  the `SelectQuery` has `Except` set with mode `ALL`

#### Scenario: Non-star item rejects EXCEPT ident
- **WHEN** `SELECT a EXCEPT b FROM t` is parsed
- **THEN** `ParseStmts` returns an error

### Requirement: Existing goldens SHALL be byte-identical

This change SHALL NOT alter the lexer, SHALL NOT add or rename an AST node or field, and
SHALL NOT change `APPLY` or `REPLACE` handling. Every golden file that existed before the
change SHALL match without `-update`.

#### Scenario: Pre-existing goldens remain green
- **WHEN** goldens are regenerated after the change
- **THEN** no existing JSON, format or beautify golden file changes
