# Tasks: Three-Part Qualified Star `db.table.*`

## 1. Baseline

- [x] 1.1 `make test` and `make lint` pass on `main` (`ba312ee`).
- [x] 1.2 ClickHouse 26.8 behaviour recorded in design.md Context.

## 2. Parser (D1)

- [x] 2.1 `parser/parser_table.go` `parseIdentOrFunction`: in the identifier branch of the
      dot switch, before `parseIdent` inside the loop, consume a `*` via `parseColumnStar`
      as the final field and break. The two-part `*` branch and the integer branch are
      unchanged.

## 3. Fixtures and goldens

- [x] 3.1 Add `parser/testdata/query/select_qualified_star_three_part.sql`:
      `SELECT db.t.* FROM db.t;`
      `SELECT db.t.*, t.*, 1 FROM db.t AS t;`
      `SELECT count(db.t.*) FROM db.t;`
      `SELECT a.b.c FROM t;`
- [x] 3.2 Run `make update_test`.
- [x] 3.3 `git status --porcelain parser/testdata` shows only the new fixture and its
      JSON / format / beautify goldens; no existing golden modified.

## 4. Inline tests

- [x] 4.1 `parser/parser_test.go`: table-driven `TestParser_With_QualifiedStarPath` with
      subtests named as the spec Scenarios: "Three-part qualified star" (asserts the select
      item is a `*Path` with three fields whose last `Name` is `*`), "Mixed with a two-part
      star" (asserts `t.*` is still a `*NestedIdentifier`), "Qualified star as a function
      argument", "Three-part identifier unchanged" (`a.b.c` is a `*Path` with three
      identifier fields, none `*`), "A trailing member access is not absorbed into the chain" (`SELECT db.t.*.x FROM t`
      is an `*IndexOperation` over a 3-field `*Path` ending in `*`), "Round trip keeps the qualified star" (`Format` of the first
      statement is a fixed point).

## 5. Changelog and verification

- [x] 5.1 `CHANGELOG.md`: under "## Unreleased" (create it if absent, directly under the
      "# Changelog" title) add
      "- **Fix**: a three-part qualified star `db.table.*` parses in the select list and in function arguments, like `table.*`".
- [x] 5.2 `gofmt`, `make lint`, `make test` and `go test -race ./... -count=1` pass.
- [x] 5.3 `openspec validate qualified-star-path --strict` passes.
