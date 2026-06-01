## 1. Baseline

- [x] 1.1 Confirm `parser/parser_test.go` contains `TestParser_With_ExceptIdent` exercising `EXCEPT col`, `EXCEPT (col)`, and `REPLACE(...) EXCEPT colX APPLY(...)`. Starting state: FAIL.
- [x] 1.2 Confirm `parser/testdata/query/select_item_with_modifiers.sql` exists and contains the line `SELECT * REPLACE(i + 1 AS i) EXCEPT (j) APPLY(sum) from t2;`. This fixture is a regression guard for the existing parens form.
- [x] 1.3 Capture the baseline: `go test ./parser/... -run 'TestParser_With_ExceptIdent|TestParser_InvalidSyntax|TestParser_ParseStatements|TestParser_Format|TestParser_FormatBeautify' -count=1`. `TestParser_With_ExceptIdent` must FAIL; everything else must PASS.

## 2. Implementation in `parser/parser_column.go`

- [x] 2.1 Add a new function `func (p *Parser) parseExceptExpr(_ Pos) (*FunctionExpr, error)` near `parseFunctionExpr` (around line 631). The function MUST:
  - Verify the current token is `KeywordExcept` (defensive: it is the caller's responsibility to dispatch only when this is true; if not, return `fmt.Errorf("expected EXCEPT clause but got %s", p.lastTokenKind())`).
  - Consume the EXCEPT keyword via `p.parseIdent()` into `name`.
  - If `p.matchTokenKind(TokenKindIdent)` is true: parse the single identifier via `p.parseIdent()` into `param` and return `&FunctionExpr{Name: name, Params: &ParamExprList{Items: &ColumnExprList{Items: []Expr{param}}}}`.
  - Otherwise: call `p.parseFunctionParams(p.Pos())` and return `&FunctionExpr{Name: name, Params: params}` (the existing parens-form behaviour).
- [x] 2.2 Modify `parseSelectItem` (around line 819). Split the combined modifier dispatch:
  ```go
  for {
      if p.matchKeyword(KeywordExcept) {
          modifier, err := p.parseExceptExpr(p.Pos())
          if err != nil {
              return nil, err
          }
          modifiers = append(modifiers, modifier)
      } else if p.matchKeyword(KeywordApply) || p.matchKeyword(KeywordReplace) {
          modifier, err := p.parseFunctionExpr(p.Pos())
          if err != nil {
              return nil, err
          }
          modifiers = append(modifiers, modifier)
      } else {
          break
      }
  }
  ```
- [x] 2.3 `go build ./parser/...` to confirm the package compiles.

## 3. Add `.sql` fixture inputs

- [x] 3.1 Create `parser/testdata/query/select_except_bare_ident.sql` with the single line:
  ```
  SELECT * EXCEPT col FROM t
  ```
- [x] 3.2 Create `parser/testdata/query/select_except_mixed_modifiers.sql` with the single line:
  ```
  SELECT * REPLACE(i + 1 AS i) EXCEPT colX APPLY(sum) FROM t
  ```

## 4. Generate and inspect the goldens

- [x] 4.1 Run `go test ./parser/... -run 'TestParser_ParseStatements/(select_except_bare_ident\.sql|select_except_mixed_modifiers\.sql)$' -count=1 -update`. This creates the two JSON golden files under `parser/testdata/query/output/`. **Visually inspect each generated JSON** to confirm the AST has the expected shape — `FunctionExpr` with `Name: "EXCEPT"` and a single-item `Params.Items.Items`. For the mixed-modifier fixture, also confirm REPLACE and APPLY are still represented correctly alongside.
- [x] 4.2 Run `go test ./parser/... -run 'TestParser_Format/(select_except_bare_ident\.sql|select_except_mixed_modifiers\.sql)$' -count=1 -update`. This creates `parser/testdata/query/format/select_except_bare_ident.sql` and `select_except_mixed_modifiers.sql`. **Visually inspect each generated `.sql`** to confirm the formatter renders `EXCEPT (col)` (the canonical parens form — bare-ident input round-trips through the canonical output per design Decision 2). Verify whitespace and operator placement.
- [x] 4.3 Run `go test ./parser/... -run 'TestParser_FormatBeautify/(select_except_bare_ident\.sql|select_except_mixed_modifiers\.sql)$' -count=1 -update`. This creates the `format/beautify/` counterparts. **Visually inspect** them too.
- [x] 4.4 Re-run all three commands without `-update`: `go test ./parser/... -run 'TestParser_ParseStatements|TestParser_Format|TestParser_FormatBeautify' -count=1`. All goldens (new and old) must pass.

## 5. Verify the feature contract

- [x] 5.1 `go test ./parser/... -run 'TestParser_With_ExceptIdent' -v -count=1` → expect PASS for all three SQLs in the test.

## 6. Verify regression guards

- [x] 6.1 `go test ./parser/... -run 'TestParser_InvalidSyntax' -v -count=1` → must PASS.
- [x] 6.2 `go test ./parser/... -run 'TestParser_ParseStatements/select_item_with_modifiers\.sql$' -v -count=1` → must PASS. The existing parens-form golden must continue to match byte-for-byte.
- [x] 6.3 `go test ./parser/... -run 'TestParser_Format/select_item_with_modifiers\.sql$' -v -count=1` → must PASS.
- [x] 6.4 `go test ./parser/... -run 'TestParser_FormatBeautify/select_item_with_modifiers\.sql$' -v -count=1` → must PASS.
- [x] 6.5 Spot-check that the existing fixture's JSON golden has NOT changed: `git diff parser/testdata/query/output/select_item_with_modifiers.sql.golden.json` should be empty.

## 7. Close out

- [x] 7.1 `go test ./parser/... -count=1`. Confirm the deltas from the previous full-suite baseline: `TestParser_With_ExceptIdent` transitions FAIL → PASS; the new golden sub-tests under `TestParser_ParseStatements`, `TestParser_Format`, `TestParser_FormatBeautify` are PASS. Nothing previously passing moves to fail.
- [x] 7.2 `go vet ./parser/...` produces no new warnings (the pre-existing `WriteByte` notice in `parser/format.go` is acceptable).
- [x] 7.3 `openspec validate add-select-except-bare-ident` reports the change as valid.
