## Context

`parseSelectItem` in `parser/parser_column.go` (around line 811) has a modifier loop that recognises the three modifier keywords that can follow a select-item expression: `EXCEPT`, `APPLY`, `REPLACE`. The loop currently dispatches all three to the same helper:

```go
for {
    if p.matchKeyword(KeywordExcept) || p.matchKeyword(KeywordApply) || p.matchKeyword(KeywordReplace) {
        modifier, err := p.parseFunctionExpr(p.Pos())
        ...
    } else {
        break
    }
}
```

`parseFunctionExpr` (around line 631) consumes the modifier name as an identifier (keywords are accepted in identifier position by `parseIdent`) and then immediately calls `parseFunctionParams`. `parseFunctionParams`'s first line is `expectTokenKind(TokenKindLParen)` — so any modifier without an immediately-following `(` fails with `expected the last token kind is: (, but got <…>`.

This is fine for `APPLY` (`APPLY(sum)`) and `REPLACE` (`REPLACE(i+1 AS i)`), which always take a parenthesised argument. But ClickHouse SQL accepts `EXCEPT` in two shapes: the parens form `EXCEPT (col1, col2)` (multi-column or single-column) and the bare-ident shorthand `EXCEPT col` (single-column only). The shared dispatch can't accommodate the shorthand.

The AST already represents the result as a `FunctionExpr` with `Name = "EXCEPT"` and `Params` holding the column list. The bare-ident shorthand needs to produce the same shape, just with a one-element `ParamExprList.Items.Items` instead of building it via `parseFunctionParams`.

## Goals / Non-Goals

**Goals:**
- `SELECT … EXCEPT col …` parses successfully.
- `SELECT … EXCEPT (col1, col2, …) …` continues to parse exactly as today — no AST shape change for the existing form, byte-identical golden output.
- The bare-ident shorthand produces a `FunctionExpr` with `Name = "EXCEPT"` and a one-element `Params`, structurally identical to what the parens form `EXCEPT (col)` produces today. Downstream consumers should not be able to tell which surface syntax was used.
- The `APPLY` and `REPLACE` modifier paths are untouched.

**Non-Goals:**
- A new AST node type for the EXCEPT modifier. The existing `FunctionExpr` representation is reused.
- Multi-column bare-ident form (e.g. `EXCEPT col1 col2` or `EXCEPT col1, col2`). ClickHouse only accepts the bare form for a single column. Multi-column requires parens.
- A unified `parseModifierExpr` that handles all three modifiers. Only `EXCEPT` needs the dual shape; the new helper is named specifically.
- Changes to the formatter's rendering of EXCEPT. The formatter writes `FunctionExpr` the same way regardless of which surface syntax produced it, which is the correct behaviour.

## Decisions

### Decision 1: A dedicated `parseExceptExpr` helper, not a generalised dispatcher

The new code lives in a single function `parseExceptExpr` that handles both shapes. `parseSelectItem`'s modifier loop splits the existing combined dispatch into two arms — one for `EXCEPT` (routing to the new helper) and one for `APPLY`/`REPLACE` (continuing to use `parseFunctionExpr`).

**Why:** Keeps the special-case explicit. `APPLY` and `REPLACE` always require parens; if someone in the future tries to add a bare-ident form for one of them, they'll see at a glance that the current code doesn't support it. A generalised dispatcher would obscure the asymmetry.

**Alternative considered:** Add an optional-parens mode to `parseFunctionExpr`. **Rejected** — `parseFunctionExpr` is called from many other code paths (every function call in the language), and changing its contract risks breaking them.

### Decision 2: The bare-ident path produces the same AST shape as the parens form

Both surface syntaxes produce a `FunctionExpr` with `Name = "EXCEPT"` and `Params = *ParamExprList{Items: *ColumnExprList{Items: []Expr{...}}}`. The bare-ident path constructs the `ParamExprList` literal manually with a one-element `Items` slice; the parens path constructs it via `parseFunctionParams`. The resulting struct is structurally identical.

**Why:** Downstream code (formatter, visitor, JSON marshaller) sees a single shape regardless of input syntax. The golden test for the existing parens form already asserts that shape; the new golden for the bare-ident form will assert the same shape with one fewer element.

**Implication for the formatter:** The formatter renders the parens form for any single-column `EXCEPT (col)` regardless of how it was parsed. That means a bare-ident input `EXCEPT col` will round-trip as `EXCEPT (col)` (parenthesised). This is intentional — the parser is lenient about input, but the formatter is consistent about output. If a future need for parens-stripping round-trip emerges, that's a separate formatter change.

### Decision 3: Detection uses `matchTokenKind(TokenKindIdent)`, not a peek-and-rewind

After consuming the `EXCEPT` keyword as `name`, the helper checks `p.matchTokenKind(TokenKindIdent)`. If true, the next token is an identifier — bare-ident form. If false (it would be `(`), use the parens form via `parseFunctionParams`.

**Why:** `matchTokenKind` is non-consuming, so no rewind is needed. The choice is decided by one token of lookahead.

**Edge case:** Could the next token be something other than an ident or `(`? In principle yes (e.g. EOF, or `KeywordSomething`). Both the bare-ident path's `parseIdent` and the parens path's `parseFunctionParams` will produce a descriptive error for those cases, so this is not a hole — it just falls through to the existing error paths.

### Decision 4: Lock the new behaviour in with golden fixtures

`parser/testdata/query/select_except_bare_ident.sql` (single-line: `SELECT * EXCEPT col FROM t`) and `parser/testdata/query/select_except_mixed_modifiers.sql` (`SELECT * REPLACE(i + 1 AS i) EXCEPT colX APPLY(sum) FROM t`) are added. Each generates three goldens (output/, format/, format/beautify/) for parse + format + beautify coverage. The mixed-modifier fixture is particularly valuable because it exercises the loop-terminator logic — without it, a regression that broke the modifier loop on `EXCEPT colX APPLY` would only be caught by the inline test, not the golden suite.

**Why:** This is the established convention in the repo. Every other pattern-matching feature has golden coverage; adding it for EXCEPT keeps the codebase consistent.

**Critical workflow note:** The generated goldens MUST be visually inspected before commit. If `TestParser_Format` renders `EXCEPT col` as something nonsensical (e.g. without spaces), that is a real formatter bug — not a "regenerate and move on" situation.

## Risks / Trade-offs

- **Risk: The mixed-modifier fixture exposes a subtle loop bug where `EXCEPT colX APPLY(sum)` is consumed as `EXCEPT colX_APPLY` (one identifier).** Cannot happen — `APPLY` is a keyword, not an identifier suffix, and the lexer tokenises keywords distinctly. The loop terminates the bare-ident path immediately after consuming the single ident.
- **Risk: A user writes `SELECT * EXCEPT col1, col2` expecting both columns excluded.** Today: parse error. After this change: parse error too (the comma is not a modifier terminator and not part of a bare-ident form). The user must use the parens form. Behaviour is unchanged; this is acceptable.
- **Trade-off: Formatter round-trips bare-ident input to parens output.** Documented in Decision 2. Acceptable — the parser is lenient, the formatter is canonical.
- **Trade-off: We are NOT adding a multi-column bare form.** ClickHouse doesn't support it, so neither should we.

## Migration Plan

Single commit, no dependencies, no data or config involvement. Rollback is `git revert`.
