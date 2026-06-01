## Why

ClickHouse SQL accepts two shapes for the `EXCEPT` modifier on a SELECT item: the parenthesised list `SELECT * EXCEPT (col1, col2) FROM t` and the bare-identifier shorthand `SELECT * EXCEPT col FROM t` when there is exactly one column to exclude. The bare-ident shorthand is common in dashboard- and tool-generated SQL because it reads naturally and matches how `EXCEPT` is documented for the single-column case.

Today the parser rejects the bare-ident form with `"expected the last token kind is: (, but got <ident>"` — the `EXCEPT`/`APPLY`/`REPLACE` arm of `parseSelectItem` unconditionally dispatches to `parseFunctionExpr`, which immediately requires `(`. This change adds the shorthand as a peer of the parenthesised form.

## What Changes

- A new helper `parseExceptExpr` in `parser/parser_column.go` recognises both `EXCEPT col` (bare-ident shorthand) and `EXCEPT (col1, col2, …)` (existing parens form). In the bare-ident case it wraps the single identifier in the same `ParamExprList{ColumnExprList{...}}` shape that `parseFunctionParams` produces, so downstream code does not need to distinguish the two forms.
- `parseSelectItem`'s modifier loop splits the existing combined dispatch: `KeywordExcept` now routes to `parseExceptExpr`; `KeywordApply` and `KeywordReplace` continue to use `parseFunctionExpr` exactly as today.
- No new AST node, no new token kind, no lexer change, no formatter change. Both forms produce a `FunctionExpr` with `Name: "EXCEPT"` and a `Params: *ParamExprList` carrying the column list.

## Capabilities

### New Capabilities
- `select-except-bare-ident`: Recognise `SELECT … EXCEPT col …` as shorthand for the single-column case of `SELECT … EXCEPT (col) …`, producing the same AST shape as the parenthesised form with one item in the parameter list.

### Modified Capabilities
<!-- None. -->

## Impact

- **Code touched**: two edits in `parser/parser_column.go`. One new function `parseExceptExpr` is added; one branch in the modifier loop of `parseSelectItem` is split.
- **Behavioural contract — one existing inline test** in `parser/parser_test.go`:
  - **`TestParser_With_ExceptIdent`** — three SQLs: bare-ident `EXCEPT col`, the existing parens form `EXCEPT (col)` (as a smoke test), and a mixed-modifier case `REPLACE(i + 1 AS i) EXCEPT colX APPLY(sum)`. Currently FAILs; flips to PASS after this change.
- **Behavioural contract — two new `.sql` fixtures** under `parser/testdata/query/`, exercising parse + format + beautify through `TestParser_ParseStatements`, `TestParser_Format`, `TestParser_FormatBeautify`. Each fixture produces three golden files (one `.sql.golden.json` for the JSON AST, one `.sql` for the formatted output, one `.sql` for the beautified output) for a total of 2 inputs + 6 goldens:
  - `select_except_bare_ident.sql` — `SELECT * EXCEPT col FROM t`.
  - `select_except_mixed_modifiers.sql` — `SELECT * REPLACE(i + 1 AS i) EXCEPT colX APPLY(sum) FROM t`.
- **Regression guard — the existing parens-form fixture stays unchanged.** `parser/testdata/query/select_item_with_modifiers.sql` already exercises `SELECT * REPLACE(i + 1 AS i) EXCEPT (j) APPLY(sum) from t2;`. Its golden output at `parser/testdata/query/output/select_item_with_modifiers.sql.golden.json` MUST continue to match byte-for-byte without `-update`. The bare-ident path is a NEW dispatch arm; the existing parens path must produce identical output.
- **No dependencies** added, no public API change, no breaking changes.
