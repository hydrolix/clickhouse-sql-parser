# Tasks: EXCEPT After a Star Is Always a Column Modifier

## 1. Baseline

- [x] 1.1 `make test` and `make lint` pass on `main` (`ba312ee`).
- [x] 1.2 ClickHouse 26.8 behaviour for the ambiguous forms recorded in design.md Context.

## 2. Parser (D1, D2)

- [x] 2.1 `parser/parser_column.go`: add `isStarLikeSelectExpr(expr Expr) bool`
      (`*Ident` with `Name == "*"`, or `*FunctionExpr` whose `Name.Name` equals `COLUMNS`
      case-insensitively).
- [x] 2.2 `parseSelectItem`: replace the `EXCEPT` guard with `starLike && p.matchKeyword(KeywordExcept)`.
      Leave the `APPLY` / `REPLACE` branch as is.
- [x] 2.3 `parseExceptExpr`: after the `EXCEPT` identifier, consume an identifier spelled
      `STRICT` (case-insensitive) as the mode: `Name.Name = "EXCEPT STRICT"`, `Name.NameEnd`
      = end of the `STRICT` token. Then one identifier (`parseIdent`) or a parenthesised
      list, as today.

## 3. Fixtures and goldens

- [x] 3.1 Add `parser/testdata/query/select_except_keyword_column.sql`:
      `SELECT timestamp, * EXCEPT timestamp FROM t;`, `SELECT * EXCEPT date FROM t;`,
      `SELECT * EXCEPT key FROM t;`, `SELECT COLUMNS('^a') EXCEPT timestamp FROM t;`,
      `SELECT * EXCEPT timestamp EXCEPT b FROM t;`,
      `SELECT * EXCEPT timestamp FROM t EXCEPT ALL SELECT 3;`
- [x] 3.2 Add `parser/testdata/query/select_except_strict.sql`:
      `SELECT * EXCEPT STRICT timestamp FROM t;`, `SELECT * EXCEPT STRICT (timestamp, key) FROM t;`
- [x] 3.3 Add `parser/testdata/basic/select_except_subquery_set_op.sql`:
      `SELECT 1 EXCEPT (SELECT 2)`
- [x] 3.4 Run `make update_test`.
- [x] 3.5 `git status --porcelain parser/testdata` shows only the new fixtures and their
      JSON / format / beautify goldens; no existing golden modified. In the JSON golden of
      3.3 the statement has `Except` set and the item `1` has no `Modifiers`.

## 4. Inline tests

- [x] 4.1 `parser/parser_test.go`: table-driven `TestParser_With_ExceptAfterStar` with
      subtests "Keyword-named column", "STRICT bare and list", "COLUMNS then EXCEPT",
      "Chained EXCEPT modifiers", "Set operator matrix still parses as set operation"
      (asserts `SelectQuery.Except != nil` and no modifiers for the 3 EXCEPT forms),
      "Parenthesised subquery after a non-star item is a set operation",
      "Modifier then set operation after FROM", "Non-star item rejects EXCEPT ident"
      (`SELECT a EXCEPT b FROM t` errors), "STRICT round-trips through Format"
      (`SELECT * EXCEPT STRICT(timestamp) FROM t` is a fixed point).
- [x] 4.2 Keep `TestParser_With_ExceptIdent` and `TestParser_With_SetOperators` green
      unchanged.

## 5. Changelog and verification

- [x] 5.1 `CHANGELOG.md`: add an "Unreleased" section on top with a Fix entry for
      HDX-12821 naming the behaviour change for `SELECT <non-star> EXCEPT <ident>`.
- [x] 5.2 `gofmt`, `goimports`, `make lint`, `make test` and `go test -race ./... -count=1`
      pass.
- [x] 5.3 `openspec validate fix-except-after-star --strict` passes.
