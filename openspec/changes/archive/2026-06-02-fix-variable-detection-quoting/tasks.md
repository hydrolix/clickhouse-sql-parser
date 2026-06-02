## 1. Tighten the helper

- [x] 1.1 Update `matchVariable()` in `parser/parser_common.go` so it requires `p.last().QuoteType != BackTicks` in addition to the existing `TokenKindIdent` + `$`-prefix checks.
- [x] 1.2 Rename the precedence constant `PrecedenceIndent` → `PrecedenceIdent` in `parser/parser_column.go` (declaration on line 10, single use on line 41). Confirm `grep -rn "PrecedenceIndent" .` returns no live-code hits afterward — only `.claude/PORTING_NOTES.md` and the archived porting change may still reference the old spelling, and those stay frozen.

## 2. Regression coverage

- [x] 2.1 Add a parser unit test that parses ``SELECT `$col` FROM t`` and asserts the projected expression is an `Ident` with `Name == "$col"` and `QuoteType == BackTicks` (not a `BinaryOperation`).
- [x] 2.2 Add a parser unit test that parses ``SELECT 1 FROM t WHERE `$col` = 1`` and asserts the WHERE predicate is a `BinaryOperation` whose `LeftExpr` is the backtick-quoted `Ident` and `Operation` is `=`.
- [x] 2.3 Add a focused unit test (or table case) for `matchVariable()` that covers: unquoted `$col`, unquoted `${tbl}`, unquoted `col`, backtick-quoted `` `$col` ``, double-quoted `"$col"` — verifying the new contract.

## 3. Verification

- [x] 3.1 Run `go test ./parser/...` and confirm all existing tests, including the new cases, pass without `-update`.
- [x] 3.2 Run `go build ./...` to confirm the change compiles cleanly.
