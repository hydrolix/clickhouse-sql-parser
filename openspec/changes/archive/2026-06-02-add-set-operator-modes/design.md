## Context

`SelectQuery` in `parser/ast.go` (line ~5129) carries optional set-operation slots that recurse into another `SelectQuery`. Today there are three: `UnionAll`, `UnionDistinct`, and `Except`. The parser populates exactly one per node, and the formatter (`parser/format.go` line ~2342) renders exactly one per node via an `if … else if …` chain.

`parseSelectQuery` in `parser/parser_query.go` (line ~998) drives this. After parsing a leading `parseSelectStmt`, it inspects the next keyword:
- `UNION` followed by `ALL` → recurse, store in `UnionAll`.
- `UNION` followed by `DISTINCT` → recurse, store in `UnionDistinct`.
- `UNION` followed by anything else → **error**: `"expected ALL or DISTINCT, got <tok>"`.
- `EXCEPT` → recurse, store in `Except`. (No modifier handling — `EXCEPT ALL` and `EXCEPT DISTINCT` are rejected at the recursive call level.)
- `INTERSECT` → not handled at all; `INTERSECT` is not a recognised keyword in `parser/keyword.go`, so it would be lexed as an identifier and never reach this switch.

This change fills out the matrix: all three operators accept the bare form and the two modifiers, all three are encoded uniformly on `SelectQuery`, all three follow the same parser/formatter shape.

`parser/ast.go` line 3 already establishes the precedent for a typed string alias with an empty-string sentinel and uppercase keyword values:
```go
type OrderDirection string
const (
    OrderDirectionNone OrderDirection = ""
    OrderDirectionAsc  OrderDirection = "ASC"
    OrderDirectionDesc OrderDirection = "DESC"
)
```
`OrderByExpr.Direction` is `OrderDirection`, populated explicitly by the parser, consulted by the formatter via `if o.Direction != OrderDirectionNone { … }` (`parser/format.go:2046`). The three new mode types (`UnionMode`, `ExceptMode`, `IntersectMode`) mirror this shape exactly.

The same `SelectQuery` already handles a leading `SETTINGS` clause via `parseSelectStmt`'s call to `tryParseSettingsClause`, so each leg of a set-op chain already parses its own SETTINGS independently. This change does not touch that path; it merely needs test coverage that proves the SETTINGS + set-op combination works.

## Goals / Non-Goals

**Goals:**
- Replace the two-pointer (`UnionAll`, `UnionDistinct`) UNION representation with a single-pointer + discriminator (`Union`, `UnionMode`).
- Add a companion `ExceptMode` discriminator to the existing `Except` field, and accept `EXCEPT ALL` / `EXCEPT DISTINCT` in the parser.
- Introduce `Intersect *SelectQuery` + `IntersectMode IntersectMode` and the supporting `KeywordIntersect`, accepting `INTERSECT`, `INTERSECT ALL`, and `INTERSECT DISTINCT`.
- Round-trip every surface form through the formatter unchanged: the same SQL text comes back out with the same modifier (or its absence).
- `SETTINGS` on either or both legs of any set-op continues to parse, format, and beautify correctly. Per-leg SETTINGS is the only supported placement.
- Inline and golden test coverage for all nine surface forms and for SETTINGS-with-set-op combinations.

**Non-Goals:**
- Multi-operator precedence (mixed `UNION` + `INTERSECT` chains, mixed `UNION` + `EXCEPT` chains). The current right-recursive model can't represent these correctly per ClickHouse's precedence rules; see Decision 8.
- Validating the `*_default_mode` settings, or rejecting bare forms when no permissive setting is in scope. ClickHouse diagnoses these at runtime; the parser stays syntactically permissive.
- Adding `omitempty` JSON tags to retroactively suppress the new fields' `null`/`""` rendering. The repo's convention is explicit rendering (see Decision 5 below).
- A unified `SetOp *SetOpClause { Operator, Mode, Right }` field that replaces all three pointer-pairs with one. Rejected for this scope; see Decision 1.
- A shared `SetOpMode` type used by all three operators. Rejected for this scope; see Decision 3.

## Decisions

### Decision 1: Three pointer-pair fields, NOT a single discriminated `SetOp` field

Each operator gets its own pointer (`Union`/`Except`/`Intersect`) plus its own mode discriminator (`UnionMode`/`ExceptMode`/`IntersectMode`). The per-node invariant — "at most one set-op pointer is non-nil" — is preserved by convention (parser logic) rather than by the type system.

