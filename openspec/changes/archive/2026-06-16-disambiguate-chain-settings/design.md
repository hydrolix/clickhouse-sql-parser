## Context

`SelectQuery` in `parser/ast.go` (around line 5119) is the AST node for a SELECT statement and the head of any UNION / EXCEPT / INTERSECT chain rooted on it. The set-op fields (`Union *SelectQuery`, `Except *SelectQuery`, `Intersect *SelectQuery` and their mode discriminators) were introduced by the prior `add-set-operator-modes` change; per-leg `SETTINGS` already works through `parseSelectStmt`'s `tryParseSettingsClause` call at `parser/parser_query.go:1151`. Per-leg SETTINGS lands on the leg's own `Settings` field.

`parseSelectQuery` (`parser/parser_query.go:988-1030`) is the recursive entry point for any `SelectQuery`. Its flow:

1. Optionally consume `(` (line 993 — `hasParen := …`).
2. `parseSelectStmt` — parses one SELECT body including its in-clause-list `SETTINGS`.
3. Set-op switch (lines 998–1023): consumes `UNION` / `EXCEPT` / `INTERSECT` (with optional `ALL`/`DISTINCT`), recurses into the RHS, stores it in the corresponding pointer + mode pair.
4. If `hasParen` was true, expects `)` (lines 1024–1028).
5. Returns the SelectQuery.

`parseSubQuery` (`parser/parser_query.go:957-975`) is the parallel entry point for SELECTs used as subexpressions (FROM clauses, scalar subqueries, view bodies). It consumes its own optional `(` *before* delegating to `parseSelectQuery`, then expects its own `)` after. The two paren-consuming sites — `parseSubQuery:959` and `parseSelectQuery:993` — are mutually exclusive: when a SELECT appears as a subexpression, `parseSubQuery` consumes the parens and the inner `parseSelectQuery` sees only the keyword. When a SELECT appears as a set-op leg (no `SubQuery` wrapper), the parens go through `parseSelectQuery` itself.

`parseStmt` (`parser/parser_table.go:1425-1482`) is the top-level statement dispatcher. Line 1437 routes `KeywordSelect` and `KeywordWith` to `parseSelectQuery`, but does **not** match `TokenKindLParen` — so a top-level `(SELECT …)` falls into the default-case error "unexpected token: `(`".

`SubQuery` already carries `HasParen bool` (`parser/ast.go:5106`-ish): the in-repo precedent for parens-as-AST-state on a SELECT-wrapping node.

`Format` for `SelectQuery` (in `parser/format.go`, the three set-op arms around line ~2342 after the `add-set-operator-modes` change) emits the body, then the set-op chain. It currently does not emit parens around any `SelectQuery` (parens are lost on round-trip when consumed by `parseSelectQuery` itself).

## Goals / Non-Goals

