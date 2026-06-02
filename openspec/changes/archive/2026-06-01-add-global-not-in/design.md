## Context

`parseInfix` in `parser/parser_column.go` has an arm at line ~139 that runs when the lookahead is the `GLOBAL` keyword:

```go
case p.matchKeyword(KeywordGlobal):
    _ = p.lexer.consumeToken()
    if p.expectKeyword(KeywordIn) != nil {
        return nil, fmt.Errorf("expected IN after GLOBAL, got %s", p.lastTokenKind())
    }
    rightExpr, err := p.parseSubExpr(p.Pos(), precedence)
    if err != nil {
        return nil, err
    }
    return &BinaryOperation{
        LeftExpr:  expr,
        Operation: "GLOBAL IN",
        RightExpr: rightExpr,
    }, nil
```

The pattern of packing a multi-keyword operator into the `Operation` string is the established convention in this file. Two other arms do exactly the same:

- The `KeywordNot` arm at line ~171 produces `Operation: TokenKind("NOT " + op)` for `NOT IN`, `NOT LIKE`, `NOT ILIKE`.
- The `KeywordGlobal` arm above produces `Operation: "GLOBAL IN"`.

`BinaryOperation` also has two boolean fields `HasGlobal` and `HasNot`. They are set by no current parser arm. They are read by the **formatter** in `parser/format.go` (lines 120-174), where they drive a beautify-mode-only logical-operand layout. Importantly, the formatter treats them as mutually exclusive: `if p.HasNot { … } else if p.HasGlobal { … }`. Setting both would not produce `GLOBAL NOT`; it would print only `NOT`.

So for `GLOBAL NOT IN`, the only consistent option is to keep using the `Operation` string and write `"GLOBAL NOT IN"` verbatim — exactly as the convention for `NOT IN` and `GLOBAL IN` already prescribes.

## Goals / Non-Goals

**Goals:**
- `GLOBAL NOT IN <subquery>` parses without error and produces a `BinaryOperation` with `Operation: "GLOBAL NOT IN"`.
- `GLOBAL IN <subquery>` continues to parse exactly as today, producing `Operation: "GLOBAL IN"`. No regression.
- `GLOBAL <anything-else>` produces a parser error with the updated diagnostic `"expected IN or NOT IN after GLOBAL, got <token-kind>"`.

**Non-Goals:**
- A new AST node type for distributed-IN. The string-packed convention is good enough and matches what the rest of the file does.
- Setting the `HasGlobal` / `HasNot` boolean fields. They belong to a separate beautify-mode code path, are mutually exclusive there, and would not represent `GLOBAL NOT` correctly. Out of scope.
- Touching the `KeywordNot` arm. That arm already handles bare `NOT IN`; it has no `GLOBAL` context to consider.
- Any change to the lexer or to the precedence ladder.

## Decisions

### Decision 1: Pack `"GLOBAL NOT IN"` into the `Operation` string

The implementation builds the operation string conditionally based on whether `NOT` is present after `GLOBAL`, then emits a single `BinaryOperation` with that string:

```go
case p.matchKeyword(KeywordGlobal):
    _ = p.lexer.consumeToken()
    op := "GLOBAL IN"
    if p.matchKeyword(KeywordNot) {
        _ = p.lexer.consumeToken()
        op = "GLOBAL NOT IN"
    }
    if p.expectKeyword(KeywordIn) != nil {
        return nil, fmt.Errorf("expected IN or NOT IN after GLOBAL, got %s", p.lastTokenKind())
    }
    rightExpr, err := p.parseSubExpr(p.Pos(), precedence)
    if err != nil {
        return nil, err
    }
    return &BinaryOperation{
        LeftExpr:  expr,
        Operation: TokenKind(op),
        RightExpr: rightExpr,
    }, nil
```

**Why:** This is the **same shape** every other multi-keyword operator in this file uses. A future reader who recognises the `"NOT IN"` and `"GLOBAL IN"` patterns will immediately understand `"GLOBAL NOT IN"`. Using the existing `HasGlobal` / `HasNot` booleans instead would introduce a third pattern for what is structurally the same case, and the booleans don't support both being true at once.

**Alternative considered:** Set `HasGlobal = true` and `HasNot = true` on the AST node. **Rejected** — the formatter treats those fields as mutually exclusive (`if HasNot { … } else if HasGlobal { … }` on lines 150-153 and 164-167 of `parser/format.go`), so the node would format as bare `NOT IN` and lose the `GLOBAL` qualifier. Fixing the formatter to support both would be a much larger, riskier change for no functional benefit.

**Alternative considered:** A dedicated `GlobalNotInExpr` AST node. **Rejected** — no other negated IN-family operator has a dedicated node; consistency wins.

### Decision 2: Consume the optional `NOT` with `matchKeyword` + `consumeToken`, not `expectKeyword`

`expectKeyword` returns an error if the keyword isn't there. We do NOT want an error when `NOT` is absent — `GLOBAL IN` without `NOT` is the existing, valid form. So the check uses `matchKeyword` (boolean predicate) and conditional `consumeToken`.

**Why:** Mirrors the way other optional keywords are handled elsewhere in this file. Keeps the success/failure flow obvious at the call site.

### Decision 3: The error message changes from `"expected IN after GLOBAL"` to `"expected IN or NOT IN after GLOBAL"`

Because both `IN` and `NOT IN` are now legal, the diagnostic must reflect both options. The wording change is intentional and visible to anyone parsing error messages programmatically.

**Why this is a low-risk message change:** Error messages in this repo are not part of any documented API surface, and grepping for the old wording outside the test suite found no consumers. If a downstream tool keyed off the exact text, it should be updated; that's a one-line change on their side.

## Risks / Trade-offs

- **Risk: A downstream tool greps for `"expected IN after GLOBAL"` to classify lexer/parser errors.** Mitigated by the fact that error strings here are conventionally treated as human-readable diagnostics, not stable API. The new message is strictly more accurate.
- **Risk: A user writes `GLOBAL NOT FOO` expecting `FOO` to be some new keyword.** Today this errors with `"expected IN after GLOBAL"`; after this change it errors with `"expected IN or NOT IN after GLOBAL"`. Behaviour is unchanged — it just errors with a more helpful message. The `NOT` token is consumed before the error fires, so the error position advances by one token, which is acceptable.
- **Trade-off: We are NOT setting `HasGlobal` / `HasNot`.** Anyone scanning the AST who already inspects `BinaryOperation.Operation` for `"GLOBAL IN"` / `"NOT IN"` will naturally find `"GLOBAL NOT IN"` and the consistency holds. Anyone scanning the boolean fields would need to inspect `Operation` instead — but no parser arm currently sets those booleans, so that population is empty.

## Migration Plan

Single commit, no dependencies, no data or config involvement. Rollback is `git revert`.
