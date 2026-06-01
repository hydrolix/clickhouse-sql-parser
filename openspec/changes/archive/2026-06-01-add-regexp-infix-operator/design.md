## Context

`parser/parser_column.go` already wires the `LIKE`/`ILIKE` family into the parser at three coordinated touch-points:

1. **`getNextPrecedence`** (around line 70) returns `PrecedenceBetweenLike` for `KeywordBetween`, `KeywordLike`, `KeywordIlike`.
2. **`parseInfix`** has a long-form binary-operator arm (around lines 84-92) listing `TokenKind*`/`Keyword*` predicates that all flow into the same "consume, parse RHS, emit BinaryOperation" body.
3. **`parseInfix`** has a separate `KeywordNot` arm (around line 171) with a small switch over `IN`/`LIKE`/`ILIKE` that lets the parser recognise the negated forms and emit a `BinaryOperation` whose `Operation` is the verbatim `"NOT IN"` / `"NOT LIKE"` / `"NOT ILIKE"` string.

`REGEXP` (and `NOT REGEXP`) sit in exactly the same operator family. The keyword `KeywordRegexp` already exists in the constant set (referenced at line 1103 in JSON-option parsing). The work is to wire that keyword into the same three touch-points.

## Goals / Non-Goals

**Goals:**
- `expr REGEXP pattern` parses, producing a `BinaryOperation` with `Operation: TokenKind("REGEXP")`.
- `expr NOT REGEXP pattern` parses, producing a `BinaryOperation` with `Operation: TokenKind("NOT REGEXP")`.
- Precedence of `REGEXP` matches `LIKE`/`ILIKE`/`BETWEEN` — `PrecedenceBetweenLike`.
- Existing operators behave unchanged.

**Non-Goals:**
- A new AST node type for regex matching. The `BinaryOperation` + `Operation` string convention is sufficient and matches `LIKE`.
- Regex-flavoured arithmetic (e.g. `=~` shorthand). Out of scope; only the keyword form is added.
- Validation of the regex literal itself. The right-hand side is just a string expression; whether the string is a syntactically valid regex is a runtime concern, not a parse concern.
- Touching the `tryConsumeKeywords(KeywordRegexp)` call in JSON-option parsing. That code path is unrelated and continues to work.

## Decisions

### Decision 1: `REGEXP` inherits `PrecedenceBetweenLike`

The precedence ladder groups `BETWEEN`, `LIKE`, `ILIKE` together at `PrecedenceBetweenLike`. Add `REGEXP` to the same arm rather than introducing a dedicated `PrecedenceRegexp` slot.

**Why:** ClickHouse treats `REGEXP` as a peer of `LIKE`/`ILIKE` semantically (all three are pattern-matching infix operators with the same associativity and binding). Giving them the same precedence keeps the parse tree consistent across pattern operators. A query like `a LIKE 'foo' AND b REGEXP 'bar'` groups identically to `a LIKE 'foo' AND b LIKE 'bar'`.

**Alternative considered:** A dedicated precedence slot. Rejected — there is no SQL specification or downstream consumer that would benefit from `REGEXP` binding differently from `LIKE`, and a dedicated slot would force every other operator's precedence integer to shift, with unclear payoff.

### Decision 2: `NOT REGEXP` is folded into the existing `KeywordNot` switch

The current `KeywordNot` arm of `parseInfix` switches on `IN`/`LIKE`/`ILIKE` and falls through to a shared body that builds `Operation: TokenKind("NOT " + op)`. Adding a fourth case for `REGEXP` (and updating the default-arm error message) is a one-line addition that picks up the full `NOT REGEXP` plumbing for free.

**Why:** Symmetric with the existing pattern. The shared body that emits `BinaryOperation` already handles the string concatenation, so the resulting `Operation` is naturally `"NOT REGEXP"` without further code.

**Alternative considered:** A separate top-level arm for `KeywordNot` followed by `KeywordRegexp`. Rejected — that would duplicate logic that the existing switch already encapsulates cleanly.

