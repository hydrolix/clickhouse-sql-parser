# Tasks: WITH-Prefixed Subqueries in Expressions

## 1. Baseline

- [x] 1.1 `make test` and `make lint` pass on `main` (`ba312ee`).
- [x] 1.2 ClickHouse 26.8 behaviour recorded in design.md Context.

## 2. Parser (D1)

- [x] 2.1 `parser/parser_column.go` `parseColumnExpr`: the `TokenKindLParen` arm calls
      `p.parseSubQuery(pos)` when `p.peekKeyword(KeywordSelect) || p.peekKeyword(KeywordWith)`;
      otherwise `parseFunctionParams` as today.
- [x] 2.2 Same function: the bare `case p.matchKeyword(KeywordSelect):` becomes
      `case p.matchKeyword(KeywordSelect), p.matchKeyword(KeywordWith):` and still returns
      `p.parseSelectQuery(pos)`.

## 3. Fixtures and goldens

- [x] 3.1 Add `parser/testdata/query/select_with_prefixed_subquery.sql`:
      `SELECT (WITH 1000 AS latest SELECT max(x) FROM t WHERE y > latest) AS m;`
      `SELECT (WITH x AS (SELECT 1 AS a) SELECT a FROM x);`
      `SELECT 1 IN (WITH 1 AS a SELECT a);`
      `WITH (WITH 1 AS a SELECT a) AS b SELECT b;`
      `SELECT exists(WITH 1 AS a SELECT a);`
- [x] 3.2 Run `make update_test`.
- [x] 3.3 `git status --porcelain parser/testdata` shows only the new fixture and its
      JSON / format / beautify goldens; no existing golden modified.

## 4. Inline tests

- [x] 4.1 `parser/parser_test.go`: table-driven `TestParser_With_WithPrefixedSubquery` with
      subtests named as the spec Scenarios: "Scalar subquery with a constant alias"
      (asserts the select item's `Expr` is a `*SubQuery` with `HasParen` true and a non-nil
      `Select.With`), "Scalar subquery with a CTE", "WITH-prefixed subquery as IN operand",
      "WITH-prefixed subquery as a WITH item", "WITH-prefixed subquery as a function argument"
      (asserts the argument is a `*SelectQuery`), "SELECT-prefixed subqueries unchanged"
      (`SELECT (SELECT 1)`, `SELECT 1 IN (SELECT 1)`, `SELECT exists(SELECT 1)` parse),
      "Round trip is a fixed point" (`Format` of the first statement re-parses to the same text).

## 5. Changelog and verification

- [x] 5.1 `CHANGELOG.md`: under "## Unreleased" (create it if absent, directly under the
      "# Changelog" title) add
      "- **Fix**: a subquery that starts with `WITH` parses in expression position (scalar subquery, `IN (…)`, function argument, `WITH … AS name` item), like one that starts with `SELECT`".
- [x] 5.2 `gofmt`, `make lint`, `make test` and `go test -race ./... -count=1` pass.
- [x] 5.3 `openspec validate with-prefixed-subquery-expr --strict` passes.
