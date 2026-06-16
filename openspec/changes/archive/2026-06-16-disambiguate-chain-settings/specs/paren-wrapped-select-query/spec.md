## ADDED Requirements

### Requirement: `SelectQuery` SHALL expose `HasParen` and `OuterSettings` on its exported AST surface

The `SelectQuery` struct SHALL expose two additional optional fields:

- `HasParen bool` — true when the parser consumed wrapping parens around this `SelectQuery` itself (as opposed to parens consumed by a surrounding `SubQuery` wrapper).
- `OuterSettings *SettingsClause` — the optional `SETTINGS` clause that appears immediately after the closing `)` of a paren-wrapped `SelectQuery`. Distinct from the existing `Settings` field, which continues to mean "SETTINGS inside the SELECT body's clause list".

The placement of the new fields in the struct SHALL fall between the existing `Settings` field and the existing `Format` field, so the JSON-golden diff per `SelectQuery` rendering is two contiguous added lines (see D8 in design.md).

#### Scenario: Exported AST surface gains the two new fields
- **WHEN** a Go consumer reflects on `parser.SelectQuery` after this change
- **THEN** the struct has fields `HasParen bool` and `OuterSettings *SettingsClause` in addition to every pre-existing field, with no pre-existing field renamed, removed, or reordered

#### Scenario: Default zero values for any SelectQuery without parser-consumed parens
- **WHEN** any SQL that does not include parens directly around a `SelectQuery` is parsed (e.g. `SELECT 1`, `SELECT 1 UNION ALL SELECT 2`, `SELECT * FROM (SELECT 1)`)
- **THEN** every resulting `*SelectQuery` has `HasParen == false` AND `OuterSettings == nil`

### Requirement: Parsing a parenthesised `SelectQuery` SHALL set `HasParen` on that node

When the parser consumes a matching `(` … `)` pair around a `SelectQuery` (and any set-op chain rooted on it), the resulting `*SelectQuery` SHALL have `HasParen == true`. When the parens belong to a surrounding `SubQuery` wrapper (FROM-clause subquery, scalar subquery, view body, INSERT-SELECT body, etc.), the inner `*SelectQuery.HasParen` SHALL stay `false` and the surrounding `SubQuery.HasParen` SHALL be `true` (its pre-existing behaviour, unchanged).

#### Scenario: Paren-wrapped set-op leg has HasParen true
- **WHEN** `SELECT 1 UNION ALL (SELECT 2)` is parsed
- **THEN** `ParseStmts` returns no error AND the outer `*SelectQuery.Union` is non-nil AND `outer.Union.HasParen == true` AND `outer.HasParen == false`

#### Scenario: Top-level paren-wrapped chain has HasParen true on the head
- **WHEN** `(SELECT 1 UNION ALL SELECT 2)` is parsed
- **THEN** `ParseStmts` returns no error AND the resulting `*SelectQuery` has `HasParen == true` AND its `Union` is non-nil AND `outer.Union.HasParen == false`

#### Scenario: SubQuery-wrapped SELECT keeps the inner HasParen false
- **WHEN** `SELECT * FROM (SELECT 1 UNION ALL SELECT 2)` is parsed
- **THEN** `ParseStmts` returns no error AND the FROM-clause `*SubQuery` has its own `HasParen == true` AND the wrapped `*SelectQuery` (`SubQuery.Select`) has `HasParen == false`

### Requirement: Trailing `SETTINGS` after `)` SHALL land in `OuterSettings`

When a `SETTINGS` clause appears immediately after the closing `)` of a paren-wrapped `SelectQuery`, the parser SHALL attach it to that node's `OuterSettings` field. SETTINGS appearing *before* the `)` (inside the parens, as part of the SELECT body) continues to attach to the node's `Settings` field — unchanged from today.

#### Scenario: SETTINGS inside parens attaches to the leg's `Settings`
- **WHEN** `SELECT 1 UNION ALL (SELECT 2 SETTINGS max_threads=1)` is parsed
- **THEN** `outer.Union.HasParen == true` AND `outer.Union.Settings` is non-nil AND `outer.Union.OuterSettings == nil`

