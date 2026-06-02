## ADDED Requirements

### Requirement: AST SHALL model UNION, EXCEPT, and INTERSECT as three pointer-pairs of `<Operator>, <Operator>Mode`

Three new typed aliases SHALL be added to `parser/ast.go`, each with three constants matching the `OrderDirection` precedent:

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

The `SelectQuery` struct SHALL expose its set-operator right-hand side via three optional pointer-pairs:
- `Union *SelectQuery` + `UnionMode UnionMode`
- `Except *SelectQuery` + `ExceptMode ExceptMode`
- `Intersect *SelectQuery` + `IntersectMode IntersectMode`

The legacy fields `UnionAll *SelectQuery` and `UnionDistinct *SelectQuery` SHALL be removed in the same change. The existing `Except *SelectQuery` field SHALL be retained and gain its companion `ExceptMode`. The new `Intersect *SelectQuery` and `IntersectMode IntersectMode` SHALL be added at the end of the set-op block.

**Per-node invariant.** For each operator pair, when the pointer is nil the mode SHALL be the zero value (`*ModeNone`) and MUST NOT be depended on. When the pointer is non-nil the mode SHALL be exactly one of the three constants for that operator. At most one of `Union`, `Except`, `Intersect` SHALL be non-nil on any given `SelectQuery` node.

#### Scenario: Three mode types exist with documented values
- **WHEN** a Go consumer imports the `parser` package after this change
- **THEN** `parser.UnionMode`, `parser.ExceptMode`, and `parser.IntersectMode` are each a string-typed alias AND each has exported `*None` (value `""`), `*All` (value `"ALL"`), `*Distinct` (value `"DISTINCT"`) constants of the corresponding type

#### Scenario: SelectQuery exposes the new fields and not the old ones
- **WHEN** a Go consumer reflects on `parser.SelectQuery` after this change
- **THEN** the struct has fields `Union *SelectQuery`, `UnionMode UnionMode`, `Except *SelectQuery`, `ExceptMode ExceptMode`, `Intersect *SelectQuery`, `IntersectMode IntersectMode` AND does NOT have fields named `UnionAll` or `UnionDistinct`

### Requirement: `INTERSECT` SHALL be a recognised keyword

`parser/keyword.go` SHALL declare a new constant `KeywordIntersect = "INTERSECT"` and include it in the keyword-recognition slice. The keyword SHALL be ordered alphabetically among existing entries (between `KeywordInterpolate` and `KeywordInto`).

#### Scenario: INTERSECT is recognised as a keyword token
- **WHEN** the lexer scans the literal text `INTERSECT` in a position where a keyword can appear
- **THEN** the resulting token matches `KeywordIntersect` AND the lexer does NOT treat the text as an identifier

### Requirement: Parser SHALL accept all nine surface forms

`parseSelectQuery` SHALL recognise each of `UNION`, `EXCEPT`, and `INTERSECT` followed optionally by `ALL` or `DISTINCT`. In all nine cases it SHALL recurse into `parseSelectQuery` for the right-hand side and store the result in the corresponding pointer field on the parent `SelectQuery`, setting the corresponding mode field to one of `*ModeNone` (bare), `*ModeAll`, or `*ModeDistinct`.

#### Scenario: All three UNION forms populate Union with the correct mode
- **WHEN** `SELECT 1 UNION SELECT 2`, `SELECT 1 UNION ALL SELECT 2`, and `SELECT 1 UNION DISTINCT SELECT 2` are parsed
- **THEN** each `ParseStmts` returns no error AND each outer `*SelectQuery` has its `Union` field non-nil AND `UnionMode` equal to `UnionModeNone`, `UnionModeAll`, `UnionModeDistinct` respectively AND its `Except` and `Intersect` fields are nil

#### Scenario: All three EXCEPT forms populate Except with the correct mode
- **WHEN** `SELECT 1 EXCEPT SELECT 2`, `SELECT 1 EXCEPT ALL SELECT 2`, and `SELECT 1 EXCEPT DISTINCT SELECT 2` are parsed
- **THEN** each `ParseStmts` returns no error AND each outer `*SelectQuery` has its `Except` field non-nil AND `ExceptMode` equal to `ExceptModeNone`, `ExceptModeAll`, `ExceptModeDistinct` respectively AND its `Union` and `Intersect` fields are nil

