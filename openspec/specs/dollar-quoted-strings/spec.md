## Purpose

Recognise the untagged `$$ … $$` dollar-quoted string literal form in the ClickHouse SQL lexer so queries can embed arbitrary text — including single quotes, backslashes, brace-wrapped variable placeholders, and SQL comment sequences — without escaping. Today the lexer treats `$$` as the start of two identifier tokens, so any input containing a dollar-quoted block fails to lex before the parser ever sees it. This is a lexer-only extension that does not change the parser, the AST, the formatter, or any existing golden test fixture.

## Requirements

### Requirement: Lexer SHALL recognise `$$ … $$` as a single string literal

When `consumeToken` sees `$` followed immediately by another `$` at the head of the lookahead stream, the lexer SHALL enter a text-block scanning mode and emit a single token of kind `TokenKindString`. The token's `String` field SHALL contain the verbatim bytes between the opening `$$` and the closing `$$`, with neither marker included. The lexer SHALL advance past the closing `$$` after emitting the token.

#### Scenario: Simple text block
- **WHEN** the lexer consumes the input `$$hello world$$`
- **THEN** the resulting token has kind `TokenKindString` and `String == "hello world"`

#### Scenario: Numeric text block
- **WHEN** the lexer consumes the input `$$123$$`
- **THEN** the resulting token has kind `TokenKindString` and `String == "123"`

#### Scenario: Empty text block
- **WHEN** the lexer consumes the input `$$$$`
- **THEN** the resulting token has kind `TokenKindString` and `String == ""`

### Requirement: Text-block content SHALL be taken verbatim

Inside `$$ … $$`, the lexer SHALL NOT apply any escape, quote-doubling, comment-recognition, or identifier-recognition logic. Every byte between the opening and closing markers is part of the string content, including single quotes, backslashes, brace-wrapped placeholders, and SQL comment sequences. The closing `$$` is the only terminator.

#### Scenario: Embedded brace-variable, single quotes, and trailing space
- **WHEN** the lexer consumes the input `$$${variable:format} and 'string' $$`
- **THEN** the resulting token has kind `TokenKindString` and `String == "${variable:format} and 'string' "`

### Requirement: Unterminated text block SHALL produce a lexer error

If the lexer reaches end-of-input while scanning a text block (no closing `$$` seen), it SHALL return an error whose message is `invalid string` — the same wording already used for unterminated single-quoted strings.

#### Scenario: Missing closing marker
- **WHEN** the lexer consumes the input `$$hello world`
- **THEN** the lexer returns an error whose message is `invalid string`

### Requirement: Single `$` SHALL continue to flow into the identifier path

A `$` that is not immediately followed by another `$` SHALL be consumed by `consumeIdent`, preserving the existing behaviour for bare `$ident` and brace-wrapped `${name}` / `${name:format}` placeholders.

#### Scenario: Bare `$ident` still lexes as an identifier
- **WHEN** the lexer consumes the input `$col`
- **THEN** the resulting token has kind `TokenKindIdent` and `String == "$col"`

#### Scenario: Brace-wrapped variable still lexes as an identifier
- **WHEN** the lexer consumes the input `${tbl}`
- **THEN** the resulting token has kind `TokenKindIdent` and `String == "${tbl}"`

#### Scenario: Brace-wrapped variable with format suffix still lexes as an identifier
- **WHEN** the lexer consumes the input `${y:sqlstring}`
- **THEN** the resulting token has kind `TokenKindIdent` and `String == "${y:sqlstring}"`

### Requirement: Existing string and lexer behaviour SHALL be preserved

This change SHALL NOT alter how `'…'` single-quoted strings are scanned, how comments (`--`, `/* */`, `#`) are skipped, or how any non-`$$` input is tokenised. All existing golden-file fixtures under `parser/testdata/` SHALL continue to match byte-for-byte without `-update`.

#### Scenario: Single-quoted strings unchanged
- **WHEN** the lexer consumes any input handled by the existing `TestConsumeString` cases (simple strings, backslash-escaped quotes, doubled quotes, backslash-escaped backslashes, and the Grafana-style mixed-quote sub-test)
- **THEN** the resulting tokens match the same `TokenKindString` and `String` values that the test currently asserts

#### Scenario: Golden tests remain green
- **WHEN** `TestParser_ParseStatements`, `TestParser_Format`, and `TestParser_FormatBeautify` are run after this change
- **THEN** every golden file matches without `-update`
