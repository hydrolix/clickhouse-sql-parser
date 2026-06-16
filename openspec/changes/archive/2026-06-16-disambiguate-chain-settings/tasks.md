## 1. Baseline

- [x] 1.1 Add the new inline test `TestParser_With_ChainSettingsDisambiguation` to `parser/parser_test.go` (alongside the other `TestParser_With_*` helpers). Cover the five SQLs listed in the spec:
  - `SELECT 1 UNION ALL (SELECT 2 SETTINGS max_threads=1)` — inside-parens SETTINGS, attaches to inner `Settings`.
  - `SELECT 1 UNION ALL (SELECT 2) SETTINGS max_threads=1` — outside-parens SETTINGS, attaches to inner `OuterSettings`.
  - `(SELECT 1 UNION ALL SELECT 2) SETTINGS max_threads=1` — top-level wrapped chain.
  - `SELECT 1 UNION ALL (SELECT 2)` — parens preserved, no SETTINGS.
  - `SELECT 1 UNION ALL (SELECT 2 SETTINGS max_threads=1) SETTINGS max_threads=2` — both placements coexist on the same leg.
  For each, assert the expected `HasParen` / `Settings` / `OuterSettings` field values on the inner and outer `*SelectQuery`.
- [x] 1.2 Confirm the currently-failing forms FAIL today: `go test ./parser/... -run 'TestParser_With_ChainSettingsDisambiguation' -v -count=1`. Expected starting state: the two "inside parens" cases PARSE but the AST does NOT yet have `HasParen` / `OuterSettings` fields, so the test will fail at compile-time on the field reference. The three "outside parens" / "top-level wrapped" cases would parse-error too. Treat this baseline as: test exists but is non-compiling until task 2.x.
- [x] 1.3 Capture the full test baseline (excluding the new test): `go test ./parser/... -count=1 -skip TestParser_With_ChainSettingsDisambiguation 2>&1 | tee /tmp/baseline-test-output.txt`. Save passing-count summaries for `TestParser_ParseStatements`, `TestParser_Format`, `TestParser_FormatBeautify` for post-change comparison.
- [x] 1.4 Snapshot three representative pre-change JSON goldens for spot-checking:
  - `cp parser/testdata/query/output/select_expr.sql.golden.json /tmp/select_expr.before.json` — non-set-op fixture (pure two-line addition).
  - `cp parser/testdata/query/output/select_with_union_settings.sql.golden.json /tmp/select_with_union_settings.before.json` — set-op fixture with SETTINGS (gains two lines per SelectQuery rendering).
  - `cp parser/testdata/query/compatible/1_stateful/00080_array_join_and_union.sql.golden.json /tmp/00080_array_join_and_union.before.json` — FROM-subquery-wrapped UNION fixture (HasParen=false on the inner SelectQuery; format/beautify goldens stay byte-identical).
  Plus snapshot the format/beautify goldens for `00080_array_join_and_union.sql`:
  - `cp parser/testdata/query/compatible/1_stateful/format/00080_array_join_and_union.sql /tmp/00080_array_join_and_union.format.before.sql`
  - `cp parser/testdata/query/compatible/1_stateful/format/beautify/00080_array_join_and_union.sql /tmp/00080_array_join_and_union.beautify.before.sql`
  All deleted in task 7.x.
- [x] 1.5 Confirm no source consumers exist outside `parser/` that would break on the additive fields: `grep -rn -E '\bSelectQuery\b' . --include='*.go' | grep -v /parser/ | head -10`. Expected: a small number of imports, none of which pattern-match all fields exhaustively. Verify by inspection.
- [x] 1.6 Grep for all `\(\s*SELECT` occurrences in `parser/testdata/` and categorise each match's parser path (FROM subquery / scalar / CTE / set-op-leg). Expected: the only set-op-leg match is none today (the existing `00080_…` is FROM-subquery-wrapped). If any other set-op-leg paren is found, expect its format/beautify golden to change after the formatter update — flag it before implementation.

## 2. AST changes in `parser/ast.go`

- [x] 2.1 In `type SelectQuery struct { … }` (around line 5119), add two new fields immediately after the existing `Settings *SettingsClause` field (and before `Format *FormatClause`):
  ```go
  HasParen      bool
  OuterSettings *SettingsClause
  ```
