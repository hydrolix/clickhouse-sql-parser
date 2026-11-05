# Design: WITH-Prefixed Subqueries in Expressions

## Context

`parseColumnExpr` (`parser/parser_column.go`) dispatches on the current token. Two arms
detect a subquery:

```go
case p.matchKeyword(KeywordSelect):
    return p.parseSelectQuery(pos)
…
case p.matchTokenKind(TokenKindLParen):
    if peek, _ := p.lexer.peekToken(); peek != nil {
        if peek.Kind == TokenKindKeyword && strings.EqualFold(peek.String, KeywordSelect) {
            return p.parseSubQuery(pos)
        }
    }
    return p.parseFunctionParams(p.Pos())
```

`parseSubQuery` (`parser/parser_query.go:957`) consumes an optional `(`, calls
`parseSelectQuery` and expects the matching `)`. `parseSelectQuery` already accepts a
query that starts with `SELECT`, `WITH` or `(`. The table path (`parser_table.go:361`)
and the top-level statement path already treat `WITH` as a query start.

ClickHouse 26.8 (`clickhouse local`):

| Input | Result |
| --- | --- |
| `SELECT (WITH 1000 AS latest SELECT max(x) FROM … WHERE y > latest) AS m` | ok |
| `SELECT (WITH x AS (SELECT 1 AS a) SELECT a FROM x)` | ok |
| `SELECT 1 IN (WITH 1 AS a SELECT a)` | ok |
| `WITH (WITH 1 AS a SELECT a) AS b SELECT b` | ok |
| `SELECT exists(WITH 1 AS a SELECT a)` | ok |
| `SELECT ( (WITH 1 AS a SELECT a) )` | ok |
| `SELECT * FROM (WITH 1 AS a SELECT a)` | ok (already parses in the fork) |

## Goals / Non-Goals

**Goals:**

- A WITH-prefixed subquery parses in every expression position where a SELECT-prefixed one
  does, producing the same node kind.
- No change to existing goldens.

**Non-Goals:**

- Any change to `parseSelectQuery`, `parseSubQuery`, CTE parsing or the table path.
- Accepting `with` as a bare column name without backticks.

## Decisions

### D1 — Treat `WITH` like `SELECT` at both detection points

- Parenthesised: the `TokenKindLParen` arm calls `parseSubQuery` when the peeked token is
  the keyword `SELECT` **or** `WITH`. Use `p.peekKeyword(KeywordSelect) || p.peekKeyword(KeywordWith)`,
  which already exists and reads better than the manual peek.
- Bare: `case p.matchKeyword(KeywordSelect), p.matchKeyword(KeywordWith):` returns
  `p.parseSelectQuery(pos)`.

The node kinds stay what `SELECT` produces today in each spot (`*SubQuery` with parens,
`*SelectQuery` bare), so no formatter, visitor or walker change is needed.

- **Alternative — backtracking try-parse** (attempt a subquery, else function params):
  unnecessary; `WITH` cannot start anything but a query in expression position, and the
  lookahead is one token. Rejected.

## Risks / Trade-offs

- [`with` as an unquoted column name] → Previously read as an identifier, then failed on
  the following token in practice; now a parse error at the query grammar instead.
  ClickHouse requires backticks for it as well. Accepted.
- [Upstream sync touches these arms] → Two one-line changes; worth offering upstream.

## Golden footprint

- Existing fixtures: no change in any golden family.
- New fixture `parser/testdata/query/select_with_prefixed_subquery.sql` with JSON, format
  and beautify goldens.
