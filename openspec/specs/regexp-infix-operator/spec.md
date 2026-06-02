## Purpose

Recognise `REGEXP` and `NOT REGEXP` as infix pattern-matching operators in the ClickHouse SQL parser. ClickHouse SQL supports `REGEXP` (and its negated form `NOT REGEXP`) in the same operator family as `LIKE` and `ILIKE`, but the parser today accepts only `LIKE`, `ILIKE`, `NOT LIKE`, `NOT ILIKE`, `BETWEEN`, `IN`, and `NOT IN`. Queries that use `REGEXP` for pattern filtering — a common shape in observability dashboards and log-analysis workloads — fail to parse at the operator position. This change closes the gap with two small, symmetric edits to the precedence ladder and the operator predicate set, plus a matching extension to the `NOT` switch that already groups the negated pattern-matching operators, without introducing new AST nodes, lexer changes, or formatter changes.

## Requirements

### Requirement: Parser SHALL accept `REGEXP` as an infix operator

When `parseInfix` encounters the keyword `REGEXP` between two operands, the parser SHALL recognise it as a binary pattern-matching operator. The arm SHALL consume the keyword, parse the right-hand side via the existing `parseSubExpr`, and emit a `BinaryOperation` whose `Operation` field is `TokenKind("REGEXP")`. The precedence of this operator SHALL be `PrecedenceBetweenLike` — the same level as `LIKE`, `ILIKE`, and `BETWEEN`.

#### Scenario: Bare REGEXP with a literal pattern
- **WHEN** `SELECT * FROM t WHERE x REGEXP 'foo'` is parsed
- **THEN** `ParseStmts` returns no error and produces a non-empty statement list

#### Scenario: REGEXP with a character-class pattern
- **WHEN** `SELECT * FROM t WHERE x REGEXP '(a|b)'` is parsed
- **THEN** `ParseStmts` returns no error and produces a non-empty statement list

#### Scenario: REGEXP combined with GROUP BY
- **WHEN** `SELECT count() FROM t WHERE name REGEXP 'Bot' GROUP BY name` is parsed
- **THEN** `ParseStmts` returns no error and produces a non-empty statement list

### Requirement: Parser SHALL accept `NOT REGEXP` as an infix operator

When `parseInfix` encounters the keyword sequence `NOT REGEXP`, the parser SHALL recognise it as a binary negated-pattern-matching operator alongside the existing `NOT IN`, `NOT LIKE`, and `NOT ILIKE`. The resulting `BinaryOperation` SHALL carry `Operation: TokenKind("NOT REGEXP")`.

#### Scenario: NOT REGEXP with a literal pattern
- **WHEN** `SELECT * FROM t WHERE x NOT REGEXP 'foo'` is parsed
- **THEN** `ParseStmts` returns no error and produces a non-empty statement list

### Requirement: Diagnostic for unsupported `NOT <keyword>` SHALL list REGEXP

If the parser sees `NOT` followed by a keyword that is none of `IN`, `LIKE`, `ILIKE`, or `REGEXP`, it SHALL return an error whose message contains the substring `expected IN, LIKE, ILIKE or REGEXP after NOT`. The diagnostic SHALL list all four legal continuations.

#### Scenario: NOT followed by an unsupported keyword
- **WHEN** `SELECT * FROM t WHERE x NOT BETWEEN_FOO 1 AND 2` (where `BETWEEN_FOO` is not a recognised continuation of `NOT`) is parsed
- **THEN** `ParseStmts` returns an error whose message contains `expected IN, LIKE, ILIKE or REGEXP after NOT`

### Requirement: REGEXP SHALL be exercised end-to-end through the golden fixture suite

Three `.sql` fixtures SHALL be added under `parser/testdata/query/` covering bare REGEXP in WHERE, REGEXP inside CASE WHEN, and NOT REGEXP. Each fixture SHALL be exercised by all three golden test functions — `TestParser_ParseStatements` (JSON AST), `TestParser_Format` (compact SQL re-rendering), and `TestParser_FormatBeautify` (beautified SQL) — and the corresponding golden files SHALL be committed under `output/`, `format/`, and `format/beautify/` respectively.

#### Scenario: Bare REGEXP fixture flows through all three goldens
- **WHEN** `parser/testdata/query/select_regexp.sql` containing `SELECT a, b FROM t WHERE name REGEXP '^foo'` is added
- **THEN** the corresponding golden files at `parser/testdata/query/output/select_regexp.sql.golden.json`, `parser/testdata/query/format/select_regexp.sql`, and `parser/testdata/query/format/beautify/select_regexp.sql` exist and match without `-update`

#### Scenario: REGEXP in CASE WHEN fixture flows through all three goldens
- **WHEN** `parser/testdata/query/select_case_when_regexp.sql` containing `SELECT CASE WHEN col REGEXP '^[0-9]+$' THEN toInt32(col) ELSE 0 END AS num_value FROM t` is added
- **THEN** the corresponding three golden files exist and match without `-update`

#### Scenario: NOT REGEXP fixture flows through all three goldens
- **WHEN** `parser/testdata/query/select_not_regexp.sql` containing `SELECT a, b FROM t WHERE name NOT REGEXP '^foo'` is added
- **THEN** the corresponding three golden files exist and match without `-update`

### Requirement: Existing parser, AST, and golden behaviour SHALL be preserved

This change SHALL NOT alter the lexer, SHALL NOT introduce or rename any AST node or field, SHALL NOT change the precedence of any operator other than `REGEXP`, and SHALL NOT cause any existing golden-file fixture under `parser/testdata/` to drift.

#### Scenario: Existing pattern operators unchanged
- **WHEN** any existing test exercising `LIKE`, `ILIKE`, `BETWEEN`, `IN`, `NOT IN`, `NOT LIKE`, or `NOT ILIKE` is run after this change
- **THEN** the result matches today's behaviour exactly

#### Scenario: Pre-existing goldens remain green
- **WHEN** `TestParser_ParseStatements`, `TestParser_Format`, and `TestParser_FormatBeautify` are run after this change
- **THEN** every golden file that existed before this change matches without `-update`
