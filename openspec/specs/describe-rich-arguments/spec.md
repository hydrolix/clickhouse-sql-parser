## Purpose

Broaden the argument of a `DESCRIBE`/`DESC` statement so that the parser accepts any single table expression that the FROM clause accepts — a table identifier (with optional database), a table function call (`numbers(10)`, `remote(...)`), a parenthesised subquery (`(SELECT col, expr FROM inner_table)`), and any of these with an optional `FINAL` keyword or alias — matching ClickHouse's server-side `ParserDescribeTableQuery` (which delegates to `ParserTableExpression`). Today the parser accepts only `DESCRIBE [TABLE] foo` plus the recently-landed optional `SETTINGS` clause; anything else is rejected with `"expected <ident> or <string>"` or `"<EOF> or ';' was expected"`. The change is implemented by routing `parseDescribeStmt` through the existing `parseTableExpr` helper (the same helper that already produces `SelectQuery.From.Expr` for a one-table FROM), promoting `DescribeStmt.Target` from `*TableIdentifier` to `*TableExpr`, and reusing the existing `parseSubQuery` / `parseTableFunctionExpr` / `parseTableIdentifier` pipeline — no new AST node, no new visitor method, no new keyword, no lexer change. JOIN expressions remain an explicit non-goal so the parser stays aligned with the server's grammar (which itself rejects `DESCRIBE foo JOIN bar ON ...`); users who want to describe a join result must wrap it in a subquery.

## Requirements

### Requirement: Parser SHALL accept a subquery as the argument of DESCRIBE

`parseDescribeStmt` SHALL accept a parenthesised SELECT statement as the argument of a `DESCRIBE`/`DESC` statement. The argument SHALL be parsed via the same grammar used by the FROM clause of a SELECT, so any subquery shape that FROM accepts is accepted here too.

#### Scenario: Bare subquery
- **WHEN** `DESCRIBE (SELECT 1 AS x, 2 AS y)` is parsed
- **THEN** `ParseStmts` returns no error and the resulting `*DescribeStmt`'s `Target` field is a non-nil `Expr` whose concrete type wraps a `*SubQuery`

#### Scenario: Subquery with alias
- **WHEN** `DESCRIBE (SELECT a, b FROM inner_table) AS subq` is parsed
- **THEN** `ParseStmts` returns no error

### Requirement: Parser SHALL accept an aliased table as the argument of DESCRIBE

`parseDescribeStmt` SHALL accept `<table> [AS] <alias>` as the argument of `DESCRIBE`. Both `DESCRIBE foo AS f` and the keyword-less `DESCRIBE foo f` forms SHALL parse, matching the alias grammar accepted by FROM clauses.

#### Scenario: Table with AS alias
- **WHEN** `DESCRIBE foo AS f` is parsed
- **THEN** `ParseStmts` returns no error

#### Scenario: Dotted table with AS alias
- **WHEN** `DESCRIBE db.foo AS f` is parsed
- **THEN** `ParseStmts` returns no error

### Requirement: Parser SHALL accept a table function call as the argument of DESCRIBE

`parseDescribeStmt` SHALL accept a table function call as the argument of `DESCRIBE`. Table functions are the standard ClickHouse mechanism for parameterised table-like sources (`numbers(10)`, `remote(...)`, `cluster(...)`, etc.).

#### Scenario: numbers() table function
- **WHEN** `DESCRIBE numbers(10)` is parsed
- **THEN** `ParseStmts` returns no error AND the resulting `*DescribeStmt.Target.Expr` is a `*TableFunctionExpr`

#### Scenario: Table function with multiple arguments
- **WHEN** `DESCRIBE remote('host', db.foo)` is parsed
- **THEN** `ParseStmts` returns no error

### Requirement: Parser SHALL accept the FINAL keyword after the DESCRIBE target

`parseDescribeStmt` SHALL accept `FINAL` after the target, matching the same `FINAL` placement supported in FROM clauses. The resulting `*TableExpr`'s `HasFinal` field SHALL be `true`.

#### Scenario: Table with FINAL
- **WHEN** `DESCRIBE foo FINAL` is parsed
- **THEN** `ParseStmts` returns no error AND `(*DescribeStmt.Target).HasFinal == true`

### Requirement: Parser SHALL REJECT join expressions as the argument of DESCRIBE

To match ClickHouse's server-side grammar (which uses `ParserTableExpression`, not a join parser, in `ParserDescribeTableQuery`), the parser SHALL reject `DESCRIBE <a> JOIN <b> ON ...`. Users who want to describe the result of a join must wrap it in a subquery: `DESCRIBE (SELECT * FROM a JOIN b ON ...)`.

#### Scenario: Direct JOIN argument rejected
- **WHEN** `DESCRIBE foo JOIN bar ON foo.x = bar.x` is parsed
- **THEN** `ParseStmts` returns an error

#### Scenario: Join wrapped in subquery accepted
- **WHEN** `DESCRIBE (SELECT * FROM foo JOIN bar ON foo.x = bar.x)` is parsed
- **THEN** `ParseStmts` returns no error

