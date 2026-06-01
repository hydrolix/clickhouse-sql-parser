## 1. Baseline

- [x] 1.1 Confirm `parser/lexer_test.go` contains the three `Fork: …` sub-tests under `TestConsumeNumber`: `Fork: Invalid number returns non-number kind without error`, `Fork: Invalid float returns non-number kind without error`, `Fork: Identifier with leading digit`. Starting state: all three FAIL.
- [x] 1.2 Confirm `parser/lexer_test.go` contains the existing `Invalid number` and `Invalid float number` sub-tests under `TestConsumeNumber`. Starting state: both PASS (they assert errors on the same inputs the Fork sub-tests will accept).
- [x] 1.3 Confirm `parser/parser_test.go` contains `TestParser_InvalidSyntax` with the entry `"00e1d", // invalid number that leaves lastToken nil`. Starting state: PASS.
- [x] 1.4 Capture the baseline: `go test ./parser/... -run 'TestConsumeNumber|TestParser_InvalidSyntax' -v -count=1`. Record which sub-tests PASS and FAIL. The three Fork sub-tests must FAIL; the rest must PASS. If the baseline does not match, stop and reconcile.

## 2. Implementation

- [x] 2.1 In `parser/lexer.go`, locate the digit case in `func (l *Lexer) consumeToken()`: `case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9': return l.consumeNumber()`.
- [x] 2.2 Wrap the call to use the existing `saveState()` / `restoreState()` helpers — see design.md Decision 1 for the exact shape:

  ```go
  case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
      savedState := l.saveState()
      if err := l.consumeNumber(); err != nil {
          l.restoreState(savedState)
          return l.consumeIdent(Pos(l.current))
      }
      return nil
  ```

- [x] 2.3 `go build ./parser/...` to confirm the package compiles. No other lexer code needs to change.

## 3. Remove obsolete sub-tests

- [x] 3.1 In `parser/lexer_test.go`, remove the `t.Run("Invalid number", …)` sub-test inside `TestConsumeNumber`. Its assertions (`require.Error(t, err)`) contradict the new behaviour and the same inputs are already covered by `Fork: Invalid number returns non-number kind without error`.
- [x] 3.2 In `parser/lexer_test.go`, remove the `t.Run("Invalid float number", …)` sub-test inside `TestConsumeNumber`. Same rationale as 3.1; coverage is preserved by `Fork: Invalid float returns non-number kind without error`.
- [x] 3.3 In `parser/parser_test.go`, remove the `"00e1d"` entry (and its `// invalid number that leaves lastToken nil` comment) from the `invalidSQLs` slice inside `TestParser_InvalidSyntax`. The original panic-guard concern no longer applies because the lexer never returns a nil-lastToken error for this input after this change.
- [x] 3.4 `go build ./parser/...` to confirm tests still compile.

## 4. Verify the Fork sub-tests flip to PASS

- [x] 4.1 `go test ./parser/... -run 'TestConsumeNumber/Fork:_Invalid_number_returns_non-number_kind_without_error' -v -count=1` → expect PASS.
- [x] 4.2 `go test ./parser/... -run 'TestConsumeNumber/Fork:_Invalid_float_returns_non-number_kind_without_error' -v -count=1` → expect PASS.
- [x] 4.3 `go test ./parser/... -run 'TestConsumeNumber/Fork:_Identifier_with_leading_digit' -v -count=1` → expect PASS.
- [x] 4.4 `go test ./parser/... -run 'TestConsumeNumber' -v -count=1` → the surviving sub-tests (`Integer number`, `Hexadecimal number`, `Float number`, `Name`, `Keyword`, and the three Fork sub-tests) must all PASS. No `Invalid number` or `Invalid float number` sub-tests run (they were removed in section 3).

## 5. Verify regression guards stay green

- [x] 5.1 `go test ./parser/... -run 'TestParser_InvalidSyntax' -v -count=1` → must PASS. The `"00e1d"` entry is removed in 3.3; all remaining entries should still produce errors.
- [x] 5.2 `go test ./parser/... -run 'TestConsumeString|TestConsumeComment|TestConsumeHashComment|TestConsumeTextBlock' -v -count=1` → all PASS.
- [x] 5.3 `go test ./parser/... -run 'TestParser_ParseStatements' -count=1` → all goldens match without `-update`.
- [x] 5.4 `go test ./parser/... -run 'TestParser_Format' -count=1` → all formatter goldens match without `-update`.
- [x] 5.5 `go test ./parser/... -run 'TestParser_FormatBeautify' -count=1` → all beautify goldens match without `-update`.

## 6. Close out

- [x] 6.1 `go test ./parser/... -count=1`. Confirm the deltas from the previous full-suite baseline: three Fork sub-tests transition FAIL → PASS; two upstream sub-tests (`Invalid number`, `Invalid float number`) and one parser-test entry (`"00e1d"`) are gone; nothing else moves.
- [x] 6.2 `go vet ./parser/...` produces no new warnings (the pre-existing `WriteByte` notice in `parser/format.go` is acceptable).
- [x] 6.3 `openspec validate add-numeric-identifier-fallback` reports the change as valid.
