## 1. Baseline

- [x] 1.1 Confirm `parser/lexer_test.go` contains `TestConsumeHashComment` covering leading, trailing, and consecutive `#` comments. Starting state: FAIL.
- [x] 1.2 Confirm `parser/lexer_test.go` contains `TestConsumeComment` covering `--` and `/* */`. Starting state: PASS (regression guard).
- [x] 1.3 Capture the baseline: `go test ./parser/... -run 'TestConsumeHashComment|TestConsumeComment' -v -count=1`. `TestConsumeHashComment` must FAIL; `TestConsumeComment` must PASS. If the baseline does not match, stop and reconcile.

## 2. Implementation

- [x] 2.1 In `parser/lexer.go`, locate `func (l *Lexer) skipComments()`. Add a `case '#':` arm to the switch that calls `l.consumeSingleLineComment()` followed by `continue`. Place the arm next to the existing `case '-':` arm so the line-comment forms are grouped.
- [x] 2.2 `go build ./parser/...`.

## 3. Verify

- [x] 3.1 `go test ./parser/... -run 'TestConsumeHashComment' -v -count=1` → expect PASS.
- [x] 3.2 `go test ./parser/... -run 'TestConsumeComment' -v -count=1` → must remain PASS.
- [x] 3.3 `go test ./parser/... -run 'TestParser_ParseStatements' -count=1` → all goldens match without `-update`.
- [x] 3.4 `go test ./parser/... -run 'TestParser_Format' -count=1` → all formatter goldens match without `-update`.
- [x] 3.5 `go test ./parser/... -run 'TestParser_FormatBeautify' -count=1` → all beautify goldens match without `-update`.

## 4. Close out

- [x] 4.1 `go test ./parser/... -count=1`. Confirm the only delta from the previous full-suite run is `TestConsumeHashComment` flipping FAIL → PASS. Everything else (other out-of-scope FAILs and all PASSes) unchanged.
- [x] 4.2 `go vet ./parser/...` produces no new warnings.
- [x] 4.3 `openspec validate add-hash-line-comments` reports the change as valid.