### Requirement: `DescribeStmt.Target` field SHALL be `*TableExpr` (concrete type)

The `Target` field on `*DescribeStmt` SHALL have type `*TableExpr`. This is the exact return type of `parseTableExpr` and the same type already used as `SelectQuery.From.Expr` for a one-table FROM. The inner content (table identifier, subquery, or table function) sits behind `Target.Expr` and is accessed via a single type assertion against the appropriate concrete type.

Consumers SHALL read inner content via `stmt.Target.Expr.(*TableIdentifier)`, `stmt.Target.Expr.(*SubQuery)`, or `stmt.Target.Expr.(*TableFunctionExpr)` — the same one-level unwrap pattern used everywhere else in the AST that reads inside a `*TableExpr`.

#### Scenario: Simple table target wraps in TableExpr
- **WHEN** `DESCRIBE foo` is parsed
- **THEN** the resulting `*DescribeStmt.Target` is a `*TableExpr` whose `Expr` field is a `*TableIdentifier` with `Table.Name == "foo"`

#### Scenario: SubQuery target wraps in TableExpr
- **WHEN** `DESCRIBE (SELECT 1)` is parsed
- **THEN** the resulting `*DescribeStmt.Target` is a `*TableExpr` whose `Expr` field is a `*SubQuery`

#### Scenario: Table function target wraps in TableExpr
- **WHEN** `DESCRIBE numbers(10)` is parsed
- **THEN** the resulting `*DescribeStmt.Target` is a `*TableExpr` whose `Expr` field is a `*TableFunctionExpr`

### Requirement: SETTINGS clause SHALL continue to parse after rich-argument DESCRIBE

The optional trailing `SETTINGS k=v, ...` clause introduced for `DESCRIBE` SHALL continue to parse after a rich (subquery, aliased, joined) argument. The grammar order is `DESCRIBE <argument> [SETTINGS <list>]`.

#### Scenario: Bare table with SETTINGS
- **WHEN** `DESCRIBE foo SETTINGS describe_compact_output=1` is parsed
- **THEN** `ParseStmts` returns no error AND `Settings` is non-nil

#### Scenario: Subquery with SETTINGS
- **WHEN** `DESCRIBE (SELECT 1) SETTINGS describe_compact_output=1` is parsed
- **THEN** `ParseStmts` returns no error AND `Settings` is non-nil AND `Target` wraps a `*SubQuery`

### Requirement: Pre-existing DESCRIBE goldens SHALL be regenerated with a TableExpr-wrapped Target

The four existing DESCRIBE/DESC JSON goldens (`describe_table_with_table_keyword.sql`, `describe_table_without_table_keyword.sql`, `desc_table_with_table_keyword.sql`, `desc_table_without_table_keyword.sql`) plus the two SETTINGS-bearing JSON goldens (`describe_table_with_settings.sql`, `describe_settings_multiple.sql`) SHALL be regenerated. Each `Target` field SHALL render as a `*TableExpr` object whose `Expr` field contains the previously-bare `*TableIdentifier`.

#### Scenario: Existing JSON golden shifts to TableExpr-wrapped form
- **WHEN** `TestParser_ParseStatements/describe_table_with_table_keyword.sql` runs against the post-change parser
- **THEN** the regenerated golden's `Target` field is a `*TableExpr` whose `Expr` is the same `*TableIdentifier` block (verbatim contents) that previously sat directly at `Target`

#### Scenario: SETTINGS-bearing golden also shifts
- **WHEN** `TestParser_ParseStatements/describe_table_with_settings.sql` runs against the post-change parser
- **THEN** the regenerated golden's `Target` field is a `*TableExpr` wrapping the inner `*TableIdentifier` AND `Settings` remains populated with the parsed `*SettingsClause`

### Requirement: Existing parser, AST, and unrelated golden behaviour SHALL be preserved

This change SHALL NOT alter the lexer, SHALL NOT introduce or rename any AST node, SHALL NOT introduce or rename any visitor method, SHALL NOT modify `parseJoinExpr` or `parseTableExpr`, and SHALL NOT cause any golden-file fixture outside the DESCRIBE family to drift.

#### Scenario: Non-DESCRIBE goldens unchanged
- **WHEN** the full golden suite is run after this change
- **THEN** every golden file outside `parser/testdata/ddl/{,output/,format/,format/beautify/}desc*` matches byte-for-byte without `-update`

#### Scenario: TestParser_InvalidSyntax unchanged
- **WHEN** `TestParser_InvalidSyntax` is run after this change
- **THEN** the test passes with the same set of error inputs that pass today

#### Scenario: Visitor traversal still reaches the same nodes
- **WHEN** a visitor traverses a `DescribeStmt` whose target is a bare table identifier
- **THEN** the visitor's `VisitTableIdentifier` is called at least once during the traversal (delegated through `*TableExpr.Accept`), and the visitor's `VisitDescribeExpr` is called at least once