#### Scenario: All three INTERSECT forms populate Intersect with the correct mode
- **WHEN** `SELECT 1 INTERSECT SELECT 2`, `SELECT 1 INTERSECT ALL SELECT 2`, and `SELECT 1 INTERSECT DISTINCT SELECT 2` are parsed
- **THEN** each `ParseStmts` returns no error AND each outer `*SelectQuery` has its `Intersect` field non-nil AND `IntersectMode` equal to `IntersectModeNone`, `IntersectModeAll`, `IntersectModeDistinct` respectively AND its `Union` and `Except` fields are nil

#### Scenario: Bare UNION combined with per-leg SETTINGS
- **WHEN** `SELECT 1 SETTINGS max_threads=1 UNION SELECT 2 SETTINGS max_threads=2` is parsed
- **THEN** `ParseStmts` returns no error AND the outer `*SelectQuery` has both its `Settings` and `Union` non-nil AND `outer.UnionMode == UnionModeNone` AND the inner `*SelectQuery` (`outer.Union`) also has its `Settings` non-nil

#### Scenario: INTERSECT ALL with trailing SETTINGS on the right leg
- **WHEN** `SELECT 1 INTERSECT ALL SELECT 2 SETTINGS max_threads=2` is parsed
- **THEN** `ParseStmts` returns no error AND the outer `*SelectQuery` has `Intersect` non-nil AND `outer.IntersectMode == IntersectModeAll` AND `outer.Intersect.Settings` is non-nil AND `outer.Settings` is nil

#### Scenario: Bare EXCEPT continues to parse unchanged
- **WHEN** `SELECT number FROM numbers(1, 10) EXCEPT SELECT number FROM numbers(3, 6) EXCEPT SELECT number FROM numbers(8, 9)` (the existing `select_with_multi_except.sql` fixture) is parsed
- **THEN** `ParseStmts` returns no error AND the result has `Except` populated all the way down the chain with each level's `ExceptMode == ExceptModeNone` (bare form preserved)

### Requirement: `SelectQuery.Accept()` SHALL traverse the new field shape

`(*SelectQuery).Accept(visitor)` SHALL contain exactly three set-op traversal blocks, in order — `Union`, `Except`, `Intersect` — each gated on the corresponding pointer being non-nil. The function SHALL NOT reference `UnionAll` or `UnionDistinct`.

#### Scenario: Visitor sees the UNION RHS regardless of mode
- **WHEN** a visitor traverses a `SelectQuery` whose `Union` is populated (any `UnionMode` value)
- **THEN** `VisitSelectQuery` is invoked on both the outer node and the RHS node (RHS as a child of outer) in pre-order traversal

#### Scenario: Visitor sees the INTERSECT RHS
- **WHEN** a visitor traverses a `SelectQuery` whose `Intersect` is populated
- **THEN** `VisitSelectQuery` is invoked on the RHS node referenced by `outer.Intersect` as a child of the outer node

#### Scenario: Visitor traversal skips empty set-op slots
- **WHEN** a visitor traverses a `SelectQuery` whose `Union`, `Except`, and `Intersect` are all nil
- **THEN** no traversal happens for any of the three slots; the rest of the traversal (`With`, `Top`, `SelectItems`, …, `Format`) is unchanged

### Requirement: `Walk()` SHALL traverse the new field shape

The `SelectQuery` case in `parser/walk.go`'s `Walk` function SHALL contain exactly one `Walk(n.Union, fn)` call, one `Walk(n.Except, fn)` call, and one `Walk(n.Intersect, fn)` call, in that order. It SHALL NOT reference `UnionAll` or `UnionDistinct`.

#### Scenario: Walk visits each populated set-op RHS
- **WHEN** `Walk(outer, fn)` is invoked on a `SelectQuery` whose any one of `Union`, `Except`, `Intersect` is non-nil
- **THEN** `fn` is invoked at least once on the corresponding RHS subtree

### Requirement: Formatter SHALL emit the correct keyword sequence for each operator and mode

