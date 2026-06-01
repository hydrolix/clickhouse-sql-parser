## Purpose

When the ClickHouse SQL lexer hits a digit at the head of its input it commits to parsing a number — and if the input is *almost* a number but not quite (`123e` with a dangling exponent, `0xg` with a non-hex character, `1hello_world` that just happens to start with a digit), the lexer returns an error and the parser never sees the input. This makes the parser fragile for tools that handle identifier-shaped strings produced by templating systems, code generators, or schemas where backtick-quoting was omitted — strings that the rest of the SQL would happily accept as ordinary identifiers. The fix is small and well-scoped: when number-scanning fails on a digit-prefixed input, the lexer rewinds and re-scans the same bytes as an identifier. Valid numbers continue to lex as numbers; only the "almost-number, actually-identifier" cases are rescued.

## Requirements

### Requirement: Lexer SHALL re-scan a digit-prefixed input as an identifier when number-scanning fails

When the lexer dispatches a digit-prefixed input (`'0'` through `'9'`) to `consumeNumber()` and that call returns an error, the lexer SHALL:

1. Restore the byte cursor (and the rest of its state) to the position it held immediately before the failed `consumeNumber()` call.
2. Re-dispatch the same input to `consumeIdent()`.
3. Return the token produced by `consumeIdent()` — kind `TokenKindIdent`, with the verbatim input bytes as its `String`.

When `consumeNumber()` succeeds, the lexer SHALL behave exactly as it does today — no rewind, no re-dispatch.

#### Scenario: Invalid exponent recovers as identifier
- **WHEN** the lexer consumes the input `123e`
- **THEN** `consumeToken` returns no error AND the resulting token has kind `TokenKindIdent` AND `String == "123e"`

#### Scenario: Invalid hex literal recovers as identifier
- **WHEN** the lexer consumes the input `0xg`
- **THEN** `consumeToken` returns no error AND the resulting token has kind `TokenKindIdent` AND `String == "0xg"`

#### Scenario: Identifier with leading digit
- **WHEN** the lexer consumes the input `1hello_world`
- **THEN** `consumeToken` returns no error AND the resulting token has kind `TokenKindIdent` AND `String == "1hello_world"`

#### Scenario: Valid integer still lexes as a number
- **WHEN** the lexer consumes the input `123`
- **THEN** `consumeToken` returns no error AND the resulting token has kind `TokenKindInt` AND `String == "123"`

#### Scenario: Valid float still lexes as a number
- **WHEN** the lexer consumes the input `123.456e+10`
- **THEN** `consumeToken` returns no error AND the resulting token has kind `TokenKindFloat` AND `String == "123.456e+10"`

#### Scenario: Valid hex literal still lexes as a number
- **WHEN** the lexer consumes the input `0x1F`
- **THEN** `consumeToken` returns no error AND the resulting token has kind `TokenKindInt`

### Requirement: The lexer SHALL NOT emit a number kind for an invalid number

For every input where `consumeNumber()` returns an error, the lexer's `lastToken.Kind` AFTER the fallback SHALL be neither `TokenKindInt` nor `TokenKindFloat`. The fallback's contract is that an input which is not a valid number tokenises as something OTHER than a number kind.

#### Scenario: Invalid-number fallback never yields TokenKindInt
- **WHEN** the lexer consumes any of `"123e"`, `"123e+"`, `"123e-"`, `"123E"`, `"123E+"`, `"123E-"`, `"0x"`, `"0xg"`
- **THEN** for each input the resulting token's kind is not `TokenKindInt` and not `TokenKindFloat`

#### Scenario: Invalid-float fallback never yields TokenKindFloat
- **WHEN** the lexer consumes any of `"123.456b"`, `"123.456e"`, `"123.456e+"`, `"123.456e-"`, `"123.456e+10e"`, `"123.456e-10e"`, `"123.456e10e"`, `"123.456E10e"`, `"123.456E+10e"`, `"123.456E-10e"`, `"123.456e+10e+10"`
- **THEN** for each input the resulting token's kind is not `TokenKindInt` and not `TokenKindFloat`

### Requirement: Existing tokenisation behaviour SHALL be preserved

This change SHALL NOT alter the lexer's behaviour for any input that does not start with a digit, SHALL NOT change the token kind or `String` field produced by any input that scans successfully as a number, and SHALL NOT change any other lexer error path (unclosed string, unclosed quoted identifier, etc.). All existing golden-file fixtures under `parser/testdata/` SHALL continue to match byte-for-byte without `-update`.

#### Scenario: Non-digit-prefixed inputs unchanged
- **WHEN** any input handled by the existing `TestConsumeString`, `TestConsumeComment`, `TestConsumeHashComment`, `TestConsumeTextBlock`, or the `Integer number` / `Hexadecimal number` / `Float number` / `Name` / `Keyword` sub-tests of `TestConsumeNumber` is tokenised
- **THEN** the result matches today's behaviour exactly

#### Scenario: Golden tests remain green
- **WHEN** `TestParser_ParseStatements`, `TestParser_Format`, and `TestParser_FormatBeautify` are run after this change
- **THEN** every golden file matches without `-update`