- [x] 2.2 Add a one-line doc comment on each field. `HasParen`: "True when `parseSelectQuery` itself consumed the wrapping parens (vs. the parens being consumed by an enclosing `parseSubQuery`)." `OuterSettings`: "SETTINGS clause that appears AFTER the closing `)` of a paren-wrapped SelectQuery. Distinct from `Settings`, which holds SETTINGS inside the SELECT body. Non-nil only when `HasParen` is true."
- [x] 2.3 In `func (s *SelectQuery) Accept(visitor ASTVisitor) error`, locate the end of the set-op traversal blocks (`Union`, `Except`, `Intersect`) and add a new traversal block immediately after them, before `visitor.VisitSelectQuery(s)`:
  ```go
  if s.OuterSettings != nil {
      if err := s.OuterSettings.Accept(visitor); err != nil {
          return err
      }
  }
  ```
- [x] 2.4 Do not yet build — `walk.go`, `format.go`, `parser_query.go`, `parser_table.go` still need updates.

## 3. Walk update in `parser/walk.go`

- [x] 3.1 Locate the `SelectQuery` case in `Walk`. After the existing set-op `Walk` calls (`Walk(n.Union, fn)`, `Walk(n.Except, fn)`, `Walk(n.Intersect, fn)`) and after the `Walk(n.Settings, fn)` call, add:
  ```go
  if !Walk(n.OuterSettings, fn) { return false }
  ```
  matching the short-circuit shape used by the surrounding `Walk` calls.
- [x] 3.2 Do not yet build — `format.go` and the parser files still need updates.

## 4. Formatter update in `parser/format.go`

- [x] 4.1 Locate `SelectQuery.FormatSQL`. Identify the entry point of the body emission (the WITH clause / SELECT keyword emission). Wrap the entire current emission (body + set-op chain) with a conditional `(` … `)` based on `s.HasParen`:
  - Before the existing first emission (typically the WITH or SELECT keyword), add:
    ```go
    if s.HasParen {
        formatter.WriteString("(")
    }
    ```
  - After the existing last emission of the set-op chain (the end of the three `if … else if …` arms for Union / Except / Intersect), add:
    ```go
    if s.HasParen {
        formatter.WriteString(")")
        if s.OuterSettings != nil {
            formatter.WriteByte(' ')
            formatter.WriteExpr(s.OuterSettings)
        }
    }
    ```
- [x] 4.2 Verify beautified output behaves sensibly. For the beautified formatter, the `(` should precede the first beautified line and the `)` should follow the last set-op-chain line, with the `OuterSettings` appearing on its own line after the `)` (matching the existing per-SelectQuery clause indent). If the beautify path uses different helpers (e.g. `Break` calls), apply the same shape there.
- [x] 4.3 Do not yet build — the parser still doesn't populate `HasParen` / `OuterSettings`.

## 5. Parser update in `parser/parser_query.go`

- [x] 5.1 Locate `parseSelectQuery` (line 988-1030). Inside the existing `if hasParen { … }` block (currently lines 1024-1028), after the existing `expectTokenKind(TokenKindRParen)` call, add:
  ```go
  selectStmt.HasParen = true
  outerSettings, err := p.tryParseSettingsClause(p.Pos())
  if err != nil {
      return nil, err
  }
  if outerSettings != nil {
      selectStmt.OuterSettings = outerSettings
  }
  ```
- [x] 5.2 Verify `parseSubQuery` is left untouched (it must continue to consume its own outer parens before delegating to `parseSelectQuery`, so the inner `SelectQuery.HasParen` stays false in subquery contexts).
- [x] 5.3 `go build ./parser/...`. Expected: compiles.

## 6. Parser dispatch update in `parser/parser_table.go`

- [x] 6.1 Locate `parseStmt` (line 1425). Change the SELECT dispatch (line 1437) from:
  ```go
  case p.matchKeyword(KeywordSelect), p.matchKeyword(KeywordWith):
      expr, err = p.parseSelectQuery(pos)
  ```
  to:
  ```go
  case p.matchKeyword(KeywordSelect), p.matchKeyword(KeywordWith), p.matchTokenKind(TokenKindLParen):
      expr, err = p.parseSelectQuery(pos)
  ```