#### Scenario: SETTINGS outside parens attaches to the leg's `OuterSettings`
- **WHEN** `SELECT 1 UNION ALL (SELECT 2) SETTINGS max_threads=1` is parsed
- **THEN** `outer.Union.HasParen == true` AND `outer.Union.Settings == nil` AND `outer.Union.OuterSettings` is non-nil

#### Scenario: Both SETTINGS placements coexist on the same paren-wrapped leg
- **WHEN** `SELECT 1 UNION ALL (SELECT 2 SETTINGS max_threads=1) SETTINGS max_threads=2` is parsed
- **THEN** `outer.Union.HasParen == true` AND `outer.Union.Settings` is non-nil (the inner `max_threads=1`) AND `outer.Union.OuterSettings` is non-nil (the trailing `max_threads=2`)

#### Scenario: Trailing SETTINGS on a top-level wrapped chain attaches to the head's `OuterSettings`
- **WHEN** `(SELECT 1 UNION ALL SELECT 2) SETTINGS max_threads=1` is parsed
- **THEN** the outer `*SelectQuery.HasParen == true` AND `outer.OuterSettings` is non-nil AND `outer.Union` is non-nil AND `outer.Union.HasParen == false` AND `outer.Union.OuterSettings == nil`

#### Scenario: No-parens trailing SETTINGS continues to attach to the inner `Settings`
- **WHEN** `SELECT 1 UNION ALL SELECT 2 SETTINGS max_threads=1` is parsed
- **THEN** `outer.Union` is non-nil AND `outer.Union.Settings` is non-nil AND `outer.Union.HasParen == false` AND `outer.Union.OuterSettings == nil` AND `outer.Settings == nil`

### Requirement: `OuterSettings` non-nil SHALL imply `HasParen` true (per-node invariant)

For every `*SelectQuery` produced by `ParseStmts`, the invariant `OuterSettings != nil ⇒ HasParen == true` SHALL hold. Equivalently: a node with `HasParen == false` MUST have `OuterSettings == nil`.

#### Scenario: Invariant holds across all four new fixtures
- **WHEN** each of `select_with_paren_leg_settings_inside.sql`, `select_with_paren_leg_settings_outside.sql`, `select_with_paren_chain_settings.sql`, `select_with_paren_leg_no_settings.sql` is parsed
- **THEN** every `*SelectQuery` node reachable from the result satisfies `OuterSettings == nil OR HasParen == true`

### Requirement: Top-level paren-wrapped SELECT statements SHALL parse

`ParseStmts` SHALL accept SQL whose first non-whitespace token is `(` and whose remainder forms a parenthesised `SelectQuery` (with or without a set-op chain inside, with or without a trailing `SETTINGS` clause).

#### Scenario: Top-level paren-wrapped chain with trailing SETTINGS parses
- **WHEN** `(SELECT 1 UNION ALL SELECT 2) SETTINGS max_threads=1` is parsed by `ParseStmts`
- **THEN** no error is returned AND the resulting statement is a `*SelectQuery` with `HasParen == true` AND `OuterSettings` non-nil

#### Scenario: Bare top-level paren-wrapped SELECT parses
- **WHEN** `(SELECT 1)` is parsed by `ParseStmts`
- **THEN** no error is returned AND the resulting statement is a `*SelectQuery` with `HasParen == true` AND `OuterSettings == nil` AND `Settings == nil` AND no set-op pointer populated

### Requirement: `Format` SHALL preserve parens and emit `OuterSettings`

`Format(stmt)` over a `*SelectQuery` SHALL:

- Emit `(` before the SELECT body and `)` after the (optional) set-op chain when `HasParen == true`.
- Emit the `OuterSettings` clause after the closing `)` when `OuterSettings != nil`.
- Emit no parens and no `OuterSettings` when `HasParen == false` (the invariant guarantees `OuterSettings == nil` in that case).

The existing emission of an inner `Settings` clause (when non-nil) SHALL be unchanged in all cases.