`SelectQuery.FormatSQL` SHALL contain three parallel set-op arms — `if s.Union != nil { … } else if s.Except != nil { … } else if s.Intersect != nil { … }` — each using an inner `switch` on its mode discriminator to select:
- `UNION ALL` / `UNION DISTINCT` / `UNION` for the Union arm
- `EXCEPT ALL` / `EXCEPT DISTINCT` / `EXCEPT` for the Except arm
- `INTERSECT ALL` / `INTERSECT DISTINCT` / `INTERSECT` for the Intersect arm

Each arm SHALL use the same `Break` / `WriteString` / `Break` / `WriteExpr` shape used by the existing set-op formatter, so beautified output places the operator on its own line.

#### Scenario: UNION ALL formats as `UNION ALL`
- **WHEN** a `SelectQuery` with `Union != nil` and `UnionMode == UnionModeAll` is formatted
- **THEN** the output contains the substring `UNION ALL` AND does NOT contain `UNION DISTINCT`

#### Scenario: Bare UNION round-trips as `UNION`
- **WHEN** `SELECT 1 AS v UNION SELECT 2 AS v` is parsed and formatted
- **THEN** the output contains the substring `UNION` AND does NOT contain `UNION ALL` or `UNION DISTINCT`

#### Scenario: EXCEPT ALL and EXCEPT DISTINCT round-trip with their modifiers
- **WHEN** `SELECT 1 AS v EXCEPT ALL SELECT 2 AS v` and `SELECT 1 AS v EXCEPT DISTINCT SELECT 2 AS v` are parsed and formatted
- **THEN** the outputs contain the substrings `EXCEPT ALL` and `EXCEPT DISTINCT` respectively (and not the other variant)

#### Scenario: Bare EXCEPT continues to round-trip as `EXCEPT`
- **WHEN** the existing `select_with_multi_except.sql` is formatted
- **THEN** the output is byte-identical to its pre-change format golden — each `EXCEPT` keyword appears with no `ALL`/`DISTINCT` modifier

#### Scenario: All three INTERSECT forms round-trip with the correct keyword sequence
- **WHEN** `SELECT 1 AS v INTERSECT SELECT 2 AS v`, `SELECT 1 AS v INTERSECT ALL SELECT 2 AS v`, and `SELECT 1 AS v INTERSECT DISTINCT SELECT 2 AS v` are parsed and formatted
- **THEN** the outputs contain `INTERSECT`, `INTERSECT ALL`, `INTERSECT DISTINCT` respectively (each exclusive of the other two)

#### Scenario: Beautified output places each operator on its own line
- **WHEN** any of the six new golden fixtures is beautified
- **THEN** the beautified output contains a line whose trimmed contents are exactly the operator's emitted keyword sequence (`UNION`, `UNION ALL`, `EXCEPT ALL`, `EXCEPT DISTINCT`, `INTERSECT`, `INTERSECT ALL`, `INTERSECT DISTINCT`)

#### Scenario: Existing format goldens unchanged for all four pre-existing set-op fixtures
- **WHEN** `TestParser_Format` and `TestParser_FormatBeautify` are run after this change against `select_with_union_distinct.sql`, `select_with_multi_union.sql`, `select_with_multi_union_distinct.sql`, and `select_with_multi_except.sql`
- **THEN** all eight golden files (four format, four beautify) match byte-for-byte without `-update`

### Requirement: New `.sql` fixtures SHALL exercise the newly-unlocked surface forms end-to-end

Six `.sql` fixtures SHALL be added under `parser/testdata/query/`. Each SHALL be exercised by `TestParser_ParseStatements`, `TestParser_Format`, and `TestParser_FormatBeautify`, with corresponding golden files committed under `output/`, `format/`, and `format/beautify/`:

- `select_with_bare_union.sql` — `SELECT 1 AS v UNION SELECT 2 AS v`
- `select_with_union_settings.sql` — `SELECT 1 AS v SETTINGS max_threads = 1 UNION SELECT 2 AS v SETTINGS max_threads = 2`
- `select_with_except_all.sql` — `SELECT 1 AS v EXCEPT ALL SELECT 2 AS v`
- `select_with_except_distinct.sql` — `SELECT 1 AS v EXCEPT DISTINCT SELECT 2 AS v`
- `select_with_intersect.sql` — `SELECT 1 AS v INTERSECT SELECT 2 AS v`
- `select_with_intersect_modifiers.sql` — `SELECT 1 AS v INTERSECT ALL SELECT 2 AS v INTERSECT DISTINCT SELECT 3 AS v`

