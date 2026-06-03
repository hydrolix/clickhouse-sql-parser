## Why

ClickHouse SQL supports three set operators between SELECT queries: `UNION`, `EXCEPT`, and `INTERSECT`. Each accepts an optional `ALL` or `DISTINCT` modifier; without a modifier (the "bare" form) ClickHouse resolves the semantics at execution time via the `union_default_mode` / `except_default_mode` / `intersect_default_mode` settings. The matrix is nine surface forms — three operators × three modifier modes (bare, `ALL`, `DISTINCT`).

This parser today supports five of those nine:

| Operator    | bare          | ALL                | DISTINCT           |
| ----------- | ------------- | ------------------ | ------------------ |
| `UNION`     | ❌ error      | ✅ `UnionAll`      | ✅ `UnionDistinct` |
| `EXCEPT`    | ✅ `Except`   | ❌ error           | ❌ error           |
| `INTERSECT` | ❌ unknown keyword | ❌ unknown keyword | ❌ unknown keyword |

The current AST encodes each supported form as its own optional pointer field on `SelectQuery` (`UnionAll`, `UnionDistinct`, `Except`). Filling in the remaining four forms by adding four more pointer fields would balloon the field set to seven near-identical optional pointers on a single struct — the wrong direction. The set-operator-modifier dimension is naturally a discriminator, not a pointer-per-value.

This change refactors all three operators to a uniform shape: one pointer (`Union` / `Except` / `Intersect`) for the right-hand side, paired with a typed discriminator (`UnionMode` / `ExceptMode` / `IntersectMode`) for the modifier. The shape mirrors the existing `OrderDirection` precedent in `parser/ast.go:3-9` (typed string alias, empty-string sentinel for "absent", uppercase keyword values for the explicit cases). The parser, walker, and formatter are migrated together, and the four missing surface forms are unlocked along the way.

## What Changes

**Refactor (AST-shape change):**
- Three new typed aliases are added to `parser/ast.go` alongside `OrderDirection`:
  - `type UnionMode string` with `UnionModeNone = ""`, `UnionModeAll = "ALL"`, `UnionModeDistinct = "DISTINCT"`.
  - `type ExceptMode string` with `ExceptModeNone = ""`, `ExceptModeAll = "ALL"`, `ExceptModeDistinct = "DISTINCT"`.
  - `type IntersectMode string` with `IntersectModeNone = ""`, `IntersectModeAll = "ALL"`, `IntersectModeDistinct = "DISTINCT"`.
- The `SelectQuery` struct (`parser/ast.go` ~line 5129):
  - **REMOVES** the existing fields `UnionAll *SelectQuery` and `UnionDistinct *SelectQuery`.
  - **ADDS** `Union *SelectQuery` and `UnionMode UnionMode` (in the same struct-position the removed fields occupied).
  - **KEEPS** the existing `Except *SelectQuery` field, and **ADDS** a companion `ExceptMode ExceptMode` immediately after it.
  - **ADDS** two new fields at the end of the set-op block: `Intersect *SelectQuery` and `IntersectMode IntersectMode`.
- Per-node invariant (unchanged in spirit): at most one of `Union`, `Except`, `Intersect` is non-nil. When a pointer is nil, its companion `*Mode` is the zero value and not meaningful. When a pointer is non-nil, its companion is one of the three constants.
- `SelectQuery.Accept()` and `Walk()` collapse the prior pair of UNION traversal blocks into one, keep one EXCEPT traversal block (mode-agnostic — the recursion target is unchanged), and gain one new INTERSECT traversal block.
- `SelectQuery.FormatSQL()` in `parser/format.go` becomes a three-arm chain (`if s.Union != nil { … } else if s.Except != nil { … } else if s.Intersect != nil { … }`), each arm using an inner `switch` on the mode discriminator to choose the emitted keyword sequence (`UNION` / `UNION ALL` / `UNION DISTINCT`, analogous for the other two).

