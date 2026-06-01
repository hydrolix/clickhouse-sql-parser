## Context

`DescribeStmt` already exists in `parser/ast.go` (line ~6289). It carries the `DESC`/`DESCRIBE` position, a derived `StatementEnd`, an optional `DescribeType` string for the `TABLE` keyword, and a `Target *TableIdentifier` for the table being described. Today's `parseDescribeStmt` in `parser/parser_table.go` (line ~1649) consumes `DESC`/`DESCRIBE`, optional `TABLE`, then a `TableIdentifier` — and stops. Anything after the identifier (semicolon, EOF, or any other token) is the responsibility of the statement-list parser.

The same codebase already has a fully-built `SettingsClause` AST node (line ~2752 in `ast.go`) and a `tryParseSettingsClause(pos Pos) (*SettingsClause, error)` helper (line ~1227 in `parser_table.go`). The helper's contract:
- If the current token is NOT `KeywordSettings`, return `(nil, nil)` — caller treats this as "no settings".
- If the current token IS `KeywordSettings`, consume it and delegate to `parseSettingsClause`, returning either the clause or a parser error.

This means the extension to `parseDescribeStmt` is a one-liner-plus-end-update: after parsing the table identifier, call the helper and capture the result.

## Goals / Non-Goals

**Goals:**
- `DESCRIBE [TABLE] foo SETTINGS k1=v1, k2=v2, …` parses successfully and produces a `DescribeStmt` whose `Settings` field is a populated `*SettingsClause`.
- `DESCRIBE [TABLE] foo` (without SETTINGS) continues to parse exactly as today and produces a `DescribeStmt` whose `Settings` is `nil`.
- `DescribeStmt.End()` returns the position after the SETTINGS clause when present; otherwise the position after the table identifier (today's behaviour).
- `DescribeStmt.Accept(visitor)` traverses the new sub-tree when present, otherwise behaves as today.

**Non-Goals:**
- A new AST node for DESCRIBE-with-SETTINGS. Reusing `DescribeStmt` and making the field optional is sufficient.
- A new visitor method. `VisitDescribeExpr` is the existing hook; consumers that already implement it pick up the settings sub-tree automatically via the `Accept` traversal.
- Validation of which setting names are valid for DESCRIBE. ClickHouse will reject unsupported settings at execution time; the parser stays syntactically permissive.
- Renaming `DescribeStmt` to anything else, or introducing a parallel `DescribeQuery` node. The existing name is fine, the existing visitor method is fine, and renaming would break consumers and force regeneration of every DESCRIBE golden.

## Decisions

### Decision 1: Extend `DescribeStmt` rather than introduce a new AST node

Adding `Settings *SettingsClause` as an optional field is the smallest viable change. The `DescribeStmt` shape stays the same for the SETTINGS-absent case; the field is nil and downstream code that doesn't know about it ignores it.

**Why:** No consumer that already pattern-matches `*DescribeStmt` needs to change. No visitor that already implements `VisitDescribeExpr` needs to change. The existing two golden fixtures (`describe_table_with_table_keyword.sql`, `describe_table_without_table_keyword.sql`) keep their tests intact; only the JSON-snapshot goldens shift by one line (see Decision 4).

**Alternative considered:** Introduce a new `DescribeQuery` AST node with `Expr Expr` instead of `Target *TableIdentifier`, plus a new `VisitDescribeQuery` visitor method. **Rejected.** It would:
- Delete `DescribeStmt`, breaking every consumer of `VisitDescribeExpr`.
- Force regeneration of every DESCRIBE golden (not just a single-line shift).
- Diverge from the rest of the AST, which uses `*Stmt` naming for top-level statements.

### Decision 2: Use the existing `tryParseSettingsClause` helper

`tryParseSettingsClause` was specifically designed for this pattern — call it where an optional SETTINGS clause might appear, get `(nil, nil)` if absent, get `(*SettingsClause, nil)` if present, get `(nil, err)` only if SETTINGS is present and malformed.

**Why:** Consistency with `parseSelectQuery`, `parseAlterTable`, and every other place that accepts a trailing SETTINGS. Reusing the helper means the SETTINGS syntax inside DESCRIBE is identical to the SETTINGS syntax inside SELECT — no surprises for SQL authors.

**Implication:** A malformed SETTINGS clause after DESCRIBE (e.g. `DESCRIBE TABLE foo SETTINGS =`) produces the same parser error as it would inside SELECT, propagated up from `parseDescribeStmt`.

### Decision 3: `End()` and `Accept()` follow the established "optional trailing clause" pattern

The codebase has many examples of statements with optional trailing clauses (e.g. `SelectQuery` with its `Format` clause). The convention is:
- `End()` returns the position after the most-recently-trailing clause that is present.
- `Accept()` traverses the optional clause only when it's non-nil, then calls the visitor method.

`DescribeStmt` follows the same pattern.

### Decision 4: Accept the JSON-golden change for the two existing DESCRIBE fixtures

The current JSON goldens for DESCRIBE statements render every field explicitly, including `null` for nil pointers (e.g. `Target.Database` shows `"Database": null`). Adding `Settings *SettingsClause` will add `"Settings": null` to both existing goldens.

The alternatives (`json:"-"` or `json:",omitempty"` tag) would each compromise the AST snapshot. `json:"-"` makes the field invisible in the JSON dump, which is misleading. `json:",omitempty"` would deviate from the rest of `DescribeStmt`'s rendering convention (where nil sub-fields are shown explicitly).

The third option — leave the tag untagged and let the field render — is the consistent choice. The trade-off is that the two existing goldens shift by exactly one line each. The change is trivially reviewable: the diff is one `"Settings": null,` added near the end of each JSON.

**Workflow:** After implementing the AST change, re-run `TestParser_ParseStatements -update` once. Visually inspect the diff of the two updated goldens. Confirm it is exactly one new line per file. Commit.

### Decision 5: Add new golden fixtures for the SETTINGS path

The inline test `TestParser_With_DescribeSettings` exercises parse-only. To lock in the format and beautify behaviour, two new `.sql` fixtures are added under `parser/testdata/ddl/`:

- `describe_table_with_settings.sql` — single-setting case.
- `describe_settings_multiple.sql` — multiple settings, with and without the `TABLE` keyword to cover both forms.

Each generates the standard three goldens (output/, format/, format/beautify/). Visual inspection of the generated formats is mandatory.

## Risks / Trade-offs

- **Risk: A consumer that constructs `*DescribeStmt` literals will fail to compile because the new field is required.** Go struct literals with named fields don't break when new fields are added — they just leave the new field zero-valued. Positional struct literals would break, but they're discouraged for non-trivial structs and not present in this repo. **Mitigation:** none required.
- **Risk: The two existing goldens shift unexpectedly.** Mitigated by Decision 4's explicit workflow: regenerate, visually verify the one-line diff, commit.
- **Risk: A user writes `DESCRIBE foo SETTINGS k=v FORMAT JSON` expecting FORMAT after SETTINGS.** Today's `parseDescribeStmt` does not handle a FORMAT clause. After this change, FORMAT after SETTINGS still fails — same as before for non-SETTINGS DESCRIBE. FORMAT support is out of scope.
- **Trade-off: We do NOT match origin/main's `DescribeQuery` design.** Stated in Decision 1's "Alternative considered". The cost of matching is too high (delete + recreate + regenerate every golden) for the benefit (lockstep with a divergent fork's design).

## Migration Plan

Single commit, no dependencies, no data or config involvement. Rollback is `git revert`. The two existing DESCRIBE goldens shift by one line each — those diffs are part of the commit.
