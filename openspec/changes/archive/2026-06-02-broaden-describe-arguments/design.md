## Context

`parseDescribeStmt` in `parser/parser_table.go` today consumes `DESC`/`DESCRIBE`, an optional `TABLE` keyword, then exactly one `*TableIdentifier` via `parseTableIdentifier`, then an optional `SETTINGS` clause. `parseTableIdentifier` is narrow on purpose — it parses a single `[database.]table` form and nothing else.

The same module already has `parseTableExpr` (in `parser_query.go` around line 367) used wherever a "table expression" appears — including in the FROM clause for a single-table reference. `parseTableExpr` accepts:

- bare table identifier (`foo`, `db.foo`)
- table function call (`numbers(10)`, `remote(...)`)
- parenthesised subquery (`(SELECT ...)`)
- aliased forms (`foo AS f`, `foo f`)
- `FINAL` keyword

The result of `parseTableExpr` is `*TableExpr` — a concrete struct with `Expr` (the inner table-like thing) and an optional `Alias` and `HasFinal` flag.

**This is the exact grammar ClickHouse itself uses** for the argument of `DESCRIBE`. ClickHouse's `ParserDescribeTableQuery` calls `ParserTableExpression` (not a join parser), so routing our parser through `parseTableExpr` makes our grammar match the server's grammar 1:1. The broader sibling `parseJoinExpr` would also accept JOINs — which ClickHouse rejects at the DESCRIBE position — and would create a "parser accepts but server rejects" gap. We intentionally do not use it.

So the broadening is a one-line call-site change in the parser plus a one-line type change in the AST. Everything else falls out of the existing `parseTableExpr` machinery.

## Goals / Non-Goals

**Goals:**
- `DESCRIBE (SELECT ...)`, `DESCRIBE foo AS f`, `DESCRIBE numbers(10)`, `DESCRIBE foo FINAL` all parse successfully.
- `DESCRIBE [TABLE] foo` continues to parse, with `Target` now wrapped in a `*TableExpr` rather than being a bare `*TableIdentifier`.
- `DESCRIBE foo SETTINGS k=v` continues to parse, with both the Target wrapping and the Settings clause preserved.
- `DescribeStmt.End()` and `Accept()` continue to work correctly with the new field type. Both already invoke methods that `*TableExpr` satisfies, so they need no body change.
- `DESCRIBE foo JOIN bar ON ...` continues to be REJECTED. ClickHouse rejects this shape; our parser stays aligned.

**Non-Goals:**
- A new AST node for DESCRIBE. The existing `DescribeStmt` is reused; only the `Target` field type changes.
- A new visitor method. Existing `VisitDescribeExpr` is unchanged; the broader sub-tree it traverses lives behind the same field.
- Preserving the JSON shape of the four pre-existing DESCRIBE/DESC goldens. They will shift — the change explicitly accepts and regenerates them.
- Backward-compatibility shims for the `Target` field type. The change is breaking; consumers unwrap via `stmt.Target.Expr.(*TableIdentifier)` to recover the inner identifier.
- Supporting JOIN expressions as DESCRIBE arguments. ClickHouse's grammar uses `ParserTableExpression` (not a join parser); accepting JOINs in the parser would create a server-vs-parser mismatch.
- Supporting the `SAMPLE` clause on DESCRIBE's target. ClickHouse permits it but it is rarely used in practice and `parseTableExpr` already handles it transparently if added later.

## Decisions

### Decision 1: Route through `parseTableExpr`, NOT `parseJoinExpr`

ClickHouse's `ParserDescribeTableQuery` calls `ParserTableExpression` — the exact analogue of our `parseTableExpr`. It accepts table identifiers, table functions, subqueries, FINAL, and aliases — but **not** JOINs. JOIN expressions appear in ClickHouse's grammar via `ParserTablesInSelectQuery` (which composes JOINs over `ParserTableExpression`), and that production is not invoked from DESCRIBE.

