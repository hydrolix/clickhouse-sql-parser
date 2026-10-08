# Design: Float Values in SETTINGS

## Context

`parseSettingsExpr` (`parser/parser_table.go`) parses one `name = value` item. Every
settings list goes through it: `parseSettingsList` (SELECT `SETTINGS`, `FORMAT …
SETTINGS`, `SET`, `CREATE TABLE … SETTINGS`, `ALTER … MODIFY SETTING`) and
`parseDictionarySettingsClause`. Its value switch accepts `TokenKindInt`, `TokenKindString`,
`TokenKindLBrace` (map), `TRUE`/`FALSE` and a `$variable`. Anything else falls to the
default error.

The lexer already produces `TokenKindFloat` for `0.5`, `-0.5`, `+0.5`, `1.` and `1.5e-3`
(a sign is folded into the number when the previous token is not an operand, which holds
after `=`). It produces `TokenKindInt` for `1e-3` (no dot), which is why that form parses
today. `.5` lexes as `TokenKindDot` followed by `TokenKindInt`.

`parseNumber` (`parser/parser_common.go`) already handles all three cases: `Int`, `Float`,
and `Dot` + base-10 `Int`, which it joins into `".5"` with `NumPos` at the dot. It is what
expression parsing uses for numeric literals.

ClickHouse 26.8 (`clickhouse-local`) accepts, for a float setting: `0.5`, `.5`, `-.5`,
`1.`, `1.5e-1`, and `0.5` after `FORMAT … SETTINGS`. It accepts `0.5` for an integer
setting (`max_threads`). It rejects `inf` and `nan` for a float setting with "Float setting
value must be finite" — a value error, not a syntax error.

## Goals / Non-Goals

**Goals:**

- Accept decimal float literals as settings values in every settings list.
- Keep the AST shape and all existing goldens unchanged.

**Non-Goals:**

- **Sign before a leading dot (`-.5`, `+.5`).** The lexer only folds a sign into a number
  when a digit follows it, so `-.5` lexes as `-`, `.`, `5`. Supporting it means either a
  lexer change that affects every expression or parser-side sign handling in settings.
  Not needed by any known query; can follow separately.
- **Hexadecimal floats (`0x1p3`).** The lexer doesn't recognise a binary exponent after a
  hex mantissa.
- **`inf` / `nan`.** They lex as identifiers, and ClickHouse rejects them as values for
  float settings anyway.
- **Arbitrary expressions as values** (`x = 1 + 1`, arrays, tuples).

## Decisions

### D1 — Route Float and leading-dot tokens to `parseNumber`

Change the first case of the value switch to:

```go
case p.matchTokenKind(TokenKindInt), p.matchTokenKind(TokenKindFloat), p.matchTokenKind(TokenKindDot):
```

The body (`p.parseNumber(p.Pos())`) is unchanged.

- **Why `parseNumber`:** it is the one place that turns number tokens into a
  `NumberLiteral`, including the `.5` join and the base-10 check after a dot. Reusing it
  keeps settings values identical to numeric literals in expressions.
- **Why include `TokenKindDot`:** without it `.5` still fails, and ClickHouse accepts it.
  A bare `.` that isn't followed by a base-10 integer still errors, from `parseNumber`.
- **Alternative — parse the value with the general expression parser:** would also accept
  `-.5` and arbitrary expressions, but changes the AST type of existing values (e.g. a
  signed integer could become a unary expression), shifts goldens, and widens the accepted
  grammar well beyond what ClickHouse allows. Rejected.

### D2 — No AST or formatter change

A float value is a `NumberLiteral` whose `Literal` is the source text (`"0.5"`, `"-0.5"`,
`".5"`, `"1."`). `Format` already emits `Literal` verbatim, and `Walk` / `Accept` already
visit a `SettingExpr`'s value. `NumEnd` is the token end, so `SettingsClause.ListEnd` is
the end of the last value's text with no closing delimiter to compensate — the same as
for integers. chproxy's splice logic relies on that.

## Risks / Trade-offs

- [`. 5` with a space is accepted and formats as `.5`] → Existing `parseNumber` behaviour,
  shared with expressions (`SELECT . 5`). Not widened by this change in a way that matters;
  left as is.
- [`1.` formats as `1.`] → Kept verbatim on purpose: the fixed point holds and the
  formatter never rewrites literals.
- [A later upstream sync touches the same switch] → The change is one line in one `case`;
  conflicts are trivial. Worth offering upstream.

## Golden footprint

- Existing fixtures: no change in any golden family. Verified by `git status` after
  `make update_test` showing only new files.
- New fixtures: `query/select_with_float_settings.sql`,
  `ddl/create_table_with_float_settings.sql`, `ddl/alter_table_modify_float_setting.sql`
  and `basic/set_float_statement.sql`, each adding its own JSON, format and beautify
  goldens.
