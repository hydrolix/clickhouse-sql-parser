# select-except-bare-ident Specification (delta)

## MODIFIED Requirements

### Requirement: Parser SHALL accept `EXCEPT col` as a select-item modifier

The parser SHALL consume `EXCEPT` and a single identifier as one modifier when the
modifier loop in `parseSelectItem` meets `KeywordExcept` after a star-like select item
(`*` or `COLUMNS(...)`), producing a `FunctionExpr` with `Name = "EXCEPT"` and a
one-element `Params` carrying the identifier. The identifier MAY be a token the lexer
classifies as a keyword (for example `timestamp`, `date`, `key`). After a non-star item,
`EXCEPT` SHALL NOT be consumed as a modifier (see `select-star-except`).

#### Scenario: Bare-ident EXCEPT alone
- **WHEN** `SELECT * EXCEPT col FROM t` is parsed
- **THEN** `ParseStmts` returns no error and produces a non-empty statement list

#### Scenario: Bare-ident EXCEPT combined with other modifiers
- **WHEN** `SELECT * REPLACE(i + 1 AS i) EXCEPT colX APPLY(sum) FROM t` is parsed
- **THEN** `ParseStmts` returns no error and produces a non-empty statement list

#### Scenario: Bare keyword-named column
- **WHEN** `SELECT * EXCEPT timestamp FROM t` is parsed
- **THEN** `ParseStmts` returns no error AND the modifier has the single parameter
  `timestamp`

### Requirement: Existing parser, AST, and golden behaviour SHALL be preserved

This change SHALL NOT alter the lexer, SHALL NOT introduce or rename any AST node or field,
SHALL NOT modify `parseFunctionExpr`, SHALL NOT alter the behaviour of `APPLY` or `REPLACE`
modifiers, and SHALL NOT cause any pre-existing golden-file fixture under `parser/testdata/`
to drift. `EXCEPT` after a non-star select item is a set operator, not a modifier.

#### Scenario: APPLY and REPLACE modifiers unchanged
- **WHEN** any existing test or fixture exercising `APPLY(...)` or `REPLACE(...)` as a
  select-item modifier is run after this change
- **THEN** the result matches today's behaviour exactly

#### Scenario: Pre-existing goldens remain green
- **WHEN** `TestParser_ParseStatements`, `TestParser_Format`, and `TestParser_FormatBeautify`
  are run after this change
- **THEN** every golden file that existed before this change matches without `-update`