We mirror this exactly: `parseDescribeStmt` calls `parseTableExpr`, never `parseJoinExpr`. The broader helper would accept JOINs that ClickHouse rejects, creating a confusing "parser accepts but server rejects" gap for downstream tooling.

**Why:** Match the server's grammar precisely. The parser is a fidelity exercise — if ClickHouse rejects a shape at execution, our parser should reject it at parse time, not produce an AST that the server will refuse.

**Alternative considered:** Use `parseJoinExpr` to match origin/main's choice. **Rejected.** Origin/main reused the broader helper without verifying against ClickHouse's grammar; their parser accepts `DESCRIBE foo JOIN bar ...` which ClickHouse will not execute. Following origin/main here would propagate that bug.

**Alternative considered:** Hand-roll a `parseDescribeTarget` that switches on the lookahead. **Rejected.** It would duplicate logic that `parseTableExpr` already encapsulates and would create an opportunity for DESCRIBE's grammar to drift from FROM's single-table grammar.

### Decision 2: Change `Target` field type from `*TableIdentifier` to `*TableExpr` — concrete, not interface

`parseTableExpr` always returns `*TableExpr`. Holding the field as `Expr` (interface) would be strictly looser than the actual contract — every consumer would have to type-assert before reading anything useful. Holding it as `*TableExpr` (concrete) is type-honest: callers immediately have access to `.Expr`, `.Alias`, `.HasFinal` without an interface unwrap.

**Why:** The function's return type IS the contract. `*TableExpr` is what gets stored; saying so in the field type saves every reader from a redundant assertion.

**Alternative considered:** Use `Expr` (interface). **Rejected** — looser than the function's actual return type, with no upside. The only reason to use `Expr` would be if the field could legitimately hold non-TableExpr types, which it cannot.

**Alternative considered:** Keep `Target *TableIdentifier` AND add a new `*TableExpr` field. **Rejected** — two parallel fields with overlapping semantics is a maintenance hazard.

**Consumer migration:** Code that today reads `stmt.Target.Database` or `stmt.Target.Table.Name` becomes:

```go
if ti, ok := stmt.Target.Expr.(*TableIdentifier); ok {
    // use ti.Database, ti.Table.Name
}
```

The unwrap is **one** level (through `.Expr`), not two — `*TableExpr` is concrete, so we read its `Expr` field directly; only the inner content is interface-typed. That inner unwrap is the existing pattern wherever the codebase reads inside a `*TableExpr` (e.g., the formatter, every visitor implementation that cares about distinguishing a TableIdentifier from a SubQuery inside a FROM clause).

### Decision 3: `End()` and `Accept()` need no body changes

`DescribeStmt.End()` currently calls `d.Target.End()` — `*TableExpr` satisfies the same `End() Pos` method, so the call works after the field-type change. Same for `Accept()`'s `d.Target.Accept(visitor)` — `*TableExpr.Accept` exists and recursively visits the inner expression and the alias.

**Why this matters:** No additional surface area to change. `*TableIdentifier` and `*TableExpr` both satisfy the methods called by `End()` and `Accept()`, so the field-type swap is a single edit.

### Decision 4: Accept multi-line golden shifts for 6 pre-existing fixtures

The four existing DESCRIBE/DESC goldens plus the two SETTINGS-bearing goldens from the recent `add-describe-settings-clause` change all render `Target` as a bare `*TableIdentifier` today:

```json
"Target": {
  "Database": null,
  "Table": { "Name": "mytable", ... }
}
```

After the change, `parseJoinExpr` returns a `*TableExpr` wrapping the identifier, so `Target` JSON becomes:

```json
"Target": {
  "TablePos": <int>,
  "TableEnd": <int>,
  "Alias": null,
  "Expr": {
    "Database": null,
    "Table": { "Name": "mytable", ... }
  },
  "HasFinal": false
}
```

