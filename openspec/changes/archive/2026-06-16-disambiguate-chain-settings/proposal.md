## Why

When a UNION / EXCEPT / INTERSECT chain ends in a parenthesised SELECT and a `SETTINGS` clause trails the chain, the parser today cannot tell whether the SETTINGS was authored *inside* the parens (per-leg, scoped to the subquery) or *outside* the parens (chain-level, scoped to the whole set-op expression). The current AST drops the parens entirely and folds both forms onto the inner leg's `Settings` field, so:

- `SELECT 1 UNION ALL (SELECT 2 SETTINGS max_threads=1)` — author intent: per-leg
- `SELECT 1 UNION ALL SELECT 2 SETTINGS max_threads=1` — no parens, current per-leg attachment

produce **byte-identical** ASTs and round-trip to the same SQL text. Worse, the third form ClickHouse accepts:

- `SELECT 1 UNION ALL (SELECT 2) SETTINGS max_threads=1` — author intent: chain-level

**fails to parse today** with `<EOF> or ';' was expected, but got: "SETTINGS"`, and `(SELECT 1 UNION ALL SELECT 2) SETTINGS max_threads=1` fails at the leading `(`. Tools that round-trip parenthesised SQL through this parser silently lose author intent for the paren-bounded form, and chain-level SETTINGS on a paren-wrapped chain cannot be expressed at all.

This change disambiguates the placements end-to-end: the AST records whether a `SelectQuery` was wrapped in parens, and trailing SETTINGS after the closing `)` is captured in a separate field distinct from the inner SELECT's own `Settings`.

## What Changes

- **The four placements of SETTINGS around a paren-wrapped set-op leg become distinguishable in the AST and round-trip-stable through the formatter.** `SELECT 1 UNION ALL (SELECT 2 SETTINGS x=1)` and `SELECT 1 UNION ALL (SELECT 2) SETTINGS x=1` parse to different shapes; each re-formats to itself byte-for-byte.

- **`SelectQuery` gains two additive fields** as part of its exported AST surface:
  - `HasParen bool` — true when the parser consumed wrapping parens around this `SelectQuery` itself (mirrors the existing `SubQuery.HasParen` flag for the existing in-repo precedent).
  - `OuterSettings *SettingsClause` — the optional `SETTINGS` clause appearing *after* the closing `)`, distinct from the existing `Settings` field (which continues to mean "SETTINGS inside the SELECT body").

- **Per-node invariant**: `OuterSettings != nil` ⇒ `HasParen == true`. When `HasParen` is false, `OuterSettings` is nil.

- **`(SELECT … UNION … SELECT …) SETTINGS …` parses as a top-level statement.** Today this fails at the leading `(`; after the change it produces a `SelectQuery` with `HasParen=true` and `OuterSettings` populated. As a side effect, a bare top-level `(SELECT 1)` also parses (today: "unexpected token: `(`").

- **`SELECT 1 UNION ALL (SELECT 2) SETTINGS …` parses.** Today the trailing SETTINGS errors at the statement boundary; after the change it lands on the inner leg's `OuterSettings`.

- **The no-parens form is unchanged.** `SELECT 1 UNION ALL SELECT 2 SETTINGS x=1` continues to attach the SETTINGS to the inner leg's `Settings` field (`HasParen=false`, `OuterSettings=nil`). This change does NOT retroactively reinterpret no-parens trailing SETTINGS as chain-level; parens are the explicit disambiguator.

- **Subquery contexts are unchanged.** `FROM (SELECT …)`, scalar subqueries, view bodies, and INSERT-SELECT keep their existing parse shape — the wrapping `SubQuery.HasParen` continues to drive paren emission there, and the inner `SelectQuery.HasParen` stays false in those contexts.

- **`Format`, `Beautify`, `Accept`, and `Walk` all honour the new fields.** `Format(stmt)` emits `(…)` around a `HasParen=true` SelectQuery and appends the `OuterSettings` clause after the `)`. The visitor and walker traverse `OuterSettings` in lexical order (after the set-op chain, before the outer visitor call).

