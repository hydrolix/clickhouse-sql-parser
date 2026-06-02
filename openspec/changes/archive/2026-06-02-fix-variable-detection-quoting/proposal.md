## Why

`matchVariable()` currently tests only the `String` prefix `$` on a `TokenKindIdent`, but the lexer strips quote characters before populating `String`. A backtick-quoted column name such as `` `$col` `` therefore satisfies the check and is misclassified as a Grafana template variable. That misclassification flows into `getNextPrecedence` (where it would be assigned `PrecedenceIndent`) and `parseInfix` (where it would be consumed as a binary operator), producing nonsensical parses for valid SQL that uses `$`-prefixed quoted identifiers.

## What Changes

- Narrow `matchVariable()` so that backtick-quoted identifiers (`Token.QuoteType == BackTicks`) never count as Grafana variables, regardless of their `$` prefix.
- Bare `$ident` and braced `${name}` / `${name:format}` continue to match, as do double-quoted identifiers (preserving existing behaviour for that quoting style).
- All call sites that go through `matchVariable()` — `getNextPrecedence`, `parseInfix`, `parseSettingsExpr` — inherit the correction with no further change.
- Rename the precedence constant `PrecedenceIndent` → `PrecedenceIdent` in `parser/parser_column.go`. The original name is a typo: the slot represents an *identifier-shaped* variable used as an operator, not "indentation". The constant is internal (lower-case package, not exported via any documented surface) so the rename has no external impact.

## Capabilities

### New Capabilities

_None._

### Modified Capabilities

- `grafana-variable-parsing`: the `matchVariable()` helper's contract is tightened to exclude backtick-quoted identifiers from the "current token is a variable" test.

## Impact

- `parser/parser_common.go` — single-line change to `matchVariable()`.
- `parser/parser_column.go` — rename `PrecedenceIndent` → `PrecedenceIdent` (declaration on line 10, use on line 41).
- No AST shape changes, no formatter changes, no exported-symbol changes — `PrecedenceIdent` stays package-private.
- Existing golden tests under `parser/testdata/` continue to pass unchanged; new coverage is added for the `` `$col` `` discrimination case.
