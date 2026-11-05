# Tasks: Aliases in `extract(...)` Arguments

## 1. Baseline

- [x] 1.1 `make test` and `make lint` pass on `main` (`ba312ee`).
- [x] 1.2 ClickHouse 26.8 behaviour recorded in design.md Context.

## 2. Parser (D1)

- [x] 2.1 `parser/parser_column.go` `parseColumnExtractExpr`: after a parameter is
      determined (both the `unit FROM expr` branch and the plain branch), accept an optional
      `AS` + `parseIdent` and wrap the parameter in `&ColumnExpr{Expr: param, Alias: alias}`.
      Restructure the branch so the alias check runs once for both forms; keep the
      "EXTRACT requires at least one parameter" error and the `ExtractEnd` position logic.

## 3. Fixtures and goldens

- [x] 3.1 Add `parser/testdata/query/select_extract_with_alias.sql`:
      `SELECT avg(toUInt32OrNull(extract(arrayJoin(splitByString('//', s)) AS part, '(\d+)'))) AS v, part FROM t;`
      `SELECT extract(s AS src, '\d+') AS n, src FROM t;`
      `SELECT EXTRACT(YEAR FROM d AS day), day FROM t;`
      `SELECT extract(s, '\d+'), EXTRACT(MONTH FROM d) FROM t;`
- [x] 3.2 Run `make update_test`.
- [x] 3.3 `git status --porcelain parser/testdata` shows only the new fixture and its
      JSON / format / beautify goldens; no existing golden modified.

## 4. Inline tests

- [x] 4.1 `parser/parser_test.go`: table-driven `TestParser_With_ExtractArgAlias` with
      subtests named as the spec Scenarios: "Alias on a regex extract argument" (asserts the
      first parameter is a `*ColumnExpr` whose `Alias.Name` is `part`), "Alias on the
      haystack argument", "Alias inside EXTRACT unit FROM expr" (asserts the parameter is a
      `*ColumnExpr` wrapping an `*IntervalFrom`), "Unaliased parameters unchanged" (asserts
      the parameter kinds for `extract(s, '\d+')` and `EXTRACT(MONTH FROM d)` are not
      `*ColumnExpr`), "Round trip keeps the alias" (`Format` of the first statement is a
      fixed point containing `AS part`).

## 5. Changelog and verification

- [x] 5.1 `CHANGELOG.md`: under "## Unreleased" (create it if absent, directly under the
      "# Changelog" title) add
      "- **Fix**: an argument of `extract(...)` may carry `AS alias` (`extract(arrayJoin(x) AS part, '(\d+)')`), as in ClickHouse".
- [x] 5.2 `gofmt`, `make lint`, `make test` and `go test -race ./... -count=1` pass.
- [x] 5.3 `openspec validate extract-arg-alias --strict` passes.
