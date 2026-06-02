## MODIFIED Requirements

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

## ADDED Requirements

### Requirement: Backtick-quoted identifiers SHALL NOT be interpreted as Grafana variables

The parser SHALL treat any backtick-quoted identifier (`Token.QuoteType == BackTicks`) as an ordinary identifier even if its body begins with `$`. In particular, such tokens SHALL NOT be assigned `PrecedenceIdent` by `getNextPrecedence`, SHALL NOT be consumed as a binary operator by `parseInfix`, and SHALL NOT be accepted as a variable-shaped value by `parseSettingsExpr`.

#### Scenario: Backtick-quoted `$`-prefixed column in a select expression
- **WHEN** ``SELECT `$col` FROM t`` is parsed
- **THEN** `ParseStmts` returns no error and the resulting expression is an `Ident` whose `Name == "$col"` and `QuoteType == BackTicks`, NOT a `BinaryOperation`

#### Scenario: Backtick-quoted `$`-prefixed identifier in a WHERE comparison
- **WHEN** ``SELECT 1 FROM t WHERE `$col` = 1`` is parsed
- **THEN** `ParseStmts` returns no error and the WHERE predicate is a comparison between `` `$col` `` and `1`, NOT a binary expression that consumes `` `$col` `` as the operator