**Why:** Matches the existing AST style (optional pointers per concept, e.g. `Where`/`Prewhere`/`Having`, `Top`/`Limit`, `With`/`Format`). Each operator is name-addressable directly from consumer code, with no need to switch on a `.Operator` string. The visitor protocol is unchanged. The JSON dump renders each operator as its own field, which makes the AST snapshot more navigable than a single `SetOp` envelope would.

**Alternative considered:** Replace all three set-op slots with `SetOp *SetOpClause { Operator string, Mode string, Right *SelectQuery }`. **Rejected.** Operationally cleaner in some ways (one field, one traversal, one format arm), but the downstream cost is higher: every consumer that today reads `Union`/`Except` directly would need to switch on `SetOp.Operator`; the JSON dump becomes less self-describing; the migration is structural rather than additive. Adopting the three-pair shape first and revisiting consolidation later if patterns warrant it is the lower-risk move.

### Decision 2: Model each mode after `OrderDirection`

`OrderDirection` (`parser/ast.go:3-9`) is the in-repo precedent. The three new types follow it exactly:

```go
type UnionMode string
const (
    UnionModeNone     UnionMode = ""
    UnionModeAll      UnionMode = "ALL"
    UnionModeDistinct UnionMode = "DISTINCT"
)

type ExceptMode string
const (
    ExceptModeNone     ExceptMode = ""
    ExceptModeAll      ExceptMode = "ALL"
    ExceptModeDistinct ExceptMode = "DISTINCT"
)

type IntersectMode string
const (
    IntersectModeNone     IntersectMode = ""
    IntersectModeAll      IntersectMode = "ALL"
    IntersectModeDistinct IntersectMode = "DISTINCT"
)
```

**Invariant.** For each operator, when the pointer is nil, the mode is the zero value (`*ModeNone`) and is not meaningful. When the pointer is non-nil, the mode is exactly one of the three constants. The overload of `*ModeNone` between "no operator" (pointer == nil) and "operator present, no modifier" (pointer != nil, mode == None) mirrors `OrderDirectionNone`'s overload between "no direction at all" and the default direction; the companion pointer being nil is the disambiguator.

### Decision 3: Three separate mode types, NOT a single shared `SetOpMode`

Each operator gets its own typed alias even though all three have identical string values (`""`/`"ALL"`/`"DISTINCT"`).

**Why:** Matches `OrderDirection`'s scope-specific naming (it isn't called `Direction`; it's specifically for ordering). Surface area is small enough that the duplication cost is negligible — three eight-line blocks, all alphabetically grouped at the top of `parser/ast.go`. Type safety: `UnionMode("ALL")` and `ExceptMode("ALL")` aren't interchangeable at call sites, which makes accidental cross-assignment a compile error.

**Alternative considered:** A single `type SetOpMode string` with `SetOpModeNone/All/Distinct`, reused across all three operator pairs. **Rejected for this scope.** It would consolidate the eight-line declaration trio to one, save a small amount of typing, and reflect that the three modes ARE semantically identical concepts. But it sacrifices scope-specific naming (which `OrderDirection` chose explicitly) and offers no actual functional advantage — the values are constants known at compile time. If consolidation proves worthwhile later (e.g. if a helper function genuinely benefits from being mode-type-polymorphic), the rename is a trivial mechanical refactor.

### Decision 4: Parser uses a shared modifier-consumption helper

The three operator branches in `parseSelectQuery` share the same modifier-parsing logic (try `ALL`, else try `DISTINCT`, else none). A small helper avoids triplicating it:

```go
func (p *Parser) consumeOptionalSetOpModifier() string {
    switch {
    case p.tryConsumeKeywords(KeywordAll):
        return "ALL"
    case p.tryConsumeKeywords(KeywordDistinct):
        return "DISTINCT"
    default:
        return ""
    }
}
```

Each operator branch then reads the helper's string return and wraps it in its own typed alias at the assignment site:
```go
case p.tryConsumeKeywords(KeywordUnion):
    mode := UnionMode(p.consumeOptionalSetOpModifier())
    next, err := p.parseSelectQuery(p.Pos())
    if err != nil {
        return nil, err
    }
    selectStmt.Union = next
    selectStmt.UnionMode = mode
```

The string-typed helper return is the bridge between Decision 3's three separate types and the shared parsing logic.

