## 1. Baseline

- [x] 1.1 Add the new inline test `TestParser_With_SetOperators` to `parser/parser_test.go` (alongside the other `TestParser_With_*` helpers). Cover 11 SQLs spanning the full 3×3 matrix and the SETTINGS combinations:
  - `SELECT 1 UNION SELECT 2`
  - `SELECT 1 UNION ALL SELECT 2`
  - `SELECT 1 UNION DISTINCT SELECT 2`
  - `SELECT 1 EXCEPT SELECT 2`
  - `SELECT 1 EXCEPT ALL SELECT 2`
  - `SELECT 1 EXCEPT DISTINCT SELECT 2`
  - `SELECT 1 INTERSECT SELECT 2`
  - `SELECT 1 INTERSECT ALL SELECT 2`
  - `SELECT 1 INTERSECT DISTINCT SELECT 2`
  - `SELECT 1 SETTINGS max_threads=1 UNION SELECT 2 SETTINGS max_threads=2`
  - `SELECT 1 INTERSECT ALL SELECT 2 SETTINGS max_threads=2`
- [x] 1.2 Confirm the currently-unsupported cases (bare UNION, EXCEPT ALL/DISTINCT, all three INTERSECT forms, the bare-UNION + SETTINGS combo) FAIL: `go test ./parser/... -run 'TestParser_With_SetOperators' -v -count=1`. Expected starting state: 5 SQLs PASS (UNION ALL/DISTINCT, bare EXCEPT, INTERSECT-ALL-with-trailing-SETTINGS errors at INTERSECT, the 11th case…) — wait, INTERSECT isn't yet a keyword so those will fail at the lexer-or-after-SELECT stage too. Treat the expected starting state as: only UNION ALL, UNION DISTINCT, and bare EXCEPT pass. Everything else FAILs.
- [x] 1.3 Confirm `INTERSECT` is not yet a recognised keyword: `grep -nE 'KeywordIntersect|"INTERSECT"' parser/keyword.go`. Expected: no matches.
- [x] 1.4 Confirm no current fixture uses `INTERSECT` as an identifier (would conflict with the new keyword): `grep -rn -iE '\bintersect\b' parser/testdata/ | grep -v _binary`. Expected: no fixture references INTERSECT (it has not been a keyword and isn't expected to appear).
- [x] 1.5 Capture the full test baseline: `go test ./parser/... -count=1 2>&1 | tee /tmp/baseline-test-output.txt`. Save passing-count summaries for `TestParser_ParseStatements`, `TestParser_Format`, `TestParser_FormatBeautify` for post-change comparison.
- [x] 1.6 Snapshot three representative pre-change JSON goldens for spot-checking:
  - `cp parser/testdata/query/output/select_with_union_distinct.sql.golden.json /tmp/select_with_union_distinct.before.json` — UNION fixture (subtree migration).
  - `cp parser/testdata/query/output/select_with_multi_except.sql.golden.json /tmp/select_with_multi_except.before.json` — EXCEPT fixture (gains `ExceptMode`).
  - `cp parser/testdata/query/output/select_expr.sql.golden.json /tmp/select_expr.before.json` — non-set-op fixture (pure rename + addition).
  All three deleted in task 9.x.
- [x] 1.7 Confirm no source consumers exist outside `parser/`: `grep -rn -E 'UnionAll|UnionDistinct' . --include='*.go' | grep -v /parser/`. Expected: empty. If anything appears, STOP and add explicit migration tasks for those call sites.

## 2. Keyword addition in `parser/keyword.go`

- [x] 2.1 Add `KeywordIntersect = "INTERSECT"` to the constant declaration block, alphabetically between `KeywordInterpolate` and `KeywordInto` (around lines 115–116).
- [x] 2.2 Add `KeywordIntersect,` to the keyword slice, alphabetically between `KeywordInterpolate` and `KeywordInto` (around lines 374–375).
- [x] 2.3 `go build ./parser/...`. Expected: compiles.

## 3. AST changes in `parser/ast.go`

- [x] 3.1 Immediately after the existing `OrderDirection` block (after line 9), add three new typed aliases with their constants. Place all three in one grouped block for readability:
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
- [x] 3.2 In `type SelectQuery struct { … }` (around line 5129):
  - **REMOVE** the fields `UnionAll *SelectQuery` and `UnionDistinct *SelectQuery`.
  - **ADD** in their place (same struct position, between `Format` and `Except`):
    ```go
    Union     *SelectQuery
    UnionMode UnionMode
    ```
  - **ADD** immediately after the existing `Except *SelectQuery` field:
    ```go
    ExceptMode ExceptMode
    Intersect     *SelectQuery
    IntersectMode IntersectMode
    ```
  Final block order: `Union`, `UnionMode`, `Except`, `ExceptMode`, `Intersect`, `IntersectMode`.
- [x] 3.3 In `func (s *SelectQuery) Accept(visitor ASTVisitor) error` (around line 5162):
  - **REMOVE** the two traversal blocks for `s.UnionAll` and `s.UnionDistinct` (around lines 5237–5246).
  - **REPLACE** with a single block:
    ```go
    if s.Union != nil {
        if err := s.Union.Accept(visitor); err != nil {
            return err
        }
    }
    ```
    Position: same as the removed blocks (between the existing `Format` traversal and the existing `Except` traversal).
  - The existing `s.Except` traversal block is kept verbatim.
  - **ADD** after the existing `s.Except` traversal block:
    ```go
    if s.Intersect != nil {
        if err := s.Intersect.Accept(visitor); err != nil {
            return err
        }
    }
    ```
- [x] 3.4 Do not yet build — `walk.go`, `format.go`, `parser_query.go` still reference the old names.

## 4. Walk update in `parser/walk.go`

- [x] 4.1 Locate the `SelectQuery` case in `Walk` (around lines 66–69).
  - **REMOVE** the `Walk(n.UnionAll, fn)` and `Walk(n.UnionDistinct, fn)` calls.
  - **REPLACE** with a single `if !Walk(n.Union, fn) { return false }` (matching the existing local helper-call shape).
  - Preserve the existing `Walk(n.Except, fn)` call verbatim, AND **ADD** a new `if !Walk(n.Intersect, fn) { return false }` immediately after it.
- [x] 4.2 Do not yet build — `format.go` and `parser_query.go` still reference the old fields.

## 5. Formatter update in `parser/format.go`

- [x] 5.1 Locate the set-op chain in `SelectQuery.FormatSQL` (around lines 2342–2357). Replace the entire chain (the `if s.UnionAll != nil` / `else if s.UnionDistinct != nil` / `else if s.Except != nil` block) with three parallel arms, one per operator, each using an inner `switch` on the mode:
  ```go
  if s.Union != nil {
      formatter.Break()
      switch s.UnionMode {
      case UnionModeAll:
          formatter.WriteString("UNION ALL")
      case UnionModeDistinct:
          formatter.WriteString("UNION DISTINCT")
      default: // UnionModeNone — bare
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
      default: // ExceptModeNone — bare
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
      default: // IntersectModeNone — bare
          formatter.WriteString("INTERSECT")
      }
      formatter.Break()
      formatter.WriteExpr(s.Intersect)
  }
  ```
- [x] 5.2 Do not yet build — `parser_query.go` still references the old fields.

## 6. Parser update in `parser/parser_query.go`

- [x] 6.1 Add a small private helper near the top of the file (or alongside `parseSelectQuery`):
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
- [x] 6.2 Locate the `switch` block inside `parseSelectQuery` (around line 1008–1032). Rewrite the entire UNION arm AND the EXCEPT arm AND add a new INTERSECT arm so all three follow the same shape:
  ```go
  case p.tryConsumeKeywords(KeywordUnion):
      mode := UnionMode(p.consumeOptionalSetOpModifier())
      next, err := p.parseSelectQuery(p.Pos())
      if err != nil {
          return nil, err
      }
      selectStmt.Union = next
      selectStmt.UnionMode = mode
  case p.tryConsumeKeywords(KeywordExcept):
      mode := ExceptMode(p.consumeOptionalSetOpModifier())
      next, err := p.parseSelectQuery(p.Pos())
      if err != nil {
          return nil, err
      }
      selectStmt.Except = next
      selectStmt.ExceptMode = mode
  case p.tryConsumeKeywords(KeywordIntersect):
      mode := IntersectMode(p.consumeOptionalSetOpModifier())
      next, err := p.parseSelectQuery(p.Pos())
      if err != nil {
          return nil, err
      }
      selectStmt.Intersect = next
      selectStmt.IntersectMode = mode
  ```
  The previous `default: return nil, fmt.Errorf("expected ALL or DISTINCT, ...")` under the UNION arm is gone — its absence is the bare-UNION acceptance.
- [x] 6.3 `go build ./parser/...`. Expected: compiles. If references to `UnionAll`/`UnionDistinct` remain anywhere, the build will surface them — fix and re-run.
- [x] 6.4 `go vet ./parser/...`. Expected: no new warnings (the pre-existing `WriteByte` notice is acceptable).

## 7. Verify behavioural fix and regenerate JSON goldens

- [x] 7.1 `go test ./parser/... -run 'TestParser_With_SetOperators' -v -count=1`. Expected: all 11 cases now PASS.
- [x] 7.2 `go test ./parser/... -run 'TestParser_InvalidSyntax' -v -count=1`. Expected: PASS.
- [x] 7.3 `go test ./parser/... -run 'TestParser_ParseStatements' -count=1`. Expected: many failures — every SelectQuery-containing JSON golden references the old field shape.
- [x] 7.4 Regenerate: `go test ./parser/... -run 'TestParser_ParseStatements' -count=1 -update`.
- [x] 7.5 Sanity-check the regen scope: `git diff --stat parser/testdata | head -120`. The changed-files list should be JSON goldens only (under `**/output/*.sql.golden.json`). No `.sql` file under `parser/testdata/**/format/` or `parser/testdata/**/format/beautify/` should appear in the diff. The count of changed goldens should be approximately 90.
- [x] 7.6 Spot-check the UNION fixture diff: `diff /tmp/select_with_union_distinct.before.json parser/testdata/query/output/select_with_union_distinct.sql.golden.json`. Expected: the populated `UnionDistinct: { … }` subtree is now `Union: { … }` (same subtree contents at the new field name with the same recursive rename), `"UnionAll": null,` is gone, `"UnionDistinct": null,` is gone (or transformed at the inner SelectQuery), `"UnionMode": "DISTINCT"` is present at the outer SelectQuery, `"ExceptMode": ""` lines and `"Intersect": null, "IntersectMode": ""` lines appear at every SelectQuery rendering. No other field movement.
- [x] 7.7 Spot-check the EXCEPT fixture diff: `diff /tmp/select_with_multi_except.before.json parser/testdata/query/output/select_with_multi_except.sql.golden.json`. Expected: the populated `"Except": { … }` subtree stays at the same field, and each SelectQuery rendering gains `"UnionMode": ""`, `"ExceptMode": ""`, `"Intersect": null`, `"IntersectMode": ""` lines; `"UnionAll": null,` and `"UnionDistinct": null,` lines are removed and replaced by `"Union": null,` and `"UnionMode": "",`. No other field movement.
- [x] 7.8 Spot-check the non-set-op fixture diff: `diff /tmp/select_expr.before.json parser/testdata/query/output/select_expr.sql.golden.json`. Expected: at each SelectQuery rendering, exactly two lines removed (`"UnionAll": null,` and `"UnionDistinct": null,`) and exactly five lines added (`"Union": null,`, `"UnionMode": "",`, `"ExceptMode": "",`, `"Intersect": null,`, `"IntersectMode": ""`). Net delta +3 lines per rendering. No positional movement of other fields.
- [x] 7.9 Confirm format and beautify goldens did NOT shift for the four existing set-op fixtures: `git diff parser/testdata/query/format/select_with_union_distinct.sql parser/testdata/query/format/beautify/select_with_union_distinct.sql parser/testdata/query/format/select_with_multi_union.sql parser/testdata/query/format/beautify/select_with_multi_union.sql parser/testdata/query/format/select_with_multi_union_distinct.sql parser/testdata/query/format/beautify/select_with_multi_union_distinct.sql parser/testdata/query/format/select_with_multi_except.sql parser/testdata/query/format/beautify/select_with_multi_except.sql`. Expected: empty diff for all eight files.
- [x] 7.10 Run `go test ./parser/... -run 'TestParser_Format|TestParser_FormatBeautify' -count=1`. Expected: PASS — no format/beautify golden should have drifted.

## 8. Add new `.sql` fixtures and goldens for the newly-unlocked surface forms

- [x] 8.1 Create `parser/testdata/query/select_with_bare_union.sql` (single line):
  ```
  SELECT 1 AS v UNION SELECT 2 AS v
  ```
- [x] 8.2 Create `parser/testdata/query/select_with_union_settings.sql` (single line):
  ```
  SELECT 1 AS v SETTINGS max_threads = 1 UNION SELECT 2 AS v SETTINGS max_threads = 2
  ```
- [x] 8.3 Create `parser/testdata/query/select_with_except_all.sql` (single line):
  ```
  SELECT 1 AS v EXCEPT ALL SELECT 2 AS v
  ```
- [x] 8.4 Create `parser/testdata/query/select_with_except_distinct.sql` (single line):
  ```
  SELECT 1 AS v EXCEPT DISTINCT SELECT 2 AS v
  ```
- [x] 8.5 Create `parser/testdata/query/select_with_intersect.sql` (single line):
  ```
  SELECT 1 AS v INTERSECT SELECT 2 AS v
  ```
- [x] 8.6 Create `parser/testdata/query/select_with_intersect_modifiers.sql` (single line):
  ```
  SELECT 1 AS v INTERSECT ALL SELECT 2 AS v INTERSECT DISTINCT SELECT 3 AS v
  ```
- [x] 8.7 Generate JSON goldens for all six new fixtures: `go test ./parser/... -run 'TestParser_ParseStatements/(select_with_bare_union|select_with_union_settings|select_with_except_all|select_with_except_distinct|select_with_intersect|select_with_intersect_modifiers)\.sql$' -count=1 -update`. **Visually inspect each generated JSON**:
  - `select_with_bare_union.sql.golden.json` — outer `Union` non-nil, `UnionMode == ""`, `Except` nil, `Intersect` nil.
  - `select_with_union_settings.sql.golden.json` — both outer and inner SelectQuery have `Settings` non-nil; outer `Union` non-nil; `UnionMode == ""`.
  - `select_with_except_all.sql.golden.json` — outer `Except` non-nil, `ExceptMode == "ALL"`.
  - `select_with_except_distinct.sql.golden.json` — outer `Except` non-nil, `ExceptMode == "DISTINCT"`.
  - `select_with_intersect.sql.golden.json` — outer `Intersect` non-nil, `IntersectMode == ""`, `Union`/`Except` nil.
  - `select_with_intersect_modifiers.sql.golden.json` — outer `Intersect` non-nil with `IntersectMode == "ALL"`, the inner SelectQuery (at `outer.Intersect`) has `Intersect` non-nil with `IntersectMode == "DISTINCT"`.
- [x] 8.8 Generate format goldens: `go test ./parser/... -run 'TestParser_Format/(select_with_bare_union|select_with_union_settings|select_with_except_all|select_with_except_distinct|select_with_intersect|select_with_intersect_modifiers)\.sql$' -count=1 -update`. **Visually inspect each**:
  - bare-UNION format: exactly the token sequence `UNION` between the SELECTs — not `UNION ALL`, not `UNION DISTINCT`.
  - SETTINGS+UNION format: preserves `SETTINGS max_threads = N` on both legs.
  - EXCEPT ALL/DISTINCT: emits exactly `EXCEPT ALL` / `EXCEPT DISTINCT`.
  - bare-INTERSECT: emits exactly `INTERSECT` (no modifier).
  - chained-INTERSECT-modifiers: emits `INTERSECT ALL` then `INTERSECT DISTINCT` in order.
- [x] 8.9 Generate beautify goldens: `go test ./parser/... -run 'TestParser_FormatBeautify/(select_with_bare_union|select_with_union_settings|select_with_except_all|select_with_except_distinct|select_with_intersect|select_with_intersect_modifiers)\.sql$' -count=1 -update`. **Visually inspect** each beautified file for the same properties (each operator+modifier on its own line per the formatter's `Break` calls).
- [x] 8.10 Re-run all three without `-update`: `go test ./parser/... -run 'TestParser_ParseStatements|TestParser_Format|TestParser_FormatBeautify' -count=1`. All goldens (regenerated + new) must pass.

## 9. Close out

- [x] 9.1 `go test ./parser/... -count=1`. Compare against the baseline captured in 1.5: `TestParser_With_SetOperators` flips FAIL → PASS for all formerly-unsupported cases; ~90 pre-existing JSON goldens are regenerated (per-occurrence: two-line removal + five-line addition + one populated-subtree migration for set-op fixtures); six new fixtures × 3 goldens = 18 new golden sub-tests appear and PASS. Nothing previously passing moves to fail.
- [x] 9.2 `go vet ./parser/...` produces no new warnings.
- [x] 9.3 `openspec validate add-set-operator-modes` reports the change as valid.
- [x] 9.4 Delete the temporary snapshots from tasks 1.5/1.6: `rm /tmp/baseline-test-output.txt /tmp/select_with_union_distinct.before.json /tmp/select_with_multi_except.before.json /tmp/select_expr.before.json`.
