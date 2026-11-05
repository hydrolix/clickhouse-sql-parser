# Tasks: `interval` as an Identifier

## 1. Baseline

- [x] 1.1 `make test` and `make lint` pass on `main` (`ba312ee`).
- [x] 1.2 ClickHouse 26.8 behaviour recorded in design.md Context.

## 2. Parser (D1)

- [x] 2.1 `parser/parser_column.go`: add `parseIntervalOrIdent()` that saves the lexer
      state, tries `parseInterval(true)`, and on error restores the state and returns
      `p.parseIdent()`. Place it next to `parseInterval`.
- [x] 2.2 `parseColumnExpr`: the `KeywordInterval` case calls `parseIntervalOrIdent()`.
      The comma / `AS` rule above the switch is unchanged.

## 3. Fixtures and goldens

- [x] 3.1 Add `parser/testdata/query/select_interval_as_identifier.sql`:
      `SELECT toStartOfInterval(ts, INTERVAL 1 MINUTE) AS interval, count() FROM t GROUP BY interval ORDER BY interval;`
      `SELECT interval FROM t WHERE interval > 0 ORDER BY interval DESC;`
      `SELECT (interval), interval + 1 FROM t;`
      `SELECT now() - INTERVAL 1 HOUR, INTERVAL (1) MINUTE, INTERVAL x DAY FROM t;`
- [x] 3.2 Run `make update_test`.
- [x] 3.3 `git status --porcelain parser/testdata` shows only the new fixture and its
      JSON / format / beautify goldens; no existing golden modified.

## 4. Inline tests

- [x] 4.1 `parser/parser_test.go`: table-driven `TestParser_With_IntervalIdentifier` with
      subtests named as the spec Scenarios: "interval alias in GROUP BY and ORDER BY",
      "interval in WHERE and ORDER BY DESC", "interval in select list and parentheses",
      "Interval literals unchanged" (asserts `*IntervalExpr`), "Malformed literal still
      errors" (`SELECT INTERVAL 1 FOO` returns an error), "Round trip keeps the alias"
      (`Format` of the first statement is a fixed point).

## 5. Changelog and verification

- [x] 5.1 `CHANGELOG.md`: under "## Unreleased" (create it if absent) add
      "- **Fix**: `interval` parses as an identifier wherever it is not an `INTERVAL <expr> <unit>` literal (`… AS interval GROUP BY interval ORDER BY interval`), as in ClickHouse".
- [x] 5.2 `gofmt`, `make lint`, `make test` and `go test -race ./... -count=1` pass.
- [x] 5.3 `openspec validate interval-identifier-fallback --strict` passes.