### Decision 5: Accept the JSON-golden regen as a controlled, mechanical edit

Adding three new fields and removing two old ones changes every JSON-rendered `SelectQuery` object. Per occurrence, the diff is:
- REMOVE: `"UnionAll": null,` and `"UnionDistinct": null,` (2 lines).
- ADD: `"Union": null,`, `"UnionMode": "",`, `"ExceptMode": "",`, `"Intersect": null,`, `"IntersectMode": ""` (5 lines).
- KEEP: `"Except": null,` (or its populated counterpart) — this line was already present.

Net per occurrence: +3 lines. Across 90 fixtures with on average 1–3 `SelectQuery` renderings each, the total addition is in the low hundreds of lines.

For the four set-op-populated fixtures (`select_with_union_distinct.sql`, `select_with_multi_union.sql`, `select_with_multi_union_distinct.sql`, `select_with_multi_except.sql`), the populated subtree additionally migrates: UNION fixtures' subtree moves from `UnionAll`/`UnionDistinct` to `Union` and gains `UnionMode: "ALL"`/`"DISTINCT"`; the EXCEPT fixture's `Except` subtree stays in place and gains `ExceptMode: ""` at the populated node.

**Why accept this:** The repo convention is explicit JSON rendering of every AST field, including nils (see archived `add-describe-settings-clause` Decision 4). Adding `omitempty` would deviate from that convention and bury the discriminator fields in the JSON dump precisely when the AST snapshot is the source of truth for understanding parse output.

**Workflow:** Land the AST + parser + formatter + walk + keyword changes together — the build won't succeed until they're aligned. Then run `TestParser_ParseStatements -update` once. Then `git diff --stat` to confirm the changed-files set is exactly the ~90 JSON goldens plus the 6 new fixtures (+ 18 new goldens). Then targeted spot-checks: one UNION fixture (subtree migration), one EXCEPT fixture (ExceptMode added), one non-set-op fixture (pure rename/addition). Commit.

### Decision 6: Formatter uses three parallel arms, each with an inner switch

The chain in `SelectQuery.FormatSQL` (`parser/format.go:2342-2357`) becomes:
```go
if s.Union != nil {
    formatter.Break()
    switch s.UnionMode {
    case UnionModeAll:
        formatter.WriteString("UNION ALL")
    case UnionModeDistinct:
        formatter.WriteString("UNION DISTINCT")
    default:
        formatter.WriteString("UNION")
    }
    formatter.Break()
    formatter.WriteExpr(s.Union)
} else if s.Except != nil {
    formatter.Break()
    switch s.ExceptMode {
    case ExceptModeAll:
        formatter.WriteString("EXCEPT ALL")
    case ExceptModeDistinct:
        formatter.WriteString("EXCEPT DISTINCT")
    default:
        formatter.WriteString("EXCEPT")
    }
    formatter.Break()
    formatter.WriteExpr(s.Except)
} else if s.Intersect != nil {
    formatter.Break()
    switch s.IntersectMode {
    case IntersectModeAll:
        formatter.WriteString("INTERSECT ALL")
    case IntersectModeDistinct:
        formatter.WriteString("INTERSECT DISTINCT")
    default:
        formatter.WriteString("INTERSECT")
    }
    formatter.Break()
    formatter.WriteExpr(s.Intersect)
}
```

The three arms are visibly parallel. A more aggressive consolidation (e.g. extracting `formatSetOp(formatter, op string, mode string, right *SelectQuery)`) would reduce duplication but obscure the per-operator structure that JSON-golden reviewers will look for. The three-arm shape stays.

### Decision 7: Drop the "expected ALL or DISTINCT" error path on UNION; ensure bare EXCEPT and bare INTERSECT do not regress

Today's parser explicitly enforces that `UNION` is followed by one of two keywords. After this change, the absence of `ALL`/`DISTINCT` simply means "bare form" — assign `UnionModeNone`, recurse, store in `Union`. The same logic applies to EXCEPT and INTERSECT.

**Implication for error messages.** A SQL like `SELECT 1 UNION` followed immediately by `;` or EOF — which today errors with "expected ALL or DISTINCT, got <EOF>" — will after this change error one level deeper, inside the recursive `parseSelectQuery`, with "expected SELECT, WITH or (, got <EOF>". The error still happens; the message just changes. `TestParser_InvalidSyntax` was spot-checked: it asserts only `require.Error(t, err, …)` with no message-content pinning, so this is safe.