#### Scenario: Paren-wrapped leg with inside-SETTINGS round-trips with parens preserved
- **WHEN** `SELECT 1 UNION ALL (SELECT 2 SETTINGS max_threads = 1)` is parsed and re-formatted
- **THEN** the formatted output contains `UNION ALL (SELECT 2 SETTINGS max_threads = 1)` AND does NOT collapse to `UNION ALL SELECT 2 SETTINGS max_threads = 1`

#### Scenario: Paren-wrapped leg with outside-SETTINGS round-trips with parens preserved and SETTINGS after `)`
- **WHEN** `SELECT 1 UNION ALL (SELECT 2) SETTINGS max_threads = 1` is parsed and re-formatted
- **THEN** the formatted output contains `UNION ALL (SELECT 2) SETTINGS max_threads = 1`

#### Scenario: Top-level wrapped chain with trailing SETTINGS round-trips identically
- **WHEN** `(SELECT 1 UNION ALL SELECT 2) SETTINGS max_threads = 1` is parsed and re-formatted
- **THEN** the formatted output starts with `(` AND contains `UNION ALL` AND ends with `) SETTINGS max_threads = 1`

#### Scenario: Round-trip is a fixed point for every new fixture
- **WHEN** each of the four new `.sql` fixtures is parsed, formatted, re-parsed, and re-formatted
- **THEN** the second formatting result is byte-identical to the first (the canonical `validFormatSQL` property)

### Requirement: `Accept` and `Walk` SHALL traverse `OuterSettings`

`(*SelectQuery).Accept(visitor)` and `Walk(node, fn)` SHALL traverse the `OuterSettings` subtree (when non-nil) immediately after the set-op chain (`Union`, `Except`, `Intersect`) is traversed and before the outer node's visit callback fires. The lexical order — body → set-op chain → trailing SETTINGS → outer node — SHALL be preserved.

#### Scenario: Visitor and walker see OuterSettings when present
- **WHEN** an `ASTVisitor` traverses a `*SelectQuery` whose `OuterSettings` is non-nil, or `Walk(outer, fn)` is invoked on it
- **THEN** at least one visit / `fn` invocation hits the `OuterSettings` subtree before the outer `*SelectQuery` itself is visited

#### Scenario: Traversal is unchanged when OuterSettings is nil
- **WHEN** an `ASTVisitor` traverses a `*SelectQuery` whose `OuterSettings` is nil, or `Walk` is invoked on it
- **THEN** no additional `OuterSettings` visit / `fn` invocation occurs beyond what was triggered today by the existing `Settings` traversal

### Requirement: Existing format and beautify goldens SHALL remain byte-identical

Adding the two new fields SHALL NOT cause any pre-existing `.sql` fixture's `format/` or `format/beautify/` golden to drift. The load-bearing invariant that makes this possible — parens consumed by the `SubQuery` wrapper do NOT set the inner `SelectQuery.HasParen` — is locked in by the existing `compatible/1_stateful/00080_array_join_and_union.sql` fixture, which has a UNION chain inside a FROM subquery (see D3 in design.md).

#### Scenario: All pre-existing format and beautify goldens pass without -update
- **WHEN** `TestParser_Format` and `TestParser_FormatBeautify` are run after this change against every pre-existing fixture
- **THEN** every golden file matches byte-for-byte without `-update`

#### Scenario: FROM-subquery-wrapped UNION fixture is the regression guard
- **WHEN** `TestParser_Format/00080_array_join_and_union.sql` and `TestParser_FormatBeautify/00080_array_join_and_union.sql` are run after this change
- **THEN** both pass byte-identical, confirming that subquery-consumed parens do NOT trigger the new paren-emitting branch

### Requirement: Pre-existing JSON goldens SHALL gain exactly two added lines per `SelectQuery` rendering

Every pre-existing JSON golden whose AST contains a `SelectQuery` SHALL gain exactly two added lines at each `SelectQuery` rendering: `"HasParen": false,` and `"OuterSettings": null`. No line SHALL be removed; no other field SHALL move position.

#### Scenario: Non-paren JSON golden diff is mechanical
- **WHEN** `TestParser_ParseStatements/select_expr.sql` (any small SELECT golden without parser-consumed parens) is regenerated against the post-change parser
- **THEN** the diff against the pre-change golden at each `SelectQuery` rendering consists of exactly two added lines (`"HasParen": false,` and `"OuterSettings": null`) and no other change

