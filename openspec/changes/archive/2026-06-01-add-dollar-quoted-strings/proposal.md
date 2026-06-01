## Why

ClickHouse SQL accepts a dollar-quoted string literal form, written `$$ … $$`, that lets queries embed arbitrary text — including single quotes, backslashes, brace-wrapped variable placeholders, and even SQL comments — without escaping. It is the standard escape hatch for SQL fragments that contain characters which would otherwise have to be doubled or backslash-escaped, and it is heavily used for embedding template fragments and pre-formatted snippets.

Today the lexer treats `$$` as the start of two identifier tokens, so any input containing a dollar-quoted block fails to lex before the parser ever sees it. This change teaches the lexer to recognise the untagged `$$ … $$` form.

## What Changes

- The lexer detects `$$` at the head of the lookahead stream and consumes everything up to the next `$$` as a single string literal. The resulting token has kind `TokenKindString` and its `String` field carries the verbatim bytes between the two `$$` markers — markers themselves are not included.
- A single `$` followed by anything other than `$` continues to flow into the identifier path, preserving today's behaviour for bare `$ident` and brace-wrapped `${name}` / `${name:format}` placeholders.
- An unterminated dollar-quoted block (no closing `$$` before end-of-input) returns the same `invalid string` lexer error that other unterminated string literals produce.
- No new token kind. No AST shape change. No parser change. No formatter change.

## Capabilities

### New Capabilities
- `dollar-quoted-strings`: Recognise `$$ … $$` as an untagged dollar-quoted string literal in the SQL lexer. The contents are taken verbatim; the only terminator is the next `$$` sequence.

### Modified Capabilities
<!-- None. -->

## Impact

- **Code touched**: two edits in `parser/lexer.go` — a small dispatch refinement in `consumeToken` (route `$$…` to `consumeString`, keep single `$…` going to `consumeIdent`) and an additional "text block" branch in `consumeString` that scans to the matching `$$`.
- **Behavioural contract**: a lexer-level test (`TestConsumeTextBlock` in `parser/lexer_test.go`) exercises three shapes — `$$hello world$$`, `$$123$$`, and `$$${variable:format} and 'string' $$` — and is the gating signal for the implementation.
- **Regression guards** that must stay green: `TestConsumeString` (every sub-test, including the mixed-quote escape sub-test), `TestConsumeComment`, `TestConsumeHashComment`, and the entire golden suite (`TestParser_ParseStatements`, `TestParser_Format`, `TestParser_FormatBeautify`). No `.sql` fixture under `parser/testdata/` uses `$$` today, so golden output is expected to remain byte-identical.
- **Out of scope**: tagged dollar-quoting (`$tag$ … $tag$`). Only the untagged form is recognised by this change; tagged quoting can be added later as a MODIFIED requirement to the same capability if a real use case appears.
- **No dependencies** added, no public API change, no breaking changes.