**Risk for EXCEPT regression.** Today's `EXCEPT` branch already accepts the bare form (which is the only form it supports). After this change, the branch accepts bare/`ALL`/`DISTINCT`. Bare EXCEPT MUST continue to parse identically — the existing `select_with_multi_except.sql` fixture is the regression guard. The new branch, structurally, is `KeywordExcept → optional modifier consumption → recurse → assign`, where "optional modifier consumption" returns `""` for bare. The behaviour for bare EXCEPT is unchanged; only the `ExceptMode` field is new (and is `ExceptModeNone == ""` for bare cases).

### Decision 8: Mixed-operator precedence is NOT addressed by this change

ClickHouse's documented set-operator precedence is:
- `INTERSECT` binds tighter than `UNION` and `EXCEPT`.
- `UNION` and `EXCEPT` have equal precedence and are evaluated left-to-right.

The current parser uses right-recursion: `a UNION b UNION c` is parsed as `SelectQuery{a, Union: SelectQuery{b, Union: SelectQuery{c}}}` — right-associative. For same-modifier `UNION ALL` chains this is harmless (the operator is associative). For mixed-operator chains the right-recursion breaks ClickHouse's semantics:

| SQL                              | ClickHouse precedence    | Right-recursion (this change) | Correct? |
| -------------------------------- | ------------------------ | ----------------------------- | -------- |
| `a UNION ALL b INTERSECT c`      | `a UNION ALL (b ∩ c)`    | `a UNION ALL (b ∩ c)`         | ✅ (by luck) |
| `a INTERSECT b UNION ALL c`      | `(a ∩ b) UNION ALL c`    | `a INTERSECT (b UNION ALL c)` | ❌       |
| `a UNION ALL b EXCEPT c`         | `(a UNION ALL b) EXCEPT c` | `a UNION ALL (b EXCEPT c)`  | ❌       |
| `a EXCEPT b UNION ALL c`         | `(a EXCEPT b) UNION ALL c` | `a EXCEPT (b UNION ALL c)`  | ❌       |

This is a pre-existing limitation (the EXCEPT/UNION mis-association already exists today). Adding INTERSECT does not fix it; in fact INTERSECT adds new wrong-precedence cases too.

**Decision: ship the additive feature work without fixing precedence.** Fixing precedence requires either:
1. A left-associative chain refactor: replace the per-`SelectQuery` pointer fields with an ordered slice of `(operator, mode, query)` entries, processed left-to-right with a per-operator precedence lookup.
2. A precedence-climbing rewrite of `parseSelectQuery`: introduce a precedence-aware loop where `INTERSECT` is parsed at higher precedence than `UNION`/`EXCEPT`.

Both are structural changes that warrant their own design and their own JSON-golden regen. Bundling either into this change would multiply the migration surface without obvious benefit — and the present (broken) right-recursion is what every existing fixture and every consumer expects, so changing it under the hood would be a more invasive break than the field rename.

**What this change DOES guarantee:** every same-operator-same-mode chain (e.g. `a UNION ALL b UNION ALL c`, `a INTERSECT b INTERSECT c`) is associatively unambiguous and round-trips correctly. Mixed-operator/mixed-mode chains MAY mis-associate per the table above.

**What it does NOT guarantee:** correctness of any expression whose meaning depends on cross-operator precedence.

This limitation MUST be called out in the proposal's "Impact" section so future readers know which cases were knowingly left for later. A follow-up change is anticipated to address precedence; its design will be informed by usage patterns observed after this change ships.

### Decision 9: SETTINGS placement — per-leg only

ClickHouse parses a trailing `SETTINGS` clause as belonging to the immediately preceding SELECT, not to the set-op as a whole. The existing `parseSelectStmt` already handles this correctly: each call parses its own optional SETTINGS before returning, and `parseSelectQuery`'s set-op recursion fires only on `UNION`/`EXCEPT`/`INTERSECT`, not on `SETTINGS`. So `SELECT 1 SETTINGS x=1 INTERSECT SELECT 2 SETTINGS y=2` produces two `SelectQuery` nodes each with their own `Settings` populated — no parser change needed for the SETTINGS path. The tests in this change verify this end-to-end for UNION and INTERSECT (EXCEPT-with-SETTINGS is symmetric and not separately fixtured).

### Decision 10: Test coverage spans 11 inline cases and 6 golden fixtures

Inline tests give a fast, readable per-form signal. Golden fixtures lock in the exact parse + format + beautify shape, which is what regression-protects round-tripping. Both layers are added:

- Inline (`TestParser_With_SetOperators`): 11 SQLs covering all 9 cells of the {UNION/EXCEPT/INTERSECT} × {bare/ALL/DISTINCT} matrix plus 2 SETTINGS combinations. Parse-only — `ParseStmts` must succeed.
- Golden fixtures (focused on the newly-unlocked surface forms, plus the SETTINGS combination the user explicitly requested):
  - `select_with_bare_union.sql` — bare UNION (newly accepted).
  - `select_with_union_settings.sql` — bare UNION with per-leg SETTINGS (the use case observability tooling cares about most).
  - `select_with_except_all.sql` — EXCEPT ALL (newly accepted).
  - `select_with_except_distinct.sql` — EXCEPT DISTINCT (newly accepted).
  - `select_with_intersect.sql` — bare INTERSECT (newly accepted; covers the new keyword path).
  - `select_with_intersect_modifiers.sql` — chained `INTERSECT ALL` + `INTERSECT DISTINCT` (covers both INTERSECT modifiers in one fixture, also locks in same-operator chaining for INTERSECT).

Existing fixtures (`select_with_union_distinct.sql`, `select_with_multi_union.sql`, `select_with_multi_union_distinct.sql`, `select_with_multi_except.sql`) continue to cover their respective forms — their JSON goldens are regenerated, their format/beautify goldens stay byte-identical.

## Risks / Trade-offs

- **Risk: breaking AST API change.** Any external consumer that pattern-matches `UnionAll` or `UnionDistinct` will fail to compile after this change. **Mitigation:** verified by grep — no consumer outside `parser/` exists in this repo. Internal call sites are migrated in lockstep within the same commit. External consumers (out-of-tree) need to be flagged in release notes.
- **Risk: the JSON regen accidentally masks an unrelated drift.** **Mitigation:** Decision 5's workflow — `-update`, then `git diff --stat` to confirm the changed-files set is exactly the expected ~90 JSON goldens + the 6 new fixtures, then targeted spot-checks on three representative goldens (UNION subtree migration, EXCEPT mode addition, non-set-op pure rename/addition).
- **Risk: error-message change for invalid SQL after `UNION`/`EXCEPT`/`INTERSECT`.** Today `SELECT 1 UNION <EOF>` errors with "expected ALL or DISTINCT"; after this change it errors with "expected SELECT, WITH or (". **Mitigation:** Decision 7 — `TestParser_InvalidSyntax` uses `require.Error` without message assertions.
- **Risk: bare `EXCEPT` regression.** Today's bare EXCEPT parses successfully; the new shape must preserve that. **Mitigation:** the existing `select_with_multi_except.sql` golden is the regression guard; the new branch is structurally a superset of the old branch's behaviour for bare-mode input.
- **Risk: shadowing of `INTERSECT` as an identifier.** Adding `KeywordIntersect = "INTERSECT"` to the keyword list makes it a reserved word; any pre-existing test or fixture that used `INTERSECT` as a column or table name would break. **Mitigation:** verified by grep (`grep -rn -i intersect parser/testdata/`) — no fixture uses `INTERSECT` as an identifier in this repo. If any future fixture needs to use it as an identifier, the fix is to backtick-quote it.
- **Trade-off: mixed-operator precedence is knowingly left broken.** See Decision 8. A follow-up change will address it.
- **Trade-off: three near-duplicate type declarations and three near-duplicate format arms.** See Decisions 3 and 6. Consolidation is reversible; over-consolidating now is not.

## Migration Plan

Single commit, no external dependencies, no data or config involvement. The keyword addition, AST refactor + additions, walk update, formatter rewrite, parser rewrite, JSON-golden regen, and new test inputs all land together — the build won't compile in any intermediate state, so staging the work in sub-commits would not be ergonomic. Rollback is `git revert`. The ~90 JSON goldens shift by a structured field-rename + three-field-addition pattern (zero-line delta on the rename, +3 lines on the addition per `SelectQuery` rendering) as part of the commit; the four set-op-populated fixtures additionally migrate or gain populated subtrees; the format/beautify goldens for pre-existing fixtures remain byte-identical; six new fixtures and their eighteen new goldens are committed alongside.

After this change ships, the anticipated follow-up is precedence correctness for mixed-operator chains (Decision 8). That follow-up's design will likely require revisiting the right-recursive pointer model itself.