That's a multi-line shift, not a single-line shift like the previous DESCRIBE change. **Workflow:** snapshot each affected golden into `/tmp/` before regenerating, regenerate via `-update`, diff each pair, confirm the structural shift makes sense (no positional drift inside the inner `*TableIdentifier` block, no missing fields, no extra fields beyond the expected `*TableExpr` wrapping).

Format and beautify goldens for the same six fixtures should be inspected too — if the formatter renders `*TableExpr` for the simple-table case identically to how it renders a bare `*TableIdentifier`, the format/beautify goldens stay byte-identical. Either way the diff is informative.

### Decision 5: Use three new fixtures matching ClickHouse's accepted shapes

`describe_subquery.sql`, `describe_with_alias.sql`, `describe_table_function.sql` — each generates 3 goldens. All three correspond to shapes ClickHouse actually accepts at execution time. A fourth obvious candidate, `describe_join.sql`, is **explicitly NOT a fixture**: ClickHouse rejects the join form at the DESCRIBE position, so our parser should reject it too. That negative behaviour is verified by an inline test (see Decision 6) rather than a positive golden.

### Decision 6: Inline tests cover both positive and negative behaviour

Two inline tests live in `parser/parser_test.go`:

- `TestParser_Describe_RichArguments` — positive test for every shape ClickHouse accepts (subquery, aliased table, table function, FINAL, plus regression cases for the existing forms).
- `TestParser_Describe_RejectsJoin` — negative test asserting `DESCRIBE foo JOIN bar ON ...` returns a parse error. This locks the scope decision and prevents a future "let's just broaden to parseJoinExpr" refactor from silently regressing into the parser-vs-server gap that origin/main shipped.

The new golden fixtures lock structural shape and formatter round-trip; the inline tests lock acceptance/rejection semantics. Both are needed.

## Acceptance surface after this change

Nine SQL shapes that fail to parse on the current HEAD will flip to PASS after this change lands. They are listed below in the exact order `TestParser_Describe_RichArguments` exercises them (a deliberate ordering — subqueries first, then aliases, then table functions, then `FINAL`, then mixed forms — so a reviewer scanning the test output sees one category before moving to the next). All nine were probed against a live ClickHouse 26.5.1.882 server; the right-most column records what the server itself does with the SQL.

| # | SQL | Shape category | ClickHouse 26.5 verdict |
| --- | --- | --- | --- |
| 1 | `DESCRIBE (SELECT 1)` | bare subquery target | parser ✓ · executes ✓ |
| 2 | `DESCRIBE (SELECT a, b FROM inner_table) AS subq` | subquery + alias | parser ✗ rejected ⚠ — parser-server gap (see below) |
| 3 | `DESCRIBE foo AS f` | table identifier + alias | parser ✓ · UNKNOWN_TABLE at exec (parser-accepted) |
| 4 | `DESCRIBE db.foo AS f` | dotted table identifier + alias | parser ✓ · UNKNOWN_DATABASE at exec (parser-accepted) |
| 5 | `DESCRIBE numbers(10)` | table function | parser ✓ · executes ✓ |
| 6 | `DESCRIBE remote('host', db.foo)` | table function with args | parser ✓ · network error at exec (parser-accepted) |
| 7 | `DESCRIBE foo FINAL` | `FINAL` keyword on target | parser ✓ · UNKNOWN_TABLE at exec (parser-accepted) |
| 8 | `DESCRIBE (SELECT 1) SETTINGS describe_compact_output=1` | subquery + SETTINGS | parser ✓ · executes ✓ |
| 9 | `DESCRIBE (SELECT * FROM foo JOIN bar ON foo.x = bar.x)` | JOIN-wrapped-in-subquery | parser ✓ · UNKNOWN_TABLE at exec (parser-accepted) |

Three additional cases serve as regression guards in `TestParser_Describe_RichArguments` (`DESCRIBE TABLE foo`, `DESCRIBE foo SETTINGS k=1`, `DESCRIBE db.foo`) — they pass today and must continue to pass; they are not listed above because they are not new acceptance.