#### Scenario: Set-op JSON golden gains the same two lines per leg
- **WHEN** `TestParser_ParseStatements/select_with_union_settings.sql` is regenerated against the post-change parser
- **THEN** each of the two `SelectQuery` renderings (outer and inner via `Union`) gains the same two added lines AND the existing `Settings`-populated subtree on the inner is unchanged

### Requirement: Four new `.sql` fixtures SHALL exercise the four placements end-to-end

Four `.sql` fixtures SHALL be added under `parser/testdata/query/`, each carrying its three goldens (`output/`, `format/`, `format/beautify/`):

- `select_with_paren_leg_settings_inside.sql` — `SELECT 1 UNION ALL (SELECT 2 SETTINGS max_threads = 1)`.
- `select_with_paren_leg_settings_outside.sql` — `SELECT 1 UNION ALL (SELECT 2) SETTINGS max_threads = 1`.
- `select_with_paren_chain_settings.sql` — `(SELECT 1 UNION ALL SELECT 2) SETTINGS max_threads = 1`.
- `select_with_paren_leg_no_settings.sql` — `SELECT 1 UNION ALL (SELECT 2)`.

#### Scenario: All four new fixtures flow through all three goldens
- **WHEN** the four fixtures are added with their corresponding goldens under `output/`, `format/`, and `format/beautify/`
- **THEN** `TestParser_ParseStatements`, `TestParser_Format`, and `TestParser_FormatBeautify` pass without `-update`

#### Scenario: The two inside-vs-outside fixtures are byte-distinct in the JSON golden
- **WHEN** `select_with_paren_leg_settings_inside.sql.golden.json` and `select_with_paren_leg_settings_outside.sql.golden.json` are compared
- **THEN** they differ in the inner-leg's `Settings` vs `OuterSettings` field placement — the pre-change parser would have produced identical AST shapes for these two SQLs

### Requirement: Inline tests SHALL assert the disambiguation contract

A new inline test `TestParser_With_ChainSettingsDisambiguation` SHALL be added to the parser test suite. It SHALL parse each of the four new fixtures plus the "both placements coexist" SQL (`SELECT 1 UNION ALL (SELECT 2 SETTINGS max_threads=1) SETTINGS max_threads=2`) and assert the expected `HasParen` / `Settings` / `OuterSettings` field values on the inner and outer `*SelectQuery` per the scenarios above. The test SHALL include at least one assertion that the per-node invariant holds on a no-parens fixture.

#### Scenario: All disambiguation forms pass the inline test
- **WHEN** `TestParser_With_ChainSettingsDisambiguation` is executed against the post-change parser
- **THEN** every SQL string in the test passes `require.NoError(t, err)` after `ParseStmts` AND every per-input field-shape assertion holds

### Requirement: Pre-existing parser, AST, formatter, walker, and unrelated golden behaviour SHALL be preserved

This change SHALL be additive on the exported AST: no pre-existing `SelectQuery` field renamed, removed, or reordered; no visitor method introduced or renamed; no `SubQuery` behaviour changed; no `omitempty` or `-` JSON tag added to any field. Any pre-existing parse-error contract that is asserted only by `require.Error` (not by error-message content) SHALL continue to pass.

#### Scenario: TestParser_InvalidSyntax keeps passing
- **WHEN** `TestParser_InvalidSyntax` is run after this change
- **THEN** every input that errors today continues to error (note: the specific error *message* for some leading-`(` invalid inputs may change because the dispatch path is different, but `require.Error` is the only assertion)

#### Scenario: SubQuery-wrapped SELECTs preserve their pre-change goldens
- **WHEN** any pre-existing fixture whose AST contains a `*SubQuery` wrapping a `*SelectQuery` is run through `TestParser_Format` and `TestParser_FormatBeautify`
- **THEN** the format and beautify goldens match byte-for-byte without `-update`, confirming that `SubQuery.HasParen` continues to drive paren emission for subquery contexts and the inner `SelectQuery.HasParen` stays false
