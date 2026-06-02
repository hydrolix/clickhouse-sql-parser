## Purpose

Recognise Grafana template variables (`${name}`, `${name:format}`, and bare `$ident`) at the lexer and parser level so ClickHouse SQL containing variable placeholders can be parsed without prior substitution. This enables linters, query rewriters, observability layers, and schema-aware tooling on the Grafana data path to inspect, validate, and transform queries before Grafana substitutes the variables at runtime, without requiring any change to the AST shape, formatter output, or existing golden tests for non-variable input.

## Requirements

### Requirement: Lexer SHALL tokenize `${name}` and `${name:format}` as a single identifier

The lexer SHALL treat `$` followed by `{` as the start of a braced template variable. It SHALL consume identifier characters and at most one `:format` suffix up to the matching closing `}`, and produce a single `TokenKindIdent` whose `String` field is the verbatim source text of the variable (braces and format suffix included). If the closing `}` is missing before end-of-input, the lexer SHALL return an error whose message begins with `unclosed variable:`.

#### Scenario: Bare braced variable
- **WHEN** the lexer consumes the input `${tbl}`
- **THEN** the resulting token has kind `TokenKindIdent` and `String == "${tbl}"`

#### Scenario: Variable with format suffix
- **WHEN** the lexer consumes the input `${y:sqlstring}`
- **THEN** the resulting token has kind `TokenKindIdent` and `String == "${y:sqlstring}"`

#### Scenario: Unclosed brace
- **WHEN** the lexer consumes the input `${name`
- **THEN** the lexer returns an error whose message starts with `unclosed variable:`

#### Scenario: Bare `$ident` remains unchanged
- **WHEN** the lexer consumes the input `$col`
- **THEN** the resulting token has kind `TokenKindIdent` and `String == "$col"`

### Requirement: Parser SHALL accept a braced variable as an expression operand

The parser SHALL accept `${name}` (with or without a format suffix) anywhere a column-expression operand is permitted: select-list items, `WHERE` predicate operands, `GROUP BY` / `ORDER BY` keys, and operands of comparison/arithmetic/logical operators. The parsed result SHALL appear in the AST as an ordinary identifier-shaped expression whose textual representation preserves the literal `${…}` text.

#### Scenario: Variable in WHERE comparison
- **WHEN** `SELECT 1 FROM t WHERE x = ${y}` is parsed
- **THEN** `ParseStmts` returns no error and produces a non-empty statement list

#### Scenario: Variable in select list
- **WHEN** `SELECT ${a} FROM t` is parsed
- **THEN** `ParseStmts` returns no error and produces a non-empty statement list

#### Scenario: Variable with format suffix in WHERE
- **WHEN** `SELECT 1 FROM t WHERE x = ${y:sqlstring}` is parsed
- **THEN** `ParseStmts` returns no error and produces a non-empty statement list

### Requirement: Parser SHALL accept a braced variable as a table name in FROM

The parser SHALL accept `${name}` in any position where a table identifier is legal, including the immediate target of `FROM` and within join clauses.

#### Scenario: Variable as table name
- **WHEN** `SELECT 1 FROM ${tbl}` is parsed
- **THEN** `ParseStmts` returns no error and produces a non-empty statement list

#### Scenario: Variable as table name with WHERE
- **WHEN** `SELECT 1 FROM ${tbl} WHERE x = 1` is parsed
- **THEN** `ParseStmts` returns no error and produces a non-empty statement list

### Requirement: Parser SHALL accept a braced variable inside function-call arguments

The parser SHALL accept `${name}` as any positional argument of a function call, including arguments to Grafana macros (`$__timeFilter`, `$__timeInterval`, etc.) and to regular ClickHouse functions like `toStartOfInterval`.

#### Scenario: Variable as sole function argument
- **WHEN** `SELECT foo(${a}) FROM t` is parsed
- **THEN** `ParseStmts` returns no error and produces a non-empty statement list

#### Scenario: Variable mixed with other function arguments
- **WHEN** `SELECT toStartOfInterval(${ts}, INTERVAL 1 hour) FROM t` is parsed
- **THEN** `ParseStmts` returns no error and produces a non-empty statement list

#### Scenario: Variable inside a Grafana macro call
- **WHEN** `SELECT $__timeFilter(${ts}) FROM t` is parsed
- **THEN** `ParseStmts` returns no error and produces a non-empty statement list

### Requirement: Parser SHALL accept a braced variable as an infix operator

The parser SHALL recognise `${VAR}` appearing between two operands as an infix operator, at a precedence that is higher than `PrecedenceUnknown` and lower than `PrecedenceOr`. This groups the surrounding expression as a single binary expression regardless of which real operator Grafana later substitutes at runtime.

