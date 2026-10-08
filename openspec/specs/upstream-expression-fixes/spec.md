## Purpose

Carry two upstream `AfterShip/clickhouse-sql-parser` fixes into the fork ahead of a full upstream sync: operator expressions inside table-function arguments (`0d9ef25`, #298), as in `numbers(toUInt32(dateDiff('hour', a, b)) + 1)`, and a `+`/`-` directly after `)` or `]` lexing as a binary operator rather than the sign of a numeric literal (`dd867a1`, #286/#287), as in `(1)-1` and `arr[1]-1`.

## Requirements

### Requirement: Operator expressions SHALL parse inside table-function arguments

The parser SHALL accept arithmetic and other binary-operator expressions as
table-function arguments, including when nested inside function calls, as
in upstream `AfterShip/clickhouse-sql-parser` commit `0d9ef25` (#298).

#### Scenario: Nested call with an operator in numbers()
- **WHEN** `SELECT number FROM numbers(toUInt32(dateDiff('hour', toDateTime(1), toDateTime(7200))) + 1)`
  is parsed
- **THEN** `ParseStmts` returns no error

#### Scenario: Plain operator argument
- **WHEN** `SELECT * FROM numbers(10 + 1)` is parsed
- **THEN** `ParseStmts` returns no error

### Requirement: A sign after a closing bracket SHALL lex as a binary operator

The lexer SHALL treat `+` or `-` directly after `)` or `]` as a binary
operator, not as the sign of a numeric literal, as in upstream commit
`dd867a1` (#286/#287).

#### Scenario: Subtraction after a parenthesis
- **WHEN** `SELECT (1)-1` is parsed
- **THEN** `ParseStmts` returns no error AND the select item is a binary
  `-` operation

#### Scenario: Subtraction after an index
- **WHEN** `SELECT arr[1]-1 FROM t` is parsed
- **THEN** `ParseStmts` returns no error