**New keyword:**
- `KeywordIntersect = "INTERSECT"` is added to `parser/keyword.go` in both the constant block and the keyword slice (alphabetically between `Interpolate` and `Into`).

**Parser feature changes in `parser/parser_query.go`:**
- The `parseSelectQuery` switch is rewritten so that each of `UNION` / `EXCEPT` / `INTERSECT` follows the same shape:
  1. Consume the operator keyword.
  2. Optionally consume `ALL` or `DISTINCT`; default to the `*ModeNone` sentinel.
  3. Recurse via `parseSelectQuery` into the right-hand side.
  4. Store the result and the mode on the parent `SelectQuery`.
- The existing "expected ALL or DISTINCT" error on bare `UNION` is gone; the bare form is now accepted for all three operators.

**Out of scope:**
- Multi-operator precedence (e.g. `a UNION ALL b INTERSECT c` per ClickHouse's "INTERSECT binds tighter than UNION/EXCEPT" rule, or `a EXCEPT b UNION c` per left-to-right evaluation). The right-recursive pointer chain inherited from today's parser already mis-associates mixed-operator chains; this change does NOT fix that. It is a separate structural concern that requires either a left-associative chain refactor or a precedence-climbing rewrite of `parseSelectQuery`. See Decision 8 in `design.md`.
- Adding `omitempty` JSON tags. The repo's convention is explicit rendering (see the archived `add-describe-settings-clause` change's Decision 4).
- A new visitor method. `VisitSelectQuery` remains the only hook; recursion into UNION/EXCEPT/INTERSECT right-hand sides invokes it on the child.

## Capabilities

### New Capabilities
- `set-operator-modes`: Parse and format the full nine-cell matrix of `{UNION, EXCEPT, INTERSECT} × {bare, ALL, DISTINCT}` between SELECT queries. Each operator is represented on `SelectQuery` as one optional pointer to the right-hand side plus a typed mode discriminator.

### Modified Capabilities
<!--
No previously-extracted capability spec describes the prior UNION/EXCEPT shape, so there is no existing spec to mark as MODIFIED here. The AST API change (removal of UnionAll/UnionDistinct in favour of Union+UnionMode, addition of ExceptMode, addition of Intersect+IntersectMode) is documented in detail in the new `set-operator-modes` spec's "ADDED Requirements" block.
-->

## Impact

- **Code touched**: three new typed-alias declarations (with constants) at the top of `parser/ast.go`; field rename + additions on `SelectQuery`; collapsed/extended `Accept` block in `parser/ast.go`; collapsed/extended `Walk` block in `parser/walk.go`; rewritten set-op chain in `parser/format.go`; rewritten `parseSelectQuery` set-op switch in `parser/parser_query.go`; one new `KeywordIntersect` constant + slice entry in `parser/keyword.go`. **Breaking AST API change**: consumers that pattern-match `UnionAll` or `UnionDistinct` must migrate.
- **Internal call sites that must migrate (verified by grep across `parser/`)**: `parser/ast.go` (Accept), `parser/walk.go` (Walk), `parser/format.go` (FormatSQL), `parser/parser_query.go` (parseSelectQuery). No test file references either removed field directly. No file outside `parser/` references them.
- **JSON-golden footprint**: every JSON golden under `parser/testdata/**/output/*.sql.golden.json` that today renders `"UnionAll": null` (90 files) will change. Per `SelectQuery` rendering, the diff is:
  - REMOVE: `"UnionAll": null,` and `"UnionDistinct": null,` (2 lines).
  - ADD: `"Union": null,`, `"UnionMode": "",`, `"ExceptMode": "",`, `"Intersect": null,`, `"IntersectMode": ""` (5 lines).
  - The pre-existing `"Except": null,` line stays.
  Net delta: +3 lines per `SelectQuery` rendering. The four UNION/EXCEPT-using fixtures (`select_with_union_distinct.sql`, `select_with_multi_union.sql`, `select_with_multi_union_distinct.sql`, `select_with_multi_except.sql`) additionally have their populated subtree migrate to the new field name (UNION fixtures) or pick up a `"ExceptMode": ""` line at the populated EXCEPT node.
