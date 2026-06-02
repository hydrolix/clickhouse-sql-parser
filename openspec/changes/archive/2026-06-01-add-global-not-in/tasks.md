## 1. Baseline

- [x] 1.1 Confirm `parser/parser_test.go` contains `TestParser_With_GlobalNotIn` exercising both `GLOBAL NOT IN` and `GLOBAL IN` against a `remote()` subquery. Starting state: FAIL (the `GLOBAL NOT IN` SQL trips the existing `"expected IN after GLOBAL"` error).
- [x] 1.2 Confirm `parser/parser_test.go` contains `TestParser_InvalidSyntax`. Starting state: PASS (regression guard).
- [x] 1.3 Capture the baseline: `go test ./parser/... -run 'TestParser_With_GlobalNotIn|TestParser_InvalidSyntax' -v -count=1`. `TestParser_With_GlobalNotIn` must FAIL; `TestParser_InvalidSyntax` must PASS. If the baseline does not match, stop and reconcile.

## 2. Implementation

- [x] 2.1 In `parser/parser_column.go`, locate the `case p.matchKeyword(KeywordGlobal):` arm inside `func (p *Parser) parseInfix(...)` (currently around line 139).
- [x] 2.2 Replace the arm body with the shape below — see design.md Decision 1 for rationale:

  ```go
  case p.matchKeyword(KeywordGlobal):
      _ = p.lexer.consumeToken()
      op := "GLOBAL IN"
      if p.matchKeyword(KeywordNot) {
          _ = p.lexer.consumeToken()
          op = "GLOBAL NOT IN"
      }
      if p.expectKeyword(KeywordIn) != nil {
          return nil, fmt.Errorf("expected IN or NOT IN after GLOBAL, got %s", p.lastTokenKind())
      }
      rightExpr, err := p.parseSubExpr(p.Pos(), precedence)
      if err != nil {
          return nil, err
      }
      return &BinaryOperation{
          LeftExpr:  expr,
          Operation: TokenKind(op),
          RightExpr: rightExpr,
      }, nil
  ```

- [x] 2.3 Confirm the `BinaryOperation` literal does NOT set `HasGlobal` or `HasNot`. The packed `Operation` string carries the full semantics — see design.md Decision 1.
- [x] 2.4 `go build ./parser/...` to confirm the package compiles.

## 3. Verify the feature contract

- [x] 3.1 `go test ./parser/... -run 'TestParser_With_GlobalNotIn' -v -count=1` → expect PASS. Both SQLs in the test (`GLOBAL NOT IN …` and `GLOBAL IN …`) should now parse without error.

## 4. Verify regression guards

- [x] 4.1 `go test ./parser/... -run 'TestParser_InvalidSyntax' -v -count=1` → must PASS. The existing entries do not exercise `GLOBAL` directly; they remain unaffected.
- [x] 4.2 `go test ./parser/... -run 'TestParser_ParseStatements' -count=1` → all goldens match without `-update`.
- [x] 4.3 `go test ./parser/... -run 'TestParser_Format' -count=1` → all formatter goldens match without `-update`.
- [x] 4.4 `go test ./parser/... -run 'TestParser_FormatBeautify' -count=1` → all beautify goldens match without `-update`.

## 5. Close out

- [x] 5.1 `go test ./parser/... -count=1`. Confirm the only delta from the previous full-suite run is `TestParser_With_GlobalNotIn` flipping FAIL → PASS. All other out-of-scope FAILs and all PASSes are unchanged.
- [x] 5.2 `go vet ./parser/...` produces no new warnings (the pre-existing `WriteByte` notice in `parser/format.go` is acceptable).
- [x] 5.3 `openspec validate add-global-not-in` reports the change as valid.