**Goals:**
- Distinguish in the AST between SETTINGS inside vs outside the closing `)` of a parenthesised set-op leg, so the four cases enumerated in the proposal each have distinct, round-trippable AST shapes.
- Accept `(SELECT … UNION … SELECT …) SETTINGS …` as a top-level statement — today it fails at the leading `(`.
- Accept `SELECT 1 UNION ALL (SELECT 2) SETTINGS …` — today the trailing SETTINGS errors at the statement boundary.
- Preserve byte-identical format and beautify goldens for every existing fixture (no `.sql` input currently produces a `parseSelectQuery`-consumed `(`, so no existing fixture's printed output should shift).
- Make the new fields additive on `SelectQuery`: existing consumers compile unchanged; JSON goldens gain exactly two new lines per `SelectQuery` rendering with no field renames or reorderings.

**Non-Goals:**
- Retroactively reinterpreting `SELECT 1 UNION ALL SELECT 2 SETTINGS x=1` (no parens) as chain-level SETTINGS. ClickHouse's runtime may treat it as such, but this parser keeps the existing per-leg attachment and uses parens as the explicit disambiguator.
- Fixing the broader mixed-operator precedence question (INTERSECT-tighter-than-UNION, left-associativity for UNION/EXCEPT). That is `add-set-operator-modes`'s Decision 8 and is anticipated as a separate future change.
- Generalising paren tracking to every SELECT context. `SubQuery` already tracks its own parens; this change does NOT touch `SubQuery`. `WITH` CTEs, `INSERT … SELECT`, view bodies, and other SELECT-bearing constructs that go through `parseSubQuery` are unaffected.
- Adding `omitempty` JSON tags to suppress the new `"HasParen": false` / `"OuterSettings": null` lines on every SelectQuery. The repo convention is explicit rendering — see the archived `add-describe-settings-clause` Decision 4.

## Decisions

### Decision 1: Two additive fields on `SelectQuery`, not a wrapper node

`SelectQuery` gains exactly two new fields: `HasParen bool` and `OuterSettings *SettingsClause`. No new AST type, no wrapper node, no visitor method.

**Why:** Mirrors `SubQuery.HasParen` (in-repo precedent for paren-as-state on a SELECT-bearing node). Adding fields is strictly additive — every existing consumer that reflects on `SelectQuery` continues to compile; JSON goldens gain two new lines per rendering with no positional movement. A wrapper node (e.g. `ParenSelectQuery { Inner *SelectQuery; OuterSettings *SettingsClause }`) would force every consumer that handles `SelectQuery` to also handle the wrapper, and would force visitor changes — a much larger blast radius for a feature with a narrow surface area.

**Alternative considered:** A dedicated `ParenSelectQuery` AST node carrying `Inner *SelectQuery` and `OuterSettings *SettingsClause`. **Rejected.** Cleaner type model on paper (the new state is on a dedicated node rather than overloaded on `SelectQuery`), but the downstream cost is high: every callsite that handles `SelectQuery` directly (FROM subqueries, CTEs, INSERT-SELECT, view bodies) would need to also handle the wrapper, or unwrap it at the boundary; the visitor protocol would need a new `VisitParenSelectQuery`; the JSON shape changes in a way that complicates diffs against pre-change goldens. The two-field additive shape is strictly less invasive for the same expressive power.

### Decision 2: Name `HasParen` (mirror `SubQuery.HasParen`), not `WrappedInParens` / `OpenParen`

The existing `SubQuery` carries the boolean `HasParen`. Reusing that exact name on `SelectQuery` keeps the convention uniform — a reader who already understands `SubQuery.HasParen` understands `SelectQuery.HasParen` immediately.

**Alternative considered:** `WrappedInParens` (more descriptive) or `OpenParen` (terser, mirrors a token name). **Rejected** in favour of the in-repo precedent. A descriptive doc comment on the field carries the intent without requiring a name divergence.

### Decision 3: `parseSubQuery` consumes parens *before* `parseSelectQuery` is called — so the inner `SelectQuery.HasParen` stays false in subquery contexts

This is the load-bearing invariant for the format/beautify-golden stability claim. `parseSubQuery` (`parser/parser_query.go:957-975`) consumes its optional `(` at line 959 *before* calling `parseSelectQuery` at line 961. So when a SELECT appears as a FROM subquery, scalar subquery, INSERT-SELECT body, view body, or any other `SubQuery`-wrapped position, the `(` is consumed by the wrapper and `parseSelectQuery` sees `SELECT`/`WITH` next — its `hasParen` is `false`, the new `SelectQuery.HasParen` stays `false`, and the formatter emits no extra parens for this node. The `SubQuery` wrapper still emits its own parens via `SubQuery.HasParen` (unchanged from today).

When a SELECT appears as a set-op leg (`SELECT 1 UNION ALL (SELECT 2)`), the set-op recursion at `parser/parser_query.go:1001` calls `parseSelectQuery` directly without a `parseSubQuery` wrapper. The `(` is consumed by `parseSelectQuery` itself at line 993, `HasParen` is set to `true`, and the formatter emits the parens for this leg.

**Implication for existing fixtures:** the only `.sql` fixture in the repo whose input contains `(SELECT … UNION … SELECT …)` is `parser/testdata/query/compatible/1_stateful/00080_array_join_and_union.sql` — `SELECT count() FROM (SELECT … UNION ALL SELECT …);`. Here the outer parens belong to the FROM-clause subquery (`parseSubQuery`-consumed), so the inner `SelectQuery.HasParen` stays false. Verified by grep + manual inspection; this is the regression guard.

### Decision 4: Trailing SETTINGS is consumed *after* the matching `)`, inside the `if hasParen` block

The new parser logic in `parseSelectQuery` lives strictly *inside* the existing `if hasParen { … }` block (currently lines 1024–1028), immediately after the `expectTokenKind(TokenKindRParen)`. Shape:

```go
if hasParen {
    if err := p.expectTokenKind(TokenKindRParen); err != nil {
        return nil, err
    }
    selectStmt.HasParen = true
    outerSettings, err := p.tryParseSettingsClause(p.Pos())
    if err != nil {
        return nil, err
    }
    if outerSettings != nil {
        selectStmt.OuterSettings = outerSettings
    }
}
```

**Why this placement, not after the function or outside the `if`:**
- Gating on `hasParen` enforces the per-node invariant: `OuterSettings` non-nil implies `HasParen` true. A nil-HasParen-but-non-nil-OuterSettings AST state is unreachable.
- `tryParseSettingsClause` is exactly the helper used inside `parseSelectStmt` for the in-body `SETTINGS`, so the parse behaviour is symmetric — same lexer, same comma handling, same key=value grammar.
- The set-op recursion at the inner level has already returned by this point. The inner leg has finished parsing (`)` consumed). The trailing SETTINGS naturally belongs to the *current* `parseSelectQuery` frame, not to the inner.

**Where the SETTINGS lands semantically.** In `SELECT 1 UNION ALL (SELECT 2) SETTINGS x=1`:
- Outer `parseSelectQuery` enters with `hasParen=false`, parses SELECT 1 (no SETTINGS), consumes UNION ALL, recurses.
- Inner `parseSelectQuery` enters with `hasParen=true`, parses SELECT 2 (no SETTINGS), no set-op, consumes `)`, parses trailing SETTINGS into `inner.OuterSettings`, sets `inner.HasParen=true`, returns.
- Outer assigns `outer.Union = inner`, outer has `hasParen=false` so the new block does not fire, returns.

So `OuterSettings` lands on the *inner* `SelectQuery` — the one whose parens were consumed. Semantically a consumer interprets this as "trailing SETTINGS for the parens-wrapped expression rooted at this node, i.e. for the leg as wrapped". The leg-shape SETTINGS reads naturally as "applied to this paren-bounded sub-expression", which is what the SQL text actually expresses.

**Alternative considered:** bubble the trailing SETTINGS up to the chain head (`outer.OuterSettings` in the example above). **Rejected for this scope.** Bubbling would more closely match a "chain-level" interpretation, but it requires a second post-parse pass over the chain, complicates the field's semantics (it would no longer be a local property of the parens-bounded node), and gains nothing for round-trip correctness — the formatter emits SETTINGS where the AST node sits, and the AST node sits where the SQL text places it. The local-placement rule is what the SQL text shape supports.

### Decision 5: `parseStmt` accepts `TokenKindLParen` as a SELECT dispatch path

`parser/parser_table.go:1437` is extended from:
```go
case p.matchKeyword(KeywordSelect), p.matchKeyword(KeywordWith):
    expr, err = p.parseSelectQuery(pos)
```
to:
```go
case p.matchKeyword(KeywordSelect), p.matchKeyword(KeywordWith), p.matchTokenKind(TokenKindLParen):
    expr, err = p.parseSelectQuery(pos)
```

**Why:** Without this, `(SELECT 1 UNION ALL SELECT 2) SETTINGS x=1` errors at the leading `(` with "unexpected token: `(`" (verified by probe). `parseSelectQuery` is already capable of consuming a leading `(` (line 989's match check, line 993's consumption); the dispatcher just needs to route `(`-starting input there. There is no other top-level statement that legitimately starts with `(`, so the dispatch is unambiguous.

**Side effect.** A top-level bare `(SELECT 1)` (no chain, no SETTINGS) — which is rejected today — will parse after this change. The result is a `SelectQuery` with `HasParen=true`, `OuterSettings=nil`, no chain. The formatter will emit `(SELECT 1)`. This is intentional: the parser is now able to represent parens-wrapped statements end-to-end. No existing fixture starts with `(`, so no golden drift.

### Decision 6: Formatter emits parens around the *entire* SelectQuery (body + chain), then OuterSettings after `)`

`SelectQuery.FormatSQL` is restructured so that when `s.HasParen == true`:
1. Emit `(`.
2. Emit the SELECT body and any set-op chain (the existing logic, unchanged in shape).
3. Emit `)`.
4. If `s.OuterSettings != nil`, emit it after the `)`.

When `s.HasParen == false`, the format path is unchanged — no parens, no `OuterSettings` consideration (the per-node invariant guarantees `OuterSettings` is nil in this case).

**Why "around the entire SelectQuery, not just the body":** This matches what `parseSelectQuery` actually parsed. The opening `(` was consumed before the body, and the closing `)` was expected after the set-op recursion completed. So for `(SELECT 1 UNION ALL SELECT 2) SETTINGS x=1`:
- Outer `parseSelectQuery` enters with `hasParen=true`, parses SELECT 1, consumes UNION ALL, recurses for SELECT 2 (inner.hasParen=false), assigns `outer.Union = inner`, consumes `)`, parses trailing SETTINGS into `outer.OuterSettings`, sets `outer.HasParen=true`.
- Formatter emits: `(` + body of outer + ` UNION ALL ` + body of inner + `)` + ` SETTINGS …`. → `(SELECT 1 UNION ALL SELECT 2) SETTINGS x=1`. ✓

**Beautified output.** The beautified formatter places set-op keywords on their own line. The opening `(` precedes the first line of the outer's body; the closing `)` follows the last line of the inner's body; the SETTINGS lands on its own line after `)` (matching the existing beautified shape for SETTINGS inside a SelectQuery body). The three new beautify goldens lock this in.

### Decision 7: `Accept` and `Walk` traverse `OuterSettings` after the set-op block

`(*SelectQuery).Accept` is extended to call `s.OuterSettings.Accept(visitor)` (gated on non-nil) *after* the set-op traversal blocks (the existing `Union` / `Except` / `Intersect` traversal blocks from `add-set-operator-modes`) and *before* `visitor.VisitSelectQuery(s)`. The traversal order mirrors lexical order: body → set-op chain → trailing SETTINGS → outer visitor call.

`Walk`'s `SelectQuery` case in `parser/walk.go` gains a parallel `if !Walk(n.OuterSettings, fn) { return false }` immediately after the existing `Walk(n.Settings, fn)` call (or, more precisely, immediately after the set-op `Walk` calls — wherever the existing `Settings` walk sits in the function, the new one sits one step further into the lexical order).

**Why this order:** Visitors that consume the AST top-down expect the chain to be traversed before any post-chain trailing clauses. Putting `OuterSettings` after the set-op blocks matches the "trailing clause" mental model.

### Decision 8: Golden regeneration footprint — explicit across all three golden families

This is the regen contract for code review. Three families of golden files exist under `parser/testdata/`: JSON ASTs (`**/output/*.sql.golden.json`), compact-formatted SQL (`**/format/*.sql`), and beautified SQL (`**/format/beautify/*.sql`). The expected diff against each family after this change is precisely characterised below.

**JSON family — mechanical two-line addition per SelectQuery rendering.** Every pre-existing JSON golden whose AST contains a `SelectQuery` gains exactly two added lines at each `SelectQuery` rendering: `"HasParen": false,` and `"OuterSettings": null`. No line is removed; no positional movement of any other field. Approximate scope: ~90 SelectQuery-containing fixtures with 1–3 renderings each — total addition in the low hundreds of lines.

**Format family — byte-identical for every pre-existing fixture.** The formatter only emits `(...)` and trailing `OuterSettings` when `HasParen == true`. By D3, every pre-existing fixture's parser-path produces `HasParen == false` (parens that appear today are consumed by `parseSubQuery`, not by `parseSelectQuery` itself). So no existing format golden shifts.

**Beautify family — byte-identical for every pre-existing fixture.** Same reasoning as the format family. The beautify path threads the same writer and the new branch is gated on the same `HasParen` flag.

**New fixtures.** Four new `.sql` inputs under `parser/testdata/query/` each carry three goldens (`output/`, `format/`, `format/beautify/`) → 4 inputs + 12 new goldens. The JSON goldens render populated `"HasParen": true` and either `Settings`-or-`OuterSettings`-populated subtrees at the affected nodes.

**Regression guard for the byte-identical claim.** `parser/testdata/query/compatible/1_stateful/00080_array_join_and_union.sql` is the canonical existing fixture whose input contains `(SELECT … UNION … SELECT …)` — and crucially those parens are FROM-subquery-wrapped (`parseSubQuery`-consumed), so the inner `SelectQuery.HasParen` stays `false` post-change. Its format and beautify goldens MUST match byte-for-byte; any drift indicates D3's parser-path invariant has broken.

**Workflow.** Land the AST + parser + formatter + walk changes together (partial state fails to compile or fails round-trip). Then:

1. `go test ./parser/... -run 'TestParser_ParseStatements' -count=1 -update` to regenerate JSON goldens.
2. `git diff --stat parser/testdata` — expected: only files under `**/output/*.sql.golden.json` change, no `.sql` under `**/format/` or `**/format/beautify/` changes.
3. Spot-check three goldens: one no-parens fixture (pure two-line addition), one set-op fixture with SETTINGS (two lines per leg), the FROM-subquery `00080_…` fixture (the regression guard; inner-leg `"HasParen"` must be `false`).
4. `go test ./parser/... -run 'TestParser_Format|TestParser_FormatBeautify' -count=1` *without* `-update`. This MUST pass — no `-update` is the proof the byte-identical claim holds.
5. Add the four new fixtures and generate their three goldens each, then re-run all three suites without `-update`.

### Decision 9: Per-node invariant is enforced by the parser, not the type system

Go's type system can't natively express "`OuterSettings` non-nil ⇒ `HasParen` true". The invariant is enforced operationally: the only code path that writes `OuterSettings` is the new block inside `if hasParen { … }` in `parseSelectQuery`, which also writes `HasParen = true` on the same SelectQuery. Manual construction outside the parser (e.g. in tests) could in principle violate the invariant; this is acceptable — the parser is the only normative producer of `SelectQuery` instances in this codebase, and downstream consumers reading the AST should not need to handle the violated state.

**Why not a constructor or validator:** Overkill for two fields. The existing AST uses bare struct types throughout (`SelectQuery`, `SubQuery`, etc.) — adding a constructor for just these two would diverge from convention without commensurate benefit.

### Decision 10: No interaction with `set-operator-modes` spec — it stays unchanged

The set-operator-modes spec (introduced in `add-set-operator-modes`) covers Union/Except/Intersect parsing, modes, formatter shape, and walk/accept traversal. Its scenarios that mention SETTINGS (e.g. "Bare UNION combined with per-leg SETTINGS", "INTERSECT ALL with trailing SETTINGS on the right leg") are about *no-parens* per-leg SETTINGS attached to the inner SelectQuery's `Settings` field. Those scenarios remain byte-for-byte true after this change — the new fields are additive and don't displace `Settings`. So `set-operator-modes` is not modified; the new behaviour lives entirely in the new `paren-wrapped-select-query` spec.

## Risks / Trade-offs

- **Risk: an existing fixture's input contains parens that turn out to be `parseSelectQuery`-consumed, not `parseSubQuery`-consumed, and its format golden drifts.** *Mitigation:* Decision 3 is the load-bearing invariant. Before implementation, grep for `\(\s*SELECT` across `parser/testdata/` and inspect each match's parser path (FROM subquery, scalar, CTE, INSERT-SELECT all go through `parseSubQuery`; only direct set-op-leg parens go through `parseSelectQuery`). The known case is `parser/testdata/query/compatible/1_stateful/00080_array_join_and_union.sql`, which is FROM-clause-wrapped and safe. If any other case surfaces, it's either (a) already a set-op-leg case (in which case the format change is *correct* — the parens were silently dropped before and should now be preserved) or (b) reveals a parser-path assumption to investigate before merging.
- **Risk: `parseStmt` accepting `TokenKindLParen` shadows some other statement form that starts with `(`.** *Mitigation:* No other statement form starts with `(` — DDL, INSERT, USE, SET, SETTINGS, SYSTEM, OPTIMIZE, CHECK, EXPLAIN, GRANT, SHOW, DESC, SELECT, WITH all start with their respective keywords. The `(` dispatch is unambiguous. Verified by inspection of `parser/parser_table.go:1428-1467`.
- **Risk: `parseSubQuery`-vs-`parseSelectQuery` paren-consumption boundary is subtle and easy to break by refactoring.** *Mitigation:* the spec scenario "SubQuery wraps SelectQuery and the inner's HasParen stays false" is the regression test; any future refactor that violates the invariant will break the corresponding existing fixture's format golden, surfacing the drift.
- **Risk: visitor traversal order change for the new `OuterSettings` node could surprise consumers.** *Mitigation:* the order chosen (set-op traversal → OuterSettings → visitor call) matches lexical order; no existing visitor depends on the new node (it didn't exist before); documented in the spec.
- **Trade-off: parens-tracking is now state on `SelectQuery` itself, not on a wrapper.** Future structural cleanups (e.g. introducing a chain-AST type per `add-set-operator-modes` Decision 8) inherit `HasParen` as a SelectQuery property and must migrate it. Acceptable — that future change is already known to require an AST shape rework; carrying `HasParen` along is a known small detail.

## Migration Plan

Single commit, no external dependencies. AST field additions, parser changes, formatter changes, walk update, and the new `parseStmt` dispatch all land together — partial state would either fail to compile or fail tests (the new fields are populated by the parser but not yet emitted by the formatter would cause round-trip mismatches). The ~90 SelectQuery-containing JSON goldens are regenerated in the same commit (uniform two-line addition per rendering); four new `.sql` fixtures and twelve new goldens (3 per fixture × 4 fixtures) are committed alongside. Format and beautify goldens for every existing fixture remain byte-identical — this is the visible regression guard.

Rollback is `git revert`. No data or config involvement; no runtime semantics changed for SQL that previously parsed.

After this change ships, the four new fixtures plus the new inline test serve as the executable specification of the disambiguation. Any future change touching `parseSelectQuery`'s paren-consumption path or the formatter's `SelectQuery` arm must keep these passing.

## Open Questions

None blocking. The semantics of `OuterSettings` on a non-head leg ("trailing SETTINGS for the parens-wrapped sub-expression rooted at this node") is a local property well-defined by the parser flow; downstream consumers that need "chain-level SETTINGS" can read `OuterSettings` off whichever node was the top of their chain. If a future need emerges to bubble OuterSettings to the chain head as a structural property of the chain, it can be added as a separate change without invalidating this one.
