## Purpose

Accept a decimal float literal as the value of a `SETTINGS` item (`0.5`, `-0.5`, `+0.5`, `.5`, `1.`, `1.5e-3`), as ClickHouse does, in every settings list: `SELECT … SETTINGS`, `… FORMAT … SETTINGS`, `SET`, `CREATE TABLE … SETTINGS`, `ALTER TABLE … MODIFY SETTING` and dictionary `SETTINGS(…)`. Before this, `SELECT 1 SETTINGS max_bytes_ratio_before_external_group_by = 0.5` failed with `unexpected token: "0.5", expected <number>, <bool> or <string>`, so chproxy couldn't build cache keys, strip proxy-only settings or detect non-deterministic functions for such queries (HDX-12521). The value is a `*NumberLiteral` that keeps its source text, round-trips through `Format`/`Beautify` unchanged, and leaves every previously accepted settings value untouched.

## Requirements

### Requirement: SETTINGS items SHALL accept decimal float values

A `SettingExpr` value SHALL accept a decimal float literal: digits with a fractional part
(`0.5`), a sign folded into the number (`-0.5`, `+0.5`), a leading dot (`.5`), a trailing
dot (`1.`), and an exponent (`1.5e-3`). The value SHALL be a `*NumberLiteral` whose
`Literal` is the source text of the number. This SHALL hold in every settings list:
`SELECT … SETTINGS`, `… FORMAT … SETTINGS`, `SET`, `CREATE TABLE … SETTINGS`,
`ALTER TABLE … MODIFY SETTING`, and dictionary `SETTINGS(…)`.

#### Scenario: Float value in SELECT SETTINGS
- **WHEN** `SELECT 1 SETTINGS max_bytes_ratio_before_external_group_by = 0.5` is parsed
- **THEN** `ParseStmts` returns no error AND the item's `Expr` is a `*NumberLiteral`
  with `Literal` `"0.5"`

#### Scenario: Float forms keep their source text
- **WHEN** each of `-0.5`, `+0.5`, `.5`, `1.`, `1.5e-3` is parsed as the value in
  `SELECT 1 SETTINGS x = <value>`
- **THEN** `ParseStmts` returns no error AND the item's `Expr` is a `*NumberLiteral`
  whose `Literal` equals `<value>`

#### Scenario: Float mixed with other values
- **WHEN** `SELECT 1 SETTINGS a = 0.5, b = 1, c = 'x'` is parsed
- **THEN** `Settings` has three items AND `a`'s value is a `*NumberLiteral` `"0.5"`

#### Scenario: Float after FORMAT
- **WHEN** `SELECT 1 FORMAT JSON SETTINGS x = 0.5` is parsed
- **THEN** `FormatSettings` has one item whose value is a `*NumberLiteral` `"0.5"`

#### Scenario: Float in SET
- **WHEN** `SET x = 0.5` is parsed
- **THEN** `ParseStmts` returns no error AND the `SetStmt`'s settings hold `x` with a
  `*NumberLiteral` `"0.5"`

#### Scenario: Float in DDL settings lists
- **WHEN** `CREATE TABLE t (a Int32) ENGINE = MergeTree ORDER BY a SETTINGS x = 0.5` and
  `ALTER TABLE t MODIFY SETTING x = 0.5` are parsed
- **THEN** both return no error AND the setting value is a `*NumberLiteral` `"0.5"`

#### Scenario: Positions cover the float token
- **WHEN** `SELECT 1 SETTINGS x = -0.5` is parsed
- **THEN** the value's `Pos()` is the offset of `-` AND its `End()` is one past the
  final `5` AND `SettingsClause.ListEnd` equals that `End()`

#### Scenario: Leading-dot position starts at the dot
- **WHEN** `SELECT 1 SETTINGS x = .5` is parsed
- **THEN** the value's `Pos()` is the offset of `.` AND its `End()` is one past `5`

### Requirement: Float setting values SHALL round-trip through Format

`Format` and `Beautify` SHALL emit a float setting value exactly as its `Literal`, so that
parse → format is a fixed point for SQL with float setting values.

#### Scenario: Round trip keeps the literal
- **WHEN** `SELECT 1 SETTINGS a = 0.5, b = .5, c = 1.5e-3` is parsed and formatted
- **THEN** the output is `SELECT 1 SETTINGS a=0.5, b=.5, c=1.5e-3` AND formatting the
  re-parsed output yields the same text

### Requirement: Existing settings values SHALL parse unchanged

Adding float support SHALL NOT change the AST or formatted output of any settings value
that parsed before (integer, string, `$$` string, brace map, `TRUE`/`FALSE`, `$variable`).
A value that is none of these and not a number SHALL still be a parse error.

#### Scenario: Existing goldens are byte-identical
- **WHEN** goldens are regenerated after the change
- **THEN** no existing JSON, format or beautify golden file changes

#### Scenario: Integer value is unchanged
- **WHEN** `SELECT 1 SETTINGS x = -1` is parsed
- **THEN** the value is a `*NumberLiteral` with `Literal` `"-1"`

#### Scenario: Non-number value still errors
- **WHEN** `SELECT 1 SETTINGS x = (1)` is parsed
- **THEN** `ParseStmts` returns an error