## Capabilities

### New Capabilities
- `paren-wrapped-select-query`: Track whether a `SelectQuery` was parsed as a parenthesised expression and, when so, capture an optional trailing `SETTINGS` clause that appears *after* the closing `)`. Disambiguates "SETTINGS inside the parens" (per-leg) from "SETTINGS outside the parens" (chain-level) in set-op chains, and unlocks the top-level `(chain) SETTINGS …` form.

### Modified Capabilities
<!--
The existing set-operator-modes spec already covers per-leg SETTINGS in set-op chains
(e.g. `SELECT 1 SETTINGS x=1 UNION SELECT 2 SETTINGS y=2`). Those scenarios continue
to hold byte-for-byte — the new HasParen / OuterSettings fields are strictly additive
on SelectQuery and do not alter the existing per-leg shape. Disambiguation lives
entirely in the new `paren-wrapped-select-query` spec.
-->

## Impact

- **AST API compatibility**: additive only. No existing exported field is renamed, removed, or reordered. Code that depends on the current `SelectQuery` surface compiles unchanged. Code that switch-cases on the AST gains two new fields it can ignore.

- **JSON-golden footprint**: every committed JSON golden whose AST contains a `SelectQuery` (approximately 90 fixtures across `parser/testdata/`) gains exactly two added lines per `SelectQuery` rendering: `"HasParen": false,` and `"OuterSettings": null`. No field is removed; no positional movement of any other field. The four new fixtures additionally render populated values on the affected nodes.

- **Format and beautify golden footprint**: every existing fixture's `format/` and `format/beautify/` golden remains **byte-identical**. This is the strong claim. It depends on the parser-side boundary that subquery parens are consumed by the subquery wrapper (so the inner `SelectQuery.HasParen` stays false), and is locked in by the existing `compatible/1_stateful/00080_array_join_and_union.sql` fixture — a `SELECT count() FROM (SELECT … UNION ALL SELECT …)` — which acts as the regression guard.

- **New `.sql` fixtures** under `parser/testdata/query/`, one per placement plus the no-SETTINGS round-trip case:
  - `select_with_paren_leg_settings_inside.sql` — `SELECT 1 UNION ALL (SELECT 2 SETTINGS max_threads = 1)` (per-leg).
  - `select_with_paren_leg_settings_outside.sql` — `SELECT 1 UNION ALL (SELECT 2) SETTINGS max_threads = 1` (chain-level on the leg).
  - `select_with_paren_chain_settings.sql` — `(SELECT 1 UNION ALL SELECT 2) SETTINGS max_threads = 1` (top-level wrapped chain).
  - `select_with_paren_leg_no_settings.sql` — `SELECT 1 UNION ALL (SELECT 2)` (parens preserved with no SETTINGS).
  Each carries the standard three goldens (`output/`, `format/`, `format/beautify/`).

- **New inline test** `TestParser_With_ChainSettingsDisambiguation` in `parser/parser_test.go` parses each of the four new SQLs plus a "both placements coexist" SQL, and asserts the expected `HasParen` / `Settings` / `OuterSettings` shape on the inner and outer `SelectQuery`. Today's parser fails on three of the five inputs; after the change all five PASS.

- **Round-trip property**: `Format(Parse(sql)) == Format(Parse(Format(Parse(sql))))` continues to hold for every fixture, including the four new ones.

- **No dependencies added.** No lexer changes. No new visitor method. No new keyword.

- **Rollback**: `git revert`. The fields are additive; reverting the commit restores the pre-change JSON-golden shape mechanically.

- **Performance posture**: one additional optional `SettingsClause` parse attempt at the close of any paren-wrapped `SelectQuery` (a single keyword lookahead in the common no-trailing-SETTINGS case). No hot-path concern.

- **Out of scope**: retroactively reinterpreting `SELECT 1 UNION ALL SELECT 2 SETTINGS x=1` (no parens) as chain-level SETTINGS; mixed-operator precedence (already a known limitation from the prior `add-set-operator-modes` change); generalising paren tracking to constructs already handled by `SubQuery`.
