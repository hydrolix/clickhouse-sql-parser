# settings-reserved-keyword-names Specification

## ADDED Requirements

### Requirement: A SETTINGS item name SHALL accept reserved keywords

The parser SHALL accept any identifier or keyword, including the reserved keywords `LIMIT`, `OFFSET` and
`FINAL`, as the name of a SETTINGS item, and SHALL keep the name's source spelling in `SettingExpr.Name`.

#### Scenario: Reserved keyword names
- **WHEN** `SELECT 1 SETTINGS limit = 5`, `... SETTINGS offset = 10` and `... SETTINGS final = 1` are parsed
- **THEN** `ParseStmts` returns no error AND the single item is named `limit`, `offset` and `final`
  respectively, with the given number value

#### Scenario: Mixed with ordinary names
- **WHEN** `SELECT 1 SETTINGS max_threads = 4, limit = 5, offset = 10` is parsed
- **THEN** the item names are `max_threads`, `limit`, `offset` in order

#### Scenario: Non-reserved keyword name
- **WHEN** `SELECT 1 SETTINGS date = 1` is parsed
- **THEN** `ParseStmts` returns no error AND the item is named `date`

#### Scenario: Name positions
- **WHEN** `SELECT 1 SETTINGS limit = 5` is parsed
- **THEN** `Name.Pos()` and `Name.End()` cover exactly the `limit` token

### Requirement: Quoted names SHALL keep working

A quoted SETTINGS item name SHALL keep its quote type, whether or not it is a keyword.

#### Scenario: Quoted reserved keyword
- **WHEN** ``SELECT 1 SETTINGS `limit` = 5`` is parsed
- **THEN** the item is named `limit` with `QuoteType` `BackTicks`

### Requirement: Every SETTINGS site SHALL share the rule

SETTINGS after FORMAT, `SET`, engine/DDL settings and `ALTER TABLE ... MODIFY SETTING` SHALL accept the same names.

#### Scenario: SETTINGS after FORMAT
- **WHEN** `SELECT 1 FORMAT JSON SETTINGS limit = 5` is parsed
- **THEN** the item is in `FormatSettings` AND `Settings` is nil

#### Scenario: SET statement
- **WHEN** `SET limit = 5` is parsed
- **THEN** the result is a `*SetStmt` whose item is named `limit`

#### Scenario: ALTER TABLE MODIFY SETTING
- **WHEN** `ALTER TABLE t MODIFY SETTING limit = 5` is parsed
- **THEN** the `*AlterTableModifySetting` item is named `limit`

### Requirement: Non-name tokens SHALL still be rejected

A token that is neither an identifier nor a keyword SHALL NOT be accepted as a SETTINGS item name.

#### Scenario: Missing or non-name token
- **WHEN** `SELECT 1 SETTINGS = 5`, `SELECT 1 SETTINGS 5 = 5`, `SELECT 1 SETTINGS limit` or
  ``SELECT 1 SETTINGS 'limit' = 5`` is parsed
- **THEN** `ParseStmts` returns an error

### Requirement: Output SHALL round-trip with source spelling

`Format` SHALL print a SETTINGS item name with its source spelling.

#### Scenario: Format keeps lowercase names
- **WHEN** `SELECT 1 SETTINGS limit = 5` is formatted
- **THEN** the output contains `limit=5`, not `LIMIT=5`