### Decision 3: Error-message wording is updated

When the parser sees `NOT` followed by something other than `IN`/`LIKE`/`ILIKE`/`REGEXP`, the diagnostic becomes `"expected IN, LIKE, ILIKE or REGEXP after NOT, got <token-kind>"`. The old wording (`"expected IN, LIKE or ILIKE after NOT, got %s"`) is updated in place so the diagnostic stays accurate.

**Why:** A diagnostic that omits `REGEXP` from the legal-suffix list would be misleading after this change. Error strings in this repo are not part of any documented API; updating the text is safe.

### Decision 4: Lock `NOT REGEXP` in with a dedicated test case

`TestParser_REGEXP_Bare` currently has three SQLs covering bare `REGEXP`. A fourth SQL is added — e.g. `SELECT * FROM t WHERE x NOT REGEXP 'foo'` — so the negated form has explicit test coverage. Without it, the `NOT REGEXP` plumbing could silently regress.

**Why:** Test-driven completeness. The `TestParser_With_REGEXP_Operators` test only covers bare `REGEXP`; adding a negated case to the bare test gives the symmetric coverage without needing a new test function.

### Decision 5: Cover REGEXP with golden fixtures, not just inline parse-only tests

`TestParser_REGEXP_Bare` only verifies that `ParseStmts` returns no error. That catches "the parser accepts the syntax" but does NOT lock in the formatted or beautified output. Every other pattern operator in the repo (`LIKE`, `BETWEEN`, etc.) is covered by `.sql` golden fixtures under `parser/testdata/` that flow through three layers — `TestParser_ParseStatements` (JSON AST), `TestParser_Format` (compact SQL re-rendering), and `TestParser_FormatBeautify` (beautified SQL).

This change adopts the same convention for `REGEXP`. Three new fixture files are added under `parser/testdata/query/`:

- `select_regexp.sql` — `SELECT a, b FROM t WHERE name REGEXP '^foo'`. Bare `REGEXP` in a WHERE clause.
- `select_case_when_regexp.sql` — `SELECT CASE WHEN col REGEXP '^[0-9]+$' THEN toInt32(col) ELSE 0 END AS num_value FROM t`. `REGEXP` inside a `CASE WHEN` branch, which exercises the formatter's nesting behaviour.
- `select_not_regexp.sql` — `SELECT a, b FROM t WHERE name NOT REGEXP '^foo'`. The negated form.

Each fixture produces three goldens: a JSON AST snapshot under `output/`, a formatted-SQL snapshot under `format/`, and a beautified-SQL snapshot under `format/beautify/`. The goldens are generated once via `go test -update`, then committed alongside the source `.sql` files.

**Why:** Future formatter changes that subtly alter how `REGEXP` is rendered would silently slip past inline parse-only tests but would loudly fail the golden tests. The golden coverage is also the established style for this repo — leaving `REGEXP` without it would be the outlier.

**Critical workflow note:** The goldens MUST be visually inspected before being committed. If `TestParser_Format` produces something the human reader considers wrong, that is a real formatter bug — not a "regenerate and move on" situation. Tasks call this out explicitly.

## Risks / Trade-offs

- **Risk: A downstream consumer keys on the exact text of the old `"expected IN, LIKE or ILIKE after NOT"` error string.** Mitigated by the fact that error messages here are human-readable diagnostics, not stable API. The new message is strictly more accurate and adding `REGEXP` to the legal-suffix list is the only sensible change.
- **Risk: A user writes `x ~ 'pattern'` (Postgres-style regex match) expecting it to work.** That is a separate token; this change only adds keyword `REGEXP`. The user gets the same parse error they would have gotten before. Out of scope by design.
- **Trade-off: `REGEXP` shares precedence with `LIKE` rather than getting its own slot.** Acceptable — see Decision 1. If a future need for differentiated precedence emerges, splitting into a new slot is a localised follow-up.

## Migration Plan

Single commit, no dependencies, no data or config involvement. Rollback is `git revert`.