#### Scenario: All six new fixtures flow through all three goldens
- **WHEN** the six fixtures listed above are added with their corresponding goldens under `output/`, `format/`, and `format/beautify/`
- **THEN** `go test ./parser/... -run 'TestParser_ParseStatements|TestParser_Format|TestParser_FormatBeautify' -count=1` passes without `-update`

#### Scenario: The SETTINGS+UNION JSON golden shows per-leg SETTINGS
- **WHEN** `select_with_union_settings.sql.golden.json` is generated
- **THEN** the outer `*SelectQuery` has both `Settings` and `Union` non-nil, AND `outer.Union.Settings` is also non-nil

### Requirement: Inline tests SHALL cover all nine surface forms and SETTINGS combinations

A new test function `TestParser_With_SetOperators` SHALL be added to `parser/parser_test.go`, exercising at least these 11 SQL strings:
- bare/`ALL`/`DISTINCT` UNION (3 SQLs)
- bare/`ALL`/`DISTINCT` EXCEPT (3 SQLs)
- bare/`ALL`/`DISTINCT` INTERSECT (3 SQLs)
- bare UNION with per-leg SETTINGS (1 SQL)
- INTERSECT ALL with trailing SETTINGS on the right leg (1 SQL)

Each SQL SHALL parse without error.

#### Scenario: All nine matrix cells and both SETTINGS combinations parse
- **WHEN** `TestParser_With_SetOperators` is executed against the post-change parser
- **THEN** every SQL string in the test passes `require.NoError(t, err)` after `ParseStmts`

### Requirement: Pre-existing JSON goldens SHALL be regenerated by a defined per-occurrence diff

Every JSON golden under `parser/testdata/**/output/*.sql.golden.json` that today renders `"UnionAll": null` (90 files) SHALL be regenerated as part of this change. For each `SelectQuery` rendering inside such a file:
- The lines `"UnionAll": null,` and `"UnionDistinct": null,` SHALL be removed.
- The lines `"Union": null,`, `"UnionMode": "",`, `"ExceptMode": "",`, `"Intersect": null,`, and `"IntersectMode": ""` SHALL be added in the appropriate struct positions.
- The pre-existing `"Except": null,` line (or its populated counterpart) SHALL remain.

For the four set-op-populated fixtures (`select_with_union_distinct.sql`, `select_with_multi_union.sql`, `select_with_multi_union_distinct.sql`, `select_with_multi_except.sql`):
- The UNION fixtures' previously-populated `"UnionAll": { … }` or `"UnionDistinct": { … }` subtree SHALL appear under the new `"Union"` key with byte-identical inner contents (modulo the same renames applied recursively to nested SelectQuery objects), and `"UnionMode"` SHALL render as `"ALL"` or `"DISTINCT"` at the SelectQuery node that owns the populated subtree.
- The EXCEPT fixture's `"Except": { … }` subtree SHALL appear at the same key with the same inner contents, and `"ExceptMode": ""` SHALL render at every populated EXCEPT node.

The format and beautify goldens for the four set-op fixtures SHALL remain byte-identical.

#### Scenario: Non-set-op JSON golden experiences a structured rename + addition
- **WHEN** `TestParser_ParseStatements/select_expr.sql` (any small SELECT golden without UNION/EXCEPT/INTERSECT) is run against the post-change parser and the golden is regenerated
- **THEN** at each SelectQuery rendering the diff against the pre-change golden consists of exactly two removed lines (`"UnionAll": null,` and `"UnionDistinct": null,`) and exactly five added lines (`"Union": null,`, `"UnionMode": "",`, `"ExceptMode": "",`, `"Intersect": null,`, `"IntersectMode": ""`) with no positional movement of other fields

#### Scenario: UNION JSON golden migrates the populated subtree
- **WHEN** `TestParser_ParseStatements/select_with_union_distinct.sql` is run against the post-change parser and the golden is regenerated
- **THEN** the previously-populated `"UnionDistinct": { … }` subtree appears under `"Union"` AND `"UnionMode": "DISTINCT"` appears at the outer SelectQuery AND no other AST field has shifted