- **Format-golden footprint**: `parser/testdata/**/format/**` and `parser/testdata/**/format/beautify/**` goldens MUST remain byte-identical for every existing fixture. The formatter still emits exactly `UNION ALL`, `UNION DISTINCT`, and `EXCEPT` for the corresponding modes; only the source-of-truth fields have moved.
- **New inline test in `parser/parser_test.go`** — `TestParser_With_SetOperators` — exercises at minimum 11 SQLs spanning the full 3×3 matrix plus three SETTINGS combinations:
  - `SELECT 1 UNION SELECT 2`, `SELECT 1 UNION ALL SELECT 2`, `SELECT 1 UNION DISTINCT SELECT 2`
  - `SELECT 1 EXCEPT SELECT 2`, `SELECT 1 EXCEPT ALL SELECT 2`, `SELECT 1 EXCEPT DISTINCT SELECT 2`
  - `SELECT 1 INTERSECT SELECT 2`, `SELECT 1 INTERSECT ALL SELECT 2`, `SELECT 1 INTERSECT DISTINCT SELECT 2`
  - `SELECT 1 SETTINGS max_threads=1 UNION SELECT 2 SETTINGS max_threads=2`
  - `SELECT 1 INTERSECT ALL SELECT 2 SETTINGS max_threads=2`
  Five of these currently FAIL (the four formerly-unsupported surface forms plus the bare-UNION-with-SETTINGS combo); after this change all 11 PASS.
- **New `.sql` golden fixtures** under `parser/testdata/query/`, focused on the newly-unlocked surface forms. Six fixtures × 3 goldens each = 6 inputs + 18 new goldens:
  - `select_with_bare_union.sql` — `SELECT 1 AS v UNION SELECT 2 AS v`.
  - `select_with_union_settings.sql` — `SELECT 1 AS v SETTINGS max_threads = 1 UNION SELECT 2 AS v SETTINGS max_threads = 2`.
  - `select_with_except_all.sql` — `SELECT 1 AS v EXCEPT ALL SELECT 2 AS v`.
  - `select_with_except_distinct.sql` — `SELECT 1 AS v EXCEPT DISTINCT SELECT 2 AS v`.
  - `select_with_intersect.sql` — `SELECT 1 AS v INTERSECT SELECT 2 AS v`.
  - `select_with_intersect_modifiers.sql` — `SELECT 1 AS v INTERSECT ALL SELECT 2 AS v INTERSECT DISTINCT SELECT 3 AS v` (covers chained INTERSECT with both modifiers in one fixture).
- **No dependencies** added, no runtime semantics changed for SQL that previously parsed.
- **Behavioural compatibility with `*_default_mode` settings**: the parser stays syntactically permissive — it accepts the bare form regardless of any setting. ClickHouse rejects bare forms at execution time when the corresponding default-mode setting is empty; this change does not attempt to mirror that runtime behaviour. The parser's job is shape recognition, not semantic validation.
- **Known limitation — mixed-operator precedence is NOT fixed by this change**: the existing right-recursive pointer chain mis-associates mixed-operator chains by ClickHouse's precedence rules (INTERSECT binds tighter than UNION/EXCEPT; UNION/EXCEPT are left-to-right at equal precedence). After this change, `a UNION ALL b INTERSECT c` parses as `a UNION ALL (b INTERSECT c)` (correct by luck of right-recursion matching INTERSECT's higher precedence), but `a INTERSECT b UNION ALL c` parses as `a INTERSECT (b UNION ALL c)` (wrong — should be `(a INTERSECT b) UNION ALL c`). See `design.md` Decision 8.