- [x] 6.2 `go build ./parser/...`. Expected: compiles.
- [x] 6.3 `go vet ./parser/...`. Expected: no new warnings.

## 7. Verify behavioural fix and regenerate JSON goldens

- [x] 7.1 `go test ./parser/... -run 'TestParser_With_ChainSettingsDisambiguation' -v -count=1`. Expected: all five SQLs PASS, and all field-shape assertions hold.
- [x] 7.2 `go test ./parser/... -run 'TestParser_InvalidSyntax' -v -count=1`. Expected: PASS (error-message changes are acceptable; only `require.Error` is asserted).
- [x] 7.3 `go test ./parser/... -run 'TestParser_ParseStatements' -count=1`. Expected: many failures — every SelectQuery-containing JSON golden lacks the new `HasParen` / `OuterSettings` lines.
- [x] 7.4 Regenerate JSON goldens: `go test ./parser/... -run 'TestParser_ParseStatements' -count=1 -update`.
- [x] 7.5 Sanity-check the regen scope: `git diff --stat parser/testdata | head -120`. The changed-files list should be JSON goldens only (under `**/output/*.sql.golden.json`). No `.sql` file under `parser/testdata/**/format/` or `parser/testdata/**/format/beautify/` should appear in the diff.
- [x] 7.6 Spot-check the non-set-op fixture diff: `diff /tmp/select_expr.before.json parser/testdata/query/output/select_expr.sql.golden.json`. Expected: at each `SelectQuery` rendering, exactly two added lines (`"HasParen": false,` and `"OuterSettings": null`) and no other changes.
- [x] 7.7 Spot-check the SETTINGS-in-chain fixture diff: `diff /tmp/select_with_union_settings.before.json parser/testdata/query/output/select_with_union_settings.sql.golden.json`. Expected: each of the two SelectQuery renderings (outer and inner via `Union`) gains exactly two added lines (`"HasParen": false,` and `"OuterSettings": null`) and the existing `Settings` populated subtree stays in place on the inner.
- [x] 7.8 Spot-check the FROM-subquery-wrapped UNION fixture diff: `diff /tmp/00080_array_join_and_union.before.json parser/testdata/query/compatible/1_stateful/output/00080_array_join_and_union.sql.golden.json`. Expected: each `SelectQuery` rendering (outer plus the two legs inside the FROM subquery) gains exactly two added lines (`"HasParen": false,` and `"OuterSettings": null`). The inner-leg `"HasParen"` MUST be `false` — this confirms the load-bearing invariant that `parseSubQuery`-consumed parens do NOT set the inner `SelectQuery.HasParen`.
- [x] 7.9 Confirm format and beautify goldens did NOT shift for the FROM-subquery fixture:
  - `diff /tmp/00080_array_join_and_union.format.before.sql parser/testdata/query/compatible/1_stateful/format/00080_array_join_and_union.sql`
  - `diff /tmp/00080_array_join_and_union.beautify.before.sql parser/testdata/query/compatible/1_stateful/format/beautify/00080_array_join_and_union.sql`
  Expected: empty diff for both.
- [x] 7.10 Run `go test ./parser/... -run 'TestParser_Format|TestParser_FormatBeautify' -count=1` (no `-update`). Expected: PASS — no format/beautify golden should have drifted.

## 8. Add new `.sql` fixtures and goldens for the four disambiguation forms

- [x] 8.1 Create `parser/testdata/query/select_with_paren_leg_settings_inside.sql` (single line):
  ```
  SELECT 1 UNION ALL (SELECT 2 SETTINGS max_threads = 1)
  ```
- [x] 8.2 Create `parser/testdata/query/select_with_paren_leg_settings_outside.sql` (single line):
  ```
  SELECT 1 UNION ALL (SELECT 2) SETTINGS max_threads = 1
  ```
- [x] 8.3 Create `parser/testdata/query/select_with_paren_chain_settings.sql` (single line):
  ```
  (SELECT 1 UNION ALL SELECT 2) SETTINGS max_threads = 1
  ```
- [x] 8.4 Create `parser/testdata/query/select_with_paren_leg_no_settings.sql` (single line):
  ```
  SELECT 1 UNION ALL (SELECT 2)
  ```
