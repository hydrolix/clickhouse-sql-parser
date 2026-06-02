## Context

The Grafana-variable parsing feature added in `port-grafana-variable-support` introduced a single helper `matchVariable()` on `*Parser` that branches on "is the current token a `$`-prefixed identifier". The lexer (`consumeIdent` in `parser/lexer.go`) populates `Token.String` with the slice between the opening and closing quote characters when an identifier is quoted, i.e. the quote chars are stripped. As a result `` `$col` `` and `"$col"` both produce `Token.String == "$col"` with `QuoteType == BackTicks` and `DoubleQuote` respectively. The current `matchVariable()` ignores `QuoteType`, so any quoted identifier whose body begins with `$` is misclassified as a Grafana variable.

That misclassification has three downstream effects, all in `parser/parser_column.go` and `parser/parser_table.go`:

1. `getNextPrecedence` returns `PrecedenceIdent` for the quoted ident, treating it as an infix operator.
2. `parseInfix` then consumes the quoted ident as a binary-operator token, producing a `BinaryOperation` AST node with nonsensical operands.
3. `parseSettingsExpr` accepts the quoted ident as the value side of a `key = …` setting.

There is no real ClickHouse SQL where a backtick-quoted identifier starting with `$` should be interpreted as a Grafana template variable — the whole point of backticks is to mark the contents as an ordinary identifier.

## Goals / Non-Goals

**Goals:**
- Make `matchVariable()` return `false` for backtick-quoted identifiers regardless of their body, so `` `$col` `` is treated as an ordinary identifier in every parser branch that consults the helper.
- Preserve the existing behaviour for bare `$ident`, `${name}`, `${name:format}`, and (intentionally) for double-quoted `"$col"`.
- Add regression coverage that pins the new behaviour at the parser-public-API level.

**Non-Goals:**
- Changing the lexer's `String` representation of quoted identifiers (e.g. re-introducing the quote characters into `String`) — that would ripple through every consumer of `Token.String`.
- Re-examining whether double-quoted `"$col"` should also be excluded — current callers and downstream consumers (Grafana data sources) accept this; revisiting it is out of scope for this fix.
- Touching `parseSettingsExpr`, `parseInfix`, or `getNextPrecedence` directly — the fix is funnelled through the single helper so callers stay untouched.

## Decisions

**Decision: gate on `QuoteType != BackTicks` rather than `QuoteType == Unquoted`.**

Rationale: backticks are the unambiguous "this is a column/table identifier" marker in ClickHouse syntax. Double-quoted identifiers are used in a wider set of contexts (some of which interoperate with Grafana variable substitution conventions), and excluding them by reflex would change observable behaviour for users who currently rely on `"${name}"` parsing as a variable. Targeting only backticks keeps the fix minimally invasive.

Alternative considered: `QuoteType == Unquoted`. Rejected because it would silently drop double-quoted variable matches that the existing `port-grafana-variable-support` test suite does not pin but live consumers may depend on.

**Decision: keep the fix inside `matchVariable()` and do not touch its call sites.**

Rationale: `matchVariable()` was introduced as the single source of truth for the "is-variable" predicate (see existing requirement in `grafana-variable-parsing`). Tightening it at one site automatically corrects `getNextPrecedence`, `parseInfix`, and `parseSettingsExpr` with no risk of one branch drifting from another.

**Decision: rename `PrecedenceIndent` → `PrecedenceIdent`.**

Rationale: the original name is a typo of "Ident" (identifier). "Indent" suggests indentation, which is unrelated to operator precedence and misleads readers who scan the precedence ladder. The constant is package-private (`parser`-scoped, no documentation references it outside the archived porting notes), so the rename is mechanical: one declaration, one use site, both inside `parser/parser_column.go`. Bundling the rename into this change keeps the touched call sites consistent with the new requirement wording.

## Risks / Trade-offs

- **Risk**: A user genuinely intended `` `$macro` `` to be a Grafana variable wrapped in backticks. → Mitigation: this pattern has no precedent in Grafana's variable syntax — Grafana substitutes `$var` and `${var}`, never `` `$var` ``. Backticked variables would not survive Grafana's substitution layer in any case.
- **Trade-off**: Double-quoted `"$col"` remains classified as a variable. This is intentional (see the Decisions section) but should be revisited if a future report shows it causes a real parse error on otherwise-valid SQL.
