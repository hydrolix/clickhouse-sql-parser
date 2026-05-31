## 1. Test fixtures (the behavioural contract)

Each test below corresponds to one requirement in `specs/grafana-variable-parsing/spec.md`. Most are already present in `parser/parser_test.go` and `parser/lexer_test.go` on this branch — the task is to confirm presence and starting state, and to add any that are missing. The "starting state" column says what the test does on the **un**-modified codebase, so that flipping it to PASS becomes the implementation's pass criterion.

- [x] 1.1 Confirm `parser/parser_test.go` contains `TestParser_Var_TopLevel` exercising `${var}` as an expression operand in WHERE and the select list. Starting state: FAIL.
- [x] 1.2 Confirm `parser/parser_test.go` contains `TestParser_Var_InFromClause` exercising `${var}` as a table name in `FROM`. Starting state: FAIL.
- [x] 1.3 Confirm `parser/parser_test.go` contains `TestParser_Var_FormatSuffix` exercising `${var:format}`. Starting state: FAIL.
- [x] 1.4 Confirm `parser/parser_test.go` contains `TestParser_Var_InFunctionArg` exercising `${var}` inside function-call argument lists. Starting state: FAIL.
- [x] 1.5 Confirm `parser/parser_test.go` contains `TestParser_Var_AsInfixOperator` exercising `${VAR}` between two operands. Starting state: FAIL.
- [x] 1.6 Confirm `parser/parser_test.go` contains `TestParser_With_VariableInSettings` exercising `${var}` and `$var` as `SETTINGS` values. Starting state: FAIL.
- [x] 1.7 Confirm `parser/parser_test.go` contains `TestParser_Var_BareDollarIdent` exercising bare `$ident`. Starting state: PASS (regression guard).
- [x] 1.8 Confirm `parser/parser_test.go` contains `TestParser_ExtractStillParses` exercising `EXTRACT(unit FROM expr)` and `extract(col, regex)`. Starting state: PASS (regression guard).
- [x] 1.9 If any test in 1.1–1.8 is missing, add it using the scenarios listed in `specs/grafana-variable-parsing/spec.md` as the test cases.
- [x] 1.10 Capture the baseline by running `go test ./parser/... -run 'TestParser_Var_|TestParser_With_VariableInSettings|TestParser_ExtractStillParses' -v -count=1`. The six feature tests must currently FAIL and the two regression guards must currently PASS. If the baseline does not match, stop and reconcile before proceeding.

## 2. Lexer: braced variable in `consumeIdent`

- [x] 2.1 In `parser/lexer.go`, locate `func (l *Lexer) consumeIdent(_ Pos) error`. In the unquoted-ident branch, after the existing optional leading `$` consumption, detect a following `{` and enter "variable" mode.
- [x] 2.2 In variable mode, keep advancing while the next byte is an ident-part OR `:` (so format suffixes are absorbed).
- [x] 2.3 At the end of variable mode, require a closing `}`. If present, consume it. If absent, return `fmt.Errorf("unclosed variable: %s", l.slice(0, i))`.
- [x] 2.4 Confirm the produced token kind is `TokenKindIdent` and its `String` field contains the verbatim source text from the leading `$` through the closing `}` (including any `:format` suffix).
- [x] 2.5 Sanity-check at the lexer level with a one-off ad-hoc snippet (do not commit) that `${y}` and `${y:fmt}` each produce a single `TokenKindIdent` with the expected `String`, and that `${y` returns the documented `unclosed variable:` error.

## 3. Parser helper: `matchVariable()`

- [x] 3.1 In `parser/parser_common.go`, add the helper:
  ```go
  func (p *Parser) matchVariable() bool {
      return p.matchTokenKind(TokenKindIdent) && strings.HasPrefix(p.last().String, "$")
  }
  ```
  Place it near the other `match*` helpers. Add `strings` to the import block if not already there.
- [x] 3.2 `go build ./parser/...` to confirm the package compiles.

## 4. Parser: variable as infix operator in `parser_column.go`