#### Scenario: EXCEPT JSON golden gains the mode discriminator
- **WHEN** `TestParser_ParseStatements/select_with_multi_except.sql` is run against the post-change parser and the golden is regenerated
- **THEN** the `"Except": { … }` subtree stays at the same field AND `"ExceptMode": ""` appears at every populated EXCEPT node AND no other AST field has shifted (besides the per-SelectQuery rename + additions also applied here)

#### Scenario: Pre-existing set-op format goldens unchanged
- **WHEN** `TestParser_Format` and `TestParser_FormatBeautify` are run after this change against `select_with_union_distinct.sql`, `select_with_multi_union.sql`, `select_with_multi_union_distinct.sql`, and `select_with_multi_except.sql`
- **THEN** all eight golden files (four format, four beautify) match byte-for-byte without `-update`

### Requirement: Existing parser, lexer, and unrelated golden behaviour SHALL be preserved

This change SHALL NOT alter the lexer (beyond the `KeywordIntersect` addition), SHALL NOT introduce or rename any visitor method, SHALL NOT modify `parseSelectStmt` or any helper involved in parsing optional clauses (`tryParseSettingsClause`, etc.), SHALL NOT add `omitempty` or `-` JSON tags to any existing or new field, and SHALL NOT cause any golden-file fixture whose AST does not contain a `SelectQuery` to drift.

#### Scenario: Non-SelectQuery JSON goldens unchanged
- **WHEN** the full golden suite is run after this change
- **THEN** every JSON golden whose rendered AST does not contain a `SelectQuery` (e.g. DDL-only fixtures, ALTER fixtures) matches byte-for-byte without `-update`

#### Scenario: TestParser_InvalidSyntax unchanged
- **WHEN** `TestParser_InvalidSyntax` is run after this change
- **THEN** the test passes with the same set of error inputs that pass today (note: the error *message* for `SELECT 1 UNION <EOF>`-style inputs may change from "expected ALL or DISTINCT" to "expected SELECT, WITH or (", but `require.Error` is the only assertion, so this remains a passing test)

#### Scenario: No existing fixture uses INTERSECT as an identifier
- **WHEN** the keyword `KeywordIntersect = "INTERSECT"` is added to `parser/keyword.go`
- **THEN** no pre-existing fixture under `parser/testdata/` parses differently because `INTERSECT` is now reserved (verified by grep before implementation; if a future fixture needs to use it as an identifier, the fix is backtick-quoting)

### Requirement: Mixed-operator precedence is explicitly NOT a guarantee of this change

This change SHALL NOT modify the right-recursive parsing model that `parseSelectQuery` inherits from today's implementation. Mixed-operator chains such as `a INTERSECT b UNION c` or `a UNION ALL b EXCEPT c` MAY parse with associativity that does not match ClickHouse's documented precedence rules (INTERSECT binds tighter than UNION/EXCEPT; UNION/EXCEPT are left-to-right at equal precedence). Fixing this is anticipated as a separate future change.

#### Scenario: Same-operator chains are unambiguous
- **WHEN** `SELECT 1 UNION ALL SELECT 2 UNION ALL SELECT 3`, `SELECT 1 INTERSECT ALL SELECT 2 INTERSECT DISTINCT SELECT 3`, or any same-operator multi-leg chain is parsed
- **THEN** each level of the chain has the same operator pointer populated with the correct per-level mode, and round-trip through the formatter yields the same operator-and-modifier sequence

#### Scenario: Mixed-operator chains are NOT asserted to match ClickHouse precedence
- **WHEN** `SELECT 1 INTERSECT SELECT 2 UNION ALL SELECT 3` is parsed
- **THEN** `ParseStmts` returns no error (the SQL is syntactically accepted) but this change does NOT assert that the resulting AST corresponds to ClickHouse's `(SELECT 1 INTERSECT SELECT 2) UNION ALL SELECT 3` semantics; the right-recursive parse produces `SELECT 1 INTERSECT (SELECT 2 UNION ALL SELECT 3)`, and consumers MUST NOT rely on cross-operator precedence being correct until a follow-up change addresses it
