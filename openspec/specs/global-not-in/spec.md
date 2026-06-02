## Purpose

Recognise `GLOBAL NOT IN <subquery>` as the negated form of `GLOBAL IN` in the ClickHouse SQL parser. The parser already accepts `GLOBAL IN`, but `GLOBAL NOT IN` is rejected with `"expected IN after GLOBAL"`, while every other negated IN-family operator — bare `NOT IN`, `NOT LIKE`, `NOT ILIKE` — is supported. This change closes the gap with a small extension to the same parser arm that already handles `GLOBAL IN`, without introducing new AST nodes, formatter changes, or lexer changes.

## Requirements

### Requirement: Parser SHALL accept `GLOBAL NOT IN <subquery>` as a binary operator

When `parseInfix` encounters the keyword `GLOBAL`, the parser SHALL accept either `IN` or `NOT IN` as the operator's continuation. When `NOT IN` is used, the resulting `BinaryOperation` SHALL carry `Operation: TokenKind("GLOBAL NOT IN")` (the verbatim three-keyword string). The right-hand operand is parsed by the existing `parseSubExpr` call at the same precedence as `GLOBAL IN`.

#### Scenario: Subquery on the right-hand side
- **WHEN** `SELECT * FROM t WHERE x GLOBAL NOT IN (SELECT y FROM remote('127.0.0.1', s))` is parsed
- **THEN** `ParseStmts` returns no error and produces a non-empty statement list

#### Scenario: Parenthesised value list on the right-hand side
- **WHEN** `SELECT * FROM t WHERE x GLOBAL NOT IN (1, 2, 3)` is parsed
- **THEN** `ParseStmts` returns no error and produces a non-empty statement list

### Requirement: Existing `GLOBAL IN` behaviour SHALL be preserved

`GLOBAL IN <subquery>` SHALL continue to parse exactly as it does today, producing a `BinaryOperation` with `Operation: TokenKind("GLOBAL IN")`. This change adds the negated form alongside, it does NOT modify the existing form.

#### Scenario: Plain GLOBAL IN still parses
- **WHEN** `SELECT * FROM t WHERE x GLOBAL IN (SELECT y FROM remote('127.0.0.1', s))` is parsed
- **THEN** `ParseStmts` returns no error and produces a non-empty statement list

### Requirement: `GLOBAL` followed by any non-IN, non-NOT-IN sequence SHALL still error

If the tokens following `GLOBAL` are neither `IN` nor `NOT IN`, the parser SHALL return an error whose message starts with `"expected IN or NOT IN after GLOBAL"`. The error remains a parse-time diagnostic; no AST node is produced for the malformed clause.

#### Scenario: GLOBAL followed by garbage
- **WHEN** `SELECT * FROM t WHERE x GLOBAL FOO (1, 2)` is parsed
- **THEN** `ParseStmts` returns an error whose message contains the substring `expected IN or NOT IN after GLOBAL`

#### Scenario: GLOBAL NOT followed by garbage
- **WHEN** `SELECT * FROM t WHERE x GLOBAL NOT FOO (1, 2)` is parsed
- **THEN** `ParseStmts` returns an error whose message contains the substring `expected IN or NOT IN after GLOBAL`

### Requirement: Existing parser, AST, and golden behaviour SHALL be preserved

This change SHALL NOT alter the lexer in any way, SHALL NOT introduce or rename any AST node or field, SHALL NOT set the `HasGlobal` or `HasNot` boolean fields on `BinaryOperation`, and SHALL NOT cause any existing golden-file fixture under `parser/testdata/` to drift.

#### Scenario: Golden tests remain green
- **WHEN** `TestParser_ParseStatements`, `TestParser_Format`, and `TestParser_FormatBeautify` are run after this change
- **THEN** every existing golden file matches without `-update`

#### Scenario: Other parser tests remain unaffected
- **WHEN** `TestParser_InvalidSyntax` and every other test in `parser/parser_test.go` is run after this change
- **THEN** every previously-passing test continues to pass; the only newly-passing test is `TestParser_With_GlobalNotIn`
