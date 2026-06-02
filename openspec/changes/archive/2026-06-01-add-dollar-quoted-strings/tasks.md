## 1. Baseline

- [x] 1.1 Confirm `parser/lexer_test.go` contains `TestConsumeTextBlock` covering simple text blocks, numeric content, and embedded brace-variable / single quotes / spaces. Starting state: FAIL.
- [x] 1.2 Confirm `parser/lexer_test.go` contains `TestConsumeString` (with all sub-tests including the `Fork: Grafana-style mixed quotes` sub-test). Starting state: PASS (regression guard).
- [x] 1.3 Confirm `parser/lexer_test.go` contains `TestConsumeComment` and `TestConsumeHashComment`. Starting state: PASS (regression guards).
- [x] 1.4 Capture the baseline: `go test ./parser/... -run 'TestConsumeTextBlock|TestConsumeString|TestConsumeComment|TestConsumeHashComment' -v -count=1`. `TestConsumeTextBlock` must FAIL; the rest must PASS. If the baseline does not match, stop and reconcile.

## 2. Lexer: dispatch split in `consumeToken`

- [x] 2.1 In `parser/lexer.go`, locate the `case '`', '$', '"': return l.consumeIdent(Pos(l.current))` arm in `func (l *Lexer) consumeToken()`.
- [x] 2.2 Split it so `$` becomes its own arm: if `l.peekOk(1) && l.peekN(1) == '$'`, call `l.consumeString()`; otherwise call `l.consumeIdent(Pos(l.current))`. The `` ` `` and `"` characters keep their current routing to `consumeIdent`.
- [x] 2.3 `go build ./parser/...` to confirm the package compiles.

## 3. Lexer: text-block path in `consumeString`

- [x] 3.1 In `parser/lexer.go`, locate `func (l *Lexer) consumeString() error`. Introduce two local variables at the top: `start := 1` and `isTextBlock := false`. If `l.peekOk(0) && l.peekN(0) == '$' && l.peekOk(1) && l.peekN(1) == '$'`, set `start = 2` and `isTextBlock = true`.
- [x] 3.2 Initialise the scan index from `start` (not `1`).
- [x] 3.3 Inside the scan loop, branch on `isTextBlock`: when true, break when `l.peekN(i) == '$' && l.peekOk(i+1) && l.peekN(i+1) == '$'`. When false, retain today's single-quote terminator logic.
- [x] 3.4 After the loop, retain the existing unterminated-string check (`if !l.peekOk(i) { return errors.New("invalid string") }`). The same error fires for missing `'` and missing `$$`.
- [x] 3.5 Build the token with `String: l.slice(start, i)`, `Pos: Pos(l.current + start)`, and advance with `l.skipN(i + start)` so the closing `'` or `$$` is consumed.
- [x] 3.6 `go build ./parser/...`.

## 4. Verify the feature contract

- [x] 4.1 `go test ./parser/... -run 'TestConsumeTextBlock' -v -count=1` → expect PASS.
- [x] 4.2 `go test ./parser/... -run 'TestConsumeString' -v -count=1` → all sub-tests must remain PASS.
- [x] 4.3 `go test ./parser/... -run 'TestConsumeComment|TestConsumeHashComment' -v -count=1` → must remain PASS.
- [x] 4.4 `go test ./parser/... -run 'TestParser_ParseStatements' -count=1` → all goldens match without `-update`.
- [x] 4.5 `go test ./parser/... -run 'TestParser_Format' -count=1` → all formatter goldens match without `-update`.
- [x] 4.6 `go test ./parser/... -run 'TestParser_FormatBeautify' -count=1` → all beautify goldens match without `-update`.

## 5. Close out

- [x] 5.1 `go test ./parser/... -count=1`. Confirm the only delta from the previous full-suite run is `TestConsumeTextBlock` flipping FAIL → PASS. Other out-of-scope FAILs and all PASSes are unchanged.
- [x] 5.2 `go vet ./parser/...` produces no new warnings (the pre-existing `WriteByte` notice in `parser/format.go` is acceptable).
- [x] 5.3 `openspec validate add-dollar-quoted-strings` reports the change as valid.
