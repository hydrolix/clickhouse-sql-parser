## Why

Today, when the lexer hits a digit at the head of its input it commits to parsing a number — and if the input is *almost* a number but not quite (`123e` with a dangling exponent, `0xg` with a non-hex character, `1hello_world` that just happens to start with a digit), the lexer returns an error and the parser never sees the input. This makes the parser fragile for tools that handle identifier-shaped strings produced by templating systems, code generators, or schemas where backtick-quoting was omitted — strings that the rest of the SQL would happily accept as ordinary identifiers.

The fix is small and well-scoped: when number-scanning fails on a digit-prefixed input, the lexer rewinds and re-scans the same bytes as an identifier. Valid numbers continue to lex as numbers; only the "almost-number, actually-identifier" cases are rescued.

## What Changes

- The digit-case dispatch in `consumeToken` (`'0'…'9'`) snapshots the lexer state before calling `consumeNumber()`, and on error restores the snapshot and re-dispatches to `consumeIdent()`. Successful number scans are unaffected.
- Inputs like `123e`, `123e+`, `123E-`, `0x`, `0xg`, `123.456b`, `123.456e+10e`, and `1hello_world` cease to produce lexer errors. They tokenise as a single `TokenKindIdent` whose `String` field is the verbatim input.
- Two existing sub-tests under `TestConsumeNumber` (`Invalid number`, `Invalid float number`) become obsolete — they assert that the lexer errors on inputs that the new behaviour accepts. They are removed; the three already-present `Fork: …` sub-tests under the same `TestConsumeNumber` cover the same inputs with the new expectations.
- One entry in `TestParser_InvalidSyntax` — `"00e1d"`, originally guarding against a parser panic when the lexer returned an error with `lastToken == nil` — is removed. After this change the lexer no longer errors on that input, and the parser accepts a bare identifier at statement start (the same way it accepts `foo` or `abc123` today), so the `require.Error` assertion no longer holds. The panic guard is no longer relevant because the lexer never returns a nil-lastToken error for this input.
- No new token kind. No AST shape change. No parser-level work. No formatter change.

## Capabilities

### New Capabilities
- `numeric-identifier-fallback`: When the lexer attempts to tokenise a digit-prefixed input as a number and fails, it rewinds and re-scans the bytes as an identifier instead of returning a lexer error.

### Modified Capabilities
<!-- None. -->

## Impact

- **Code touched**: a small wrap of the digit case in `func (l *Lexer) consumeToken()` in `parser/lexer.go`. Existing `saveState()` / `restoreState()` helpers on the lexer are reused; no new plumbing is added.
- **Behavioural contract**: three sub-tests under `TestConsumeNumber` in `parser/lexer_test.go` (`Fork: Invalid number returns non-number kind without error`, `Fork: Invalid float returns non-number kind without error`, `Fork: Identifier with leading digit`) currently FAIL and flip to PASS after this change.
- **Tests obsoleted by this change** (must be removed as part of the change so the suite stays self-consistent):
  - `TestConsumeNumber/Invalid number` — asserts error on inputs the new behaviour accepts.
  - `TestConsumeNumber/Invalid float number` — same.
  - `TestParser_InvalidSyntax` entry `"00e1d"` — asserts error on a query the new behaviour accepts.
- **Regression guards** that must stay green: `TestConsumeNumber/Integer number`, `…/Hexadecimal number`, `…/Float number`, `…/Name`, `…/Keyword`; `TestConsumeString` and its sub-tests; `TestConsumeComment`; `TestConsumeHashComment`; `TestConsumeTextBlock`; and the full golden suite (`TestParser_ParseStatements`, `TestParser_Format`, `TestParser_FormatBeautify`). No `.sql` fixture under `parser/testdata/` uses an "almost-number" identifier, so golden output is expected to remain byte-identical.
- **No dependencies** added, no public API change, no breaking changes.
