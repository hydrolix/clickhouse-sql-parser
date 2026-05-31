## ADDED Requirements

### Requirement: Lexer SHALL treat `#` as the start of a single-line comment

When `skipComments` encounters a `#` at the head of the lookahead stream, the lexer SHALL discard every byte from the `#` up to (and including) the next newline character, or to end-of-input if no newline follows. The discarded bytes SHALL NOT produce a token and SHALL NOT affect the AST.

#### Scenario: Leading hash comment followed by a statement
- **WHEN** `# leading comment\nSELECT 1` is parsed
- **THEN** `ParseStmts` returns no error and produces a non-empty statement list whose first statement is the `SELECT 1`

#### Scenario: Trailing hash comment after a statement
- **WHEN** `SELECT 1 # trailing comment` is parsed
- **THEN** `ParseStmts` returns no error and produces a non-empty statement list whose first statement is the `SELECT 1`

#### Scenario: Multiple consecutive hash comments
- **WHEN** `# c1\n# c2\nSELECT 1` is parsed
- **THEN** `ParseStmts` returns no error and produces a non-empty statement list whose first statement is the `SELECT 1`

### Requirement: Existing comment forms SHALL be preserved

`--` single-line comments and `/* … */` multi-line comments SHALL continue to lex exactly as before this change.

#### Scenario: `--` single-line comment still works
- **WHEN** the lexer consumes `-- comment` (or any of the existing variants tested by `TestConsumeComment`)
- **THEN** `consumeToken` returns no error and the comment text is discarded

#### Scenario: `/* … */` multi-line comment still works
- **WHEN** the lexer consumes `/* comment */`
- **THEN** `consumeToken` returns no error and the comment text is discarded

### Requirement: Hash comments SHALL NOT affect existing golden files

No `.sql` fixture under `parser/testdata/` uses `#` as a comment today. This change SHALL preserve the byte-for-byte output of every golden file produced by `TestParser_ParseStatements`, `TestParser_Format`, and `TestParser_FormatBeautify`.

#### Scenario: Golden tests remain green
- **WHEN** the full golden suite is run after this change is applied
- **THEN** every golden file matches without `-update`
