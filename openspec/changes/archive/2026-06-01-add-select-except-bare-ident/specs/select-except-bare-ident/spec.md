## ADDED Requirements

### Requirement: Parser SHALL accept `EXCEPT col` as a select-item modifier

When the modifier loop in `parseSelectItem` encounters `KeywordExcept` followed by an identifier token (not `(`), the parser SHALL consume the EXCEPT keyword and the single identifier as one modifier, producing a `FunctionExpr` with `Name = "EXCEPT"` and a one-element `Params` carrying the identifier.

#### Scenario: Bare-ident EXCEPT alone
- **WHEN** `SELECT * EXCEPT col FROM t` is parsed
- **THEN** `ParseStmts` returns no error and produces a non-empty statement list

#### Scenario: Bare-ident EXCEPT combined with other modifiers
- **WHEN** `SELECT * REPLACE(i + 1 AS i) EXCEPT colX APPLY(sum) FROM t` is parsed
- **THEN** `ParseStmts` returns no error and produces a non-empty statement list

### Requirement: Bare-ident and parens forms SHALL produce structurally identical AST

The `FunctionExpr` produced by `EXCEPT col` SHALL have the same shape (`Name`, `Params *ParamExprList`, `Params.Items *ColumnExprList`, `Params.Items.Items []Expr`) as the `FunctionExpr` produced by `EXCEPT (col)`. Downstream consumers (formatter, visitor, JSON marshaller) SHALL NOT need to distinguish the two surface syntaxes — the AST is the single source of truth.

#### Scenario: Single-column EXCEPT round-trips through the formatter consistently
- **WHEN** `SELECT * EXCEPT col FROM t` is parsed and then re-formatted with `Format(stmt)`
- **THEN** the formatted output is `SELECT * EXCEPT (col) FROM t` (the canonical parens form, with the single column preserved)

### Requirement: Existing parens form SHALL be preserved

`SELECT … EXCEPT (col1, col2, …) …` SHALL continue to parse exactly as it does today, producing the same `FunctionExpr` shape with the column list inside `Params`. The existing golden fixture `parser/testdata/query/select_item_with_modifiers.sql` SHALL continue to match byte-for-byte without `-update`.

#### Scenario: Multi-column parens form still parses
- **WHEN** `SELECT * EXCEPT (a, b, c) FROM t` is parsed
- **THEN** `ParseStmts` returns no error and produces a non-empty statement list whose AST matches the shape produced before this change

#### Scenario: Existing select-item-with-modifiers golden remains green
- **WHEN** `TestParser_ParseStatements`, `TestParser_Format`, and `TestParser_FormatBeautify` are run after this change against `parser/testdata/query/select_item_with_modifiers.sql`
- **THEN** every existing golden file matches without `-update`

### Requirement: EXCEPT SHALL be exercised end-to-end through the golden fixture suite

Two `.sql` fixtures SHALL be added under `parser/testdata/query/` covering the bare-ident form alone and the bare-ident form combined with REPLACE and APPLY modifiers. Each fixture SHALL be exercised by all three golden test functions — `TestParser_ParseStatements` (JSON AST), `TestParser_Format` (compact SQL re-rendering), and `TestParser_FormatBeautify` (beautified SQL) — and the corresponding golden files SHALL be committed under `output/`, `format/`, and `format/beautify/` respectively.

#### Scenario: Bare-ident fixture flows through all three goldens
- **WHEN** `parser/testdata/query/select_except_bare_ident.sql` containing `SELECT * EXCEPT col FROM t` is added
- **THEN** the corresponding golden files at `parser/testdata/query/output/select_except_bare_ident.sql.golden.json`, `parser/testdata/query/format/select_except_bare_ident.sql`, and `parser/testdata/query/format/beautify/select_except_bare_ident.sql` exist and match without `-update`

#### Scenario: Mixed-modifiers fixture flows through all three goldens
- **WHEN** `parser/testdata/query/select_except_mixed_modifiers.sql` containing `SELECT * REPLACE(i + 1 AS i) EXCEPT colX APPLY(sum) FROM t` is added
- **THEN** the corresponding three golden files exist and match without `-update`

### Requirement: Existing parser, AST, and golden behaviour SHALL be preserved

This change SHALL NOT alter the lexer, SHALL NOT introduce or rename any AST node or field, SHALL NOT modify `parseFunctionExpr`, SHALL NOT alter the behaviour of `APPLY` or `REPLACE` modifiers, and SHALL NOT cause any pre-existing golden-file fixture under `parser/testdata/` to drift.

#### Scenario: APPLY and REPLACE modifiers unchanged
- **WHEN** any existing test or fixture exercising `APPLY(...)` or `REPLACE(...)` as a select-item modifier is run after this change
- **THEN** the result matches today's behaviour exactly

#### Scenario: Pre-existing goldens remain green
- **WHEN** `TestParser_ParseStatements`, `TestParser_Format`, and `TestParser_FormatBeautify` are run after this change
- **THEN** every golden file that existed before this change matches without `-update`