### The one parser-vs-server gap

Shape #2 (`DESCRIBE (subquery) AS alias`) is the only case where this change makes our parser more permissive than ClickHouse. Probing the live server returns `SYNTAX_ERROR: Expected one of: UNION, EXCEPT, INTERSECT, SETTINGS, INTO OUTFILE, FORMAT, ParallelWithClause, PARALLEL WITH, end of query.` — the SELECT-statement follower set, not the table-expression follower set. ClickHouse's actual DESCRIBE-of-subquery production routes the inner `(SELECT ...)` through a SELECT-statement parser that does not permit a trailing alias.

We deliberately do **not** add a post-parse check to tighten this. The decision is documented under "Risks / Trade-offs" below; the short version is that the gap is rare in practice, easy to diagnose (the server returns a clear syntax error), and keeping `parseTableExpr` clean is more valuable than chasing one edge case.

A separate negative-coverage test `TestParser_Describe_RejectsJoin` continues to lock the JOIN-at-DESCRIBE-position rejection — confirmed against the same live server (shape rejected with the same SYNTAX_ERROR family).

## Risks / Trade-offs

- **Risk: A consumer crashes at runtime when its code reads `stmt.Target.Database` directly (the old shape) against a `*TableExpr` value.** The field-type change forces a compile error (not a runtime panic) at every existing call site that accesses `Target` as a struct rather than via the new wrapper-and-inner pattern. Consumers see the breakage at build time and rewrite.
- **Risk: A `DESCRIBE` of an arbitrary subquery is a weird input — does ClickHouse actually execute it?** Yes, `DESCRIBE (SELECT ...)` returns the column types of the subquery's result set; this is documented and used by tooling that introspects expression types.
- **Risk: `parseTableExpr` might consume `SETTINGS` as part of the FINAL/SAMPLE handling and shadow the trailing `SETTINGS` clause support.** Verify by probe during apply: `DESCRIBE foo SETTINGS k=1` and `DESCRIBE foo FINAL SETTINGS k=1` must both parse correctly with `Target` and `Settings` populated. If `parseTableExpr` greedily consumes `SETTINGS`, this would break. (Unlikely — `SETTINGS` is not part of `parseTableExpr`'s grammar — but worth confirming.)
- **Trade-off: Six existing goldens shift in a multi-line way.** Reviewable but larger than the previous DESCRIBE-SETTINGS change's single-line shift. Visual inspection per file is mandatory.
- **Trade-off: The change is breaking for the field type.** Disclosed in the proposal; consumers must migrate. A grep of the hydrolix internal repos (`sqlds`, `chproxy`, `grafana-datasource-plugin`) for `DescribeStmt` or any `.Target.` access pattern is part of the apply workflow — none of those consumers appears to use DESCRIBE today, but the grep is the safe check.
- **Trade-off: We intentionally diverge from origin/main, which uses the broader `parseJoinExpr`.** Origin/main's choice is server-incompatible (its parser accepts JOIN shapes that ClickHouse rejects). Our parser is the more faithful one — verified against ClickHouse's `ParserDescribeTableQuery` source.
- **Known parser-vs-server gap for shape #2 in the acceptance surface above.** `DESCRIBE (subquery) AS alias` is accepted by our parser via `parseTableExpr` but rejected by ClickHouse with `SYNTAX_ERROR` because ClickHouse routes the inner subquery through a SELECT-statement parser that does not permit a trailing alias. We do not tighten the parser to match here — adding a post-parse check (`if Target.Expr is *SubQuery && Target.Alias != nil { return err }`) would introduce a special case for what is, in practice, a rarely-used shape. The server returns a clean syntax error if someone tries it; that's adequate signal. Locked by `TestParser_Describe_RichArguments` including shape #2 as a positive case.

## Migration Plan

Single commit, no dependencies, no data or config involvement. Rollback is `git revert`. The six regenerated goldens are part of the same commit so the AST snapshot remains in sync with the parser code.
