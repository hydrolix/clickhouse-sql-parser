# with-prefixed-subquery-expr Specification

## ADDED Requirements

### Requirement: A parenthesised WITH-prefixed query SHALL parse as a subquery in expressions

The parser SHALL parse `( WITH … SELECT … )` in expression position as a `*SubQuery` with
`HasParen` true, exactly as it parses `( SELECT … )`, in the select list, as the operand
of `IN`, and as the value of a top-level `WITH … AS name` item.

#### Scenario: Scalar subquery with a constant alias
- **WHEN** `SELECT (WITH 1000 AS latest SELECT max(x) FROM t WHERE y > latest) AS m` is parsed
- **THEN** `ParseStmts` returns no error AND the select item's `Expr` is a `*SubQuery`
  with `HasParen` true whose `Select.With` is not nil AND the item alias is `m`

#### Scenario: Scalar subquery with a CTE
- **WHEN** `SELECT (WITH x AS (SELECT 1 AS a) SELECT a FROM x)` is parsed
- **THEN** `ParseStmts` returns no error AND the select item's `Expr` is a `*SubQuery`

#### Scenario: WITH-prefixed subquery as IN operand
- **WHEN** `SELECT 1 IN (WITH 1 AS a SELECT a)` is parsed
- **THEN** `ParseStmts` returns no error

#### Scenario: WITH-prefixed subquery as a WITH item
- **WHEN** `WITH (WITH 1 AS a SELECT a) AS b SELECT b` is parsed
- **THEN** `ParseStmts` returns no error AND the outer query's `With` has one item

### Requirement: A bare WITH-prefixed query SHALL parse as a select query in expressions

The parser SHALL parse a bare `WITH … SELECT …` in expression position as a
`*SelectQuery`, exactly as it parses a bare `SELECT …` there.

#### Scenario: WITH-prefixed subquery as a function argument
- **WHEN** `SELECT exists(WITH 1 AS a SELECT a)` is parsed
- **THEN** `ParseStmts` returns no error AND the function's single argument is a
  `*SelectQuery` whose `With` is not nil

### Requirement: SELECT-prefixed subqueries SHALL parse unchanged

Adding the `WITH` arms SHALL NOT change the node kinds or positions produced for
SELECT-prefixed subqueries.

#### Scenario: SELECT-prefixed subqueries unchanged
- **WHEN** `SELECT (SELECT 1)`, `SELECT 1 IN (SELECT 1)` and `SELECT exists(SELECT 1)` are parsed
- **THEN** each returns no error AND the subquery node kinds are those produced before
  this change (`*SubQuery` with parens, `*SelectQuery` bare)

#### Scenario: Round trip is a fixed point
- **WHEN** `SELECT (WITH 1000 AS latest SELECT max(x) FROM t WHERE y > latest) AS m` is
  parsed and formatted
- **THEN** formatting the re-parsed output yields the same text

### Requirement: Existing goldens SHALL be byte-identical

This change SHALL NOT alter the lexer, SHALL NOT add or rename an AST node or field, and
SHALL NOT change `parseSelectQuery`, `parseSubQuery`, CTE parsing or the table path. Every
golden file that existed before the change SHALL match without `-update`.

#### Scenario: Pre-existing goldens remain green
- **WHEN** goldens are regenerated after the change
- **THEN** no existing JSON, format or beautify golden file changes