- [x] 8.5 Generate JSON goldens: `go test ./parser/... -run 'TestParser_ParseStatements/(select_with_paren_leg_settings_inside|select_with_paren_leg_settings_outside|select_with_paren_chain_settings|select_with_paren_leg_no_settings)\.sql$' -count=1 -update`. **Visually inspect each generated JSON**:
  - `select_with_paren_leg_settings_inside.sql.golden.json` — outer `Union` non-nil; inner `HasParen == true`, inner `Settings` non-nil, inner `OuterSettings == null`.
  - `select_with_paren_leg_settings_outside.sql.golden.json` — outer `Union` non-nil; inner `HasParen == true`, inner `Settings == null`, inner `OuterSettings` non-nil.
  - `select_with_paren_chain_settings.sql.golden.json` — outer `HasParen == true`, outer `OuterSettings` non-nil, outer `Union` non-nil; inner `HasParen == false`.
  - `select_with_paren_leg_no_settings.sql.golden.json` — outer `Union` non-nil; inner `HasParen == true`, inner `Settings == null`, inner `OuterSettings == null`.
- [x] 8.6 Confirm the two inside-vs-outside fixtures' JSON goldens are byte-distinct: `diff parser/testdata/query/output/select_with_paren_leg_settings_inside.sql.golden.json parser/testdata/query/output/select_with_paren_leg_settings_outside.sql.golden.json | head -40`. Expected: the diff shows the inner's `Settings` populated in the first and `OuterSettings` populated in the second (and vice-versa for the unpopulated fields).
- [x] 8.7 Generate format goldens: `go test ./parser/... -run 'TestParser_Format/(select_with_paren_leg_settings_inside|select_with_paren_leg_settings_outside|select_with_paren_chain_settings|select_with_paren_leg_no_settings)\.sql$' -count=1 -update`. **Visually inspect each**:
  - inside: contains `UNION ALL (SELECT 2 SETTINGS max_threads = 1)` (parens preserved around the leg, SETTINGS inside).
  - outside: contains `UNION ALL (SELECT 2) SETTINGS max_threads = 1` (parens preserved around the leg, SETTINGS after `)`).
  - chain-settings: starts with `(`, contains `UNION ALL`, contains `) SETTINGS max_threads = 1`.
  - leg-no-settings: contains `UNION ALL (SELECT 2)` (parens preserved, no SETTINGS).
- [x] 8.8 Generate beautify goldens: `go test ./parser/... -run 'TestParser_FormatBeautify/(select_with_paren_leg_settings_inside|select_with_paren_leg_settings_outside|select_with_paren_chain_settings|select_with_paren_leg_no_settings)\.sql$' -count=1 -update`. **Visually inspect each** for sensible line breaks around the parens and the `SETTINGS` clause.
- [x] 8.9 Re-run all three test suites without `-update`: `go test ./parser/... -run 'TestParser_ParseStatements|TestParser_Format|TestParser_FormatBeautify' -count=1`. All goldens (regenerated + new) must pass.
- [x] 8.10 Round-trip idempotence check: for each of the four new fixtures, parse the format-golden, re-format, and confirm the second formatting is byte-identical to the first (the canonical "fixed-point" property already exercised by `validFormatSQL`). If any input does not reach a fixed point in two iterations, the formatter has a bug — investigate before proceeding.

## 9. Close out

- [x] 9.1 `go test ./parser/... -count=1`. Compare against the baseline captured in 1.3: `TestParser_With_ChainSettingsDisambiguation` flips FAIL → PASS; ~90 pre-existing JSON goldens are regenerated (uniform +2 lines per SelectQuery rendering); four new fixtures × 3 goldens = 12 new golden sub-tests appear and PASS. Nothing previously passing moves to fail.
- [x] 9.2 `go vet ./parser/...` produces no new warnings.
- [x] 9.3 `openspec validate disambiguate-chain-settings` reports the change as valid.
- [x] 9.4 Delete the temporary snapshots from tasks 1.3/1.4: `rm /tmp/baseline-test-output.txt /tmp/select_expr.before.json /tmp/select_with_union_settings.before.json /tmp/00080_array_join_and_union.before.json /tmp/00080_array_join_and_union.format.before.sql /tmp/00080_array_join_and_union.beautify.before.sql`.