- [x] 4.1 Insert a `PrecedenceIndent` constant into the `const ( PrecedenceUnknown = iota; PrecedenceOr; … )` block, positioned between `PrecedenceUnknown` and `PrecedenceOr` (so its integer value is exactly one above unknown, one below `OR`). See design.md Decision 3 for why.
- [x] 4.2 In `getNextPrecedence`, add `case p.matchVariable(): return PrecedenceIndent`. Place it immediately after the `KeywordOr` arm and before `KeywordAnd`.
- [x] 4.3 In `parseInfix`, add `p.matchVariable()` to the set of operator predicates that consumes the current token and treats it as a binary operator (alongside the existing `TokenKindPlus`, `TokenKindMinus`, etc.).
- [x] 4.4 Do NOT change the `case p.matchKeyword(KeywordExtract):` arm in `parseColumnExpr` — it must continue to handle the `EXTRACT(unit FROM expr)` special form. (Regression guard: `TestParser_ExtractStillParses`.)
- [x] 4.5 `go build ./parser/...`.

## 5. Parser: variable as SETTINGS value in `parser_table.go`

- [x] 5.1 In `func (p *Parser) parseSettingsExprList(...)`, locate the switch that decides how to consume the value side of a settings entry. Add a `case p.matchVariable():` arm that calls `p.parseIdent()` and assigns the result to `expr`. The setting key remains restricted to its existing grammar — do NOT add `matchVariable()` to the key side.
- [x] 5.2 `go build ./parser/...`.

## 6. Verify the feature tests flip from FAIL to PASS

- [x] 6.1 `go test ./parser/... -run 'TestParser_Var_TopLevel' -v -count=1` → expect PASS.
- [x] 6.2 `go test ./parser/... -run 'TestParser_Var_InFromClause' -v -count=1` → expect PASS.
- [x] 6.3 `go test ./parser/... -run 'TestParser_Var_FormatSuffix' -v -count=1` → expect PASS.
- [x] 6.4 `go test ./parser/... -run 'TestParser_Var_InFunctionArg' -v -count=1` → expect PASS.
- [x] 6.5 `go test ./parser/... -run 'TestParser_Var_AsInfixOperator' -v -count=1` → expect PASS.
- [x] 6.6 `go test ./parser/... -run 'TestParser_With_VariableInSettings' -v -count=1` → expect PASS.

## 7. Verify regression guards stay PASS

- [x] 7.1 `go test ./parser/... -run 'TestParser_Var_BareDollarIdent' -v -count=1` → must remain PASS.
- [x] 7.2 `go test ./parser/... -run 'TestParser_ExtractStillParses' -v -count=1` → must remain PASS. If this fails, the `KeywordExtract` arm in `parseColumnExpr` was disturbed — revert task 4.4's no-op.
- [x] 7.3 `go test ./parser/... -run 'TestParser_ParseStatements' -count=1` → all golden files match. **Do not run with `-update`.** A diff here indicates an unintended rendering change.
- [x] 7.4 `go test ./parser/... -run 'TestParser_Format' -count=1` → all formatter goldens match. Same `-update` prohibition.
- [x] 7.5 `go test ./parser/... -run 'TestParser_FormatBeautify' -count=1` → all beautify goldens match. Same `-update` prohibition.

## 8. Full suite + housekeeping

- [x] 8.1 `go test ./parser/... -count=1`. Capture the new pass/fail tally and compare to the baseline from task 1.10. Expected diff: the six feature tests transition FAIL → PASS; nothing else changes. Tests for unrelated features (REGEXP, `$$` text blocks, `#` comments, etc.) remain in whatever state they were in before — they are out of scope for this change.
- [x] 8.2 `go vet ./parser/...`. The pre-existing `WriteByte` warning in `parser/format.go` is acceptable; no new warnings introduced.
- [x] 8.3 Run `openspec verify-change port-grafana-variable-support` (or `/opsx:verify`) and resolve any findings before considering the change complete.
