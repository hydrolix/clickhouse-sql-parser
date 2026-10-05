# Design: SETTINGS After FORMAT, and Two Upstream Expression Fixes

## Context

`parseSelectStmt` (`parser/parser_query.go`) parses the clauses in a fixed order and ends
with `tryParseSettingsClause` and then `tryParseFormat`. A `SETTINGS` keyword after
`FORMAT <ident>` is left unconsumed, and `ParseStmts` fails with
`<EOF> or ';' was expected, but got: "SETTINGS"`.

ClickHouse's `ParserQueryWithOutput` parses an optional `SETTINGS` clause after
`FORMAT` / `INTO OUTFILE`, so `SELECT 1 FORMAT JSON SETTINGS max_threads = 1` is valid.

## Decisions

### D1 — A separate `FormatSettings` field, not a reuse of `Settings`

```go
// FormatSettings is the SETTINGS clause that appears AFTER the FORMAT clause,
// e.g. `SELECT 1 FORMAT JSON SETTINGS max_threads = 1`. Settings holds the
// clause before FORMAT. Both can be non-nil.
FormatSettings *SettingsClause
```

It is placed immediately after `Format` in `SelectQuery`.

- **Why not reuse `Settings` with a "position" flag?** That would hide the second clause
  when both forms are present, and it breaks the field's documented meaning. A separate
  field follows the existing `OuterSettings` precedent: one field per syntactic position,
  and consumers decide the semantics.
- **Both clauses present:** both are parsed and kept. Task 1.2 checks with
  `clickhouse-local` whether ClickHouse accepts both and which value wins. The result is
  recorded in the spec and in the field comment, but the parser doesn't merge them.

### D2 — Where it is parsed

- **`parseSelectStmt`:** after `tryParseFormat` returns non-nil, call
  `tryParseSettingsClause`, and set `FormatSettings` and `StatementEnd` from it. If there
  is no `FORMAT`, nothing changes. A second `SETTINGS` without `FORMAT` is still an error.
- In a set-operation chain, the trailing leg's `parseSelectStmt` consumes
  `FORMAT … SETTINGS …`. So `FormatSettings` attaches to the trailing leg, as `Settings`
  and `Format` already do.
- The paren-wrapped path (`(…) SETTINGS …`, `OuterSettings`) doesn't parse `FORMAT` today
  and is out of scope.

### D3 — Format, Walk, Accept

- `Format` / `Beautify` write ` SETTINGS …` after the `FORMAT` clause when
  `FormatSettings != nil`.
- `Walk` and `SelectQuery.Accept` visit `FormatSettings` after `Format`.
- The existing ASTVisitor/Walk drift test, if present, must pass.

### D4 — Cherry-picks

- `git cherry-pick -x 0d9ef25 dd867a1` from `upstream/master`.
- Both touch only code that applies cleanly. The single conflict is
  `parser/precedence_test.go`, which the fork doesn't have:
  - keep the upstream test cases that exercise these fixes, moved into a fork test file
    (`parser/precedence_test.go` is added with only those cases);
  - drop the cases that depend on other upstream commits.
- The goldens that came with the picks are regenerated, because the fork's JSON shape
  differs (for example the extra `HasParen`/`OuterSettings` fields).
- `-x` keeps the upstream SHA in each commit message, so the sync can recognise and skip
  them.

Verified in a scratch clone (2026-10-05): with both picks, the corpus failures drop from
22 to 13. The fork's suite passes except the two picked fixtures, whose JSON goldens only
need regenerating.

## Risks / Trade-offs

- **Golden churn:** one line per `SelectQuery` across all JSON goldens. This is
  mechanical, and the round-trip property is unchanged.
- **Divergence from upstream:** `FormatSettings` is fork-only until upstream accepts an
  equivalent. Task 5.2 offers it upstream.