#### Scenario: Generic placeholder operator
- **WHEN** `SELECT 1 FROM t WHERE a ${OP} b` is parsed
- **THEN** `ParseStmts` returns no error and produces a non-empty statement list

#### Scenario: Grafana's `${AND_*}` pattern
- **WHEN** `SELECT 1 FROM t WHERE statusCode ${AND_statusCode} (1, 2)` is parsed
- **THEN** `ParseStmts` returns no error and produces a non-empty statement list

### Requirement: Parser SHALL accept a `$`-prefixed identifier as a SETTINGS value

`parseSettingsExprList` SHALL recognise a `$`-prefixed identifier (either bare `$ident` or braced `${name}` / `${name:format}`) as a legal value for a settings entry. The setting key remains restricted to its existing grammar — variables on the key side are NOT permitted by this change.

#### Scenario: Single variable setting
- **WHEN** `SELECT 1 FROM t SETTINGS max_threads=$threads` is parsed
- **THEN** `ParseStmts` returns no error and produces a non-empty statement list

#### Scenario: Multiple variable settings
- **WHEN** `SELECT 1 FROM t SETTINGS max_threads=${threads}, max_memory_usage=${mem}` is parsed
- **THEN** `ParseStmts` returns no error and produces a non-empty statement list

### Requirement: A `matchVariable()` helper SHALL centralise the `$`-prefixed-ident test

The parser SHALL expose an internal helper `matchVariable() bool` on `*Parser` that returns `true` exactly when the current token is `TokenKindIdent`, its `QuoteType` is NOT `BackTicks`, and its `String` field begins with `$`. All parser branches that test for "the current token is a Grafana variable" SHALL use this helper rather than re-implementing the check.

#### Scenario: Helper returns true for braced variable
- **WHEN** the parser's current token is an unquoted ident with `String == "${tbl}"`
- **THEN** `p.matchVariable()` returns `true`

#### Scenario: Helper returns true for bare `$ident`
- **WHEN** the parser's current token is an unquoted ident with `String == "$col"`
- **THEN** `p.matchVariable()` returns `true`

#### Scenario: Helper returns false for regular identifier
- **WHEN** the parser's current token is an ident with `String == "col"`
- **THEN** `p.matchVariable()` returns `false`

#### Scenario: Helper returns false for backtick-quoted `$`-ident
- **WHEN** the parser's current token is an ident with `String == "$col"` and `QuoteType == BackTicks`
- **THEN** `p.matchVariable()` returns `false`

### Requirement: Existing parser behaviour SHALL be preserved

This change SHALL NOT alter the AST shape of any non-variable input, SHALL NOT change how the formatter renders any non-variable expression, and SHALL NOT remove or rename any exported symbol. In particular: the special form `EXTRACT(unit FROM expr)` in `parseColumnExpr` SHALL continue to parse, and all existing golden-file fixtures under `parser/testdata/` SHALL continue to round-trip unchanged.

#### Scenario: EXTRACT special form still parses
- **WHEN** `SELECT EXTRACT(HOUR FROM ts) FROM t` is parsed
- **THEN** `ParseStmts` returns no error

#### Scenario: Existing golden tests stay green
- **WHEN** `TestParser_ParseStatements`, `TestParser_Format`, and `TestParser_FormatBeautify` are run after the change is applied
- **THEN** every existing golden file matches without `-update`

### Requirement: Backtick-quoted identifiers SHALL NOT be interpreted as Grafana variables

The parser SHALL treat any backtick-quoted identifier (`Token.QuoteType == BackTicks`) as an ordinary identifier even if its body begins with `$`. In particular, such tokens SHALL NOT be assigned `PrecedenceIdent` by `getNextPrecedence`, SHALL NOT be consumed as a binary operator by `parseInfix`, and SHALL NOT be accepted as a variable-shaped value by `parseSettingsExpr`.

#### Scenario: Backtick-quoted `$`-prefixed column in a select expression
- **WHEN** ``SELECT `$col` FROM t`` is parsed
- **THEN** `ParseStmts` returns no error and the resulting expression is an `Ident` whose `Name == "$col"` and `QuoteType == BackTicks`, NOT a `BinaryOperation`

#### Scenario: Backtick-quoted `$`-prefixed identifier in a WHERE comparison
- **WHEN** ``SELECT 1 FROM t WHERE `$col` = 1`` is parsed
- **THEN** `ParseStmts` returns no error and the WHERE predicate is a comparison between `` `$col` `` and `1`, NOT a binary expression that consumes `` `$col` `` as the operator
