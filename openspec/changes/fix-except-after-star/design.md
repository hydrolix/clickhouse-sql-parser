# Design: EXCEPT After a Star Is Always a Column Modifier

## Context

`parseSelectItem` (`parser/parser_column.go`) parses the item expression, then loops over
modifiers. The `EXCEPT` arm is guarded by
`p.matchKeyword(KeywordExcept) && (p.peekTokenKind(TokenKindLParen) || p.peekTokenKind(TokenKindIdent))`.
`peekTokenKind` compares kinds strictly, while `matchTokenKind(TokenKindIdent)` and
`parseIdent` already accept keyword tokens as identifiers. `parseExceptExpr` consumes the
`EXCEPT` token through `parseIdent` (so the modifier's `Name` is the keyword itself), then
either one identifier or a parenthesised `parseFunctionParams` list.

`EXCEPT`, `UNION` and `INTERSECT` are select-item terminator keywords
(`isSelectItemTerminatorKeyword`), so when the modifier loop leaves `EXCEPT` alone the
alias logic does not take it, and `parseSelectQuery` consumes it as a set operator with an
optional `ALL`/`DISTINCT` and a right-hand side of `SELECT`, `WITH` or `(`.

`*` parses to `*Ident{Name: "*"}` (`parseColumnStar`); `COLUMNS(...)` parses to a
`*FunctionExpr` whose `Name.Name` is `COLUMNS`. `STRICT` is not a keyword; it lexes as an
identifier.

ClickHouse 26.8 (`clickhouse local`):

| Input | Result |
| --- | --- |
| `SELECT * EXCEPT timestamp FROM (SELECT 1 AS timestamp, 2 AS b)` | `2` |
| `SELECT * EXCEPT STRICT timestamp FROM …` / `EXCEPT STRICT (timestamp, key)` | ok |
| `SELECT COLUMNS('.*') EXCEPT timestamp FROM …` | ok |
| `SELECT * EXCEPT timestamp EXCEPT b FROM …` | ok |
| `SELECT * EXCEPT timestamp FROM … EXCEPT ALL SELECT 3` | ok (set op after FROM) |
| `SELECT * EXCEPT all FROM (SELECT 1 AS all)` | parses; `all` is the column |
| `SELECT * EXCEPT SELECT 1`, `SELECT * EXCEPT ALL SELECT 1` | syntax error |
| `SELECT * EXCEPT FROM (…)`, `SELECT * EXCEPT STRICT FROM (…)` | syntax error |
| `SELECT a EXCEPT b FROM …`, `SELECT a APPLY(sum) FROM …` | syntax error |
| `SELECT 1 EXCEPT SELECT 2`, `… EXCEPT ALL …`, `… EXCEPT DISTINCT …` | set operation |

## Goals / Non-Goals

**Goals:**

- Resolve `EXCEPT` by position like ClickHouse: modifier after a star-like item, set
  operator otherwise.
- Accept keyword-named columns and `STRICT`.
- Keep the AST shape and all existing goldens unchanged.

**Non-Goals:**

- `APPLY` / `REPLACE` after a non-star item (ClickHouse rejects it; the upstream fixture
  `select_item_with_modifiers.sql` contains `SELECT c0 REPLACE(c0 AS c1)` and keeping it
  byte-identical avoids upstream drift).
- Qualified stars (`t.*`, `db.t.*`); they do not parse today, independent of `EXCEPT`.
- Validating that the column list after `EXCEPT` names real columns.

## Decisions

### D1 — Guard `EXCEPT` on the item, not on the next token

Replace the token-kind guard with a star check on the parsed item:

```go
starLike := isStarLikeSelectExpr(expr) // *Ident{Name:"*"} or *FunctionExpr{Name:"COLUMNS"}
for {
    if starLike && p.matchKeyword(KeywordExcept) {
        // parseExceptExpr
    } else if p.matchKeyword(KeywordApply) || p.matchKeyword(KeywordReplace) {
        // unchanged
    } else { break }
}
```

- After a star-like item, `EXCEPT` is always consumed as a modifier, including when the
  next word is `SELECT`, `ALL` or `FROM`. ClickHouse behaves the same (those inputs are
  errors there as well; here the word becomes the column name and the statement then fails
  or parses as ClickHouse would).
- After any other item the loop breaks on `EXCEPT`; `EXCEPT` is a terminator keyword, so
  it is not taken as an alias, and `parseSelectQuery` parses the set operation. This is what
  makes `SELECT 1 EXCEPT (SELECT 2)` a set operation and `SELECT a EXCEPT b` an error.
- **Alternative — the ticket's guard** (also accept keyword tokens except `SELECT`,
  `WITH`, `ALL`, `DISTINCT`): fixes the reported query but keeps `EXCEPT(STRICT) AS ts`,
  keeps the modifier tree for `1 EXCEPT (SELECT 2)`, rejects `* EXCEPT all`, and still
  accepts `a EXCEPT b`. Rejected in favour of the position rule.

### D2 — `STRICT` lives in the modifier name

In `parseExceptExpr`, after the `EXCEPT` identifier: if the current token is an identifier
spelled `STRICT` (case-insensitive), consume it and set `Name.Name = "EXCEPT STRICT"` and
`Name.NameEnd` to the end of that token. Then, as today, parse either one identifier
(`parseIdent`, which accepts keyword tokens) or a parenthesised list.

- **Why in the name:** no new AST field. Adding `Strict bool` to `FunctionExpr` would
  serialise into every function node of every JSON golden; a dedicated modifier node would
  change the type of `SelectItem.Modifiers`, an exported field. Both rejected.
- `STRICT` is taken as the mode unconditionally, like ClickHouse (`* EXCEPT STRICT FROM`
  is an error there too). A column literally named `strict` needs the parenthesised or
  quoted form, as in ClickHouse.
- `Format` prints the modifier through `FunctionExpr`, so the output is
  `EXCEPT STRICT(col)`, which ClickHouse accepts.

### D3 — Error on a non-star `EXCEPT <ident>` comes from the set-operator path

`SELECT a EXCEPT b FROM t` now fails with `expected SELECT, WITH or (, got <ident>` from
`parseSelectQuery`. No dedicated message; the error already points at the token.

## Risks / Trade-offs

- [A consumer relied on `SELECT a EXCEPT b` parsing] → It never ran on ClickHouse; the
  corpus of 4,122 dashboard queries has no such statement. Changelog entry added.
- [`STRICT` as a bare column name] → Needs `EXCEPT (strict)` or a quoted identifier, same
  as ClickHouse.
- [Upstream sync touches `parseSelectItem`] → The change is a small guard swap plus a
  short branch in `parseExceptExpr`; worth offering upstream.

## Golden footprint

- Existing fixtures: no change in any golden family (verified by `git status` after
  `make update_test` showing only new files).
- New fixtures under `parser/testdata/query/`: `select_except_keyword_column.sql`
  (keyword-named columns, `COLUMNS(...) EXCEPT`, chained modifiers, modifier followed by a
  set operation after FROM) and `select_except_strict.sql` (bare and parenthesised
  `STRICT`), and under `parser/testdata/basic/`: `select_except_subquery_set_op.sql`
  (`SELECT 1 EXCEPT (SELECT 2)`), each with JSON, format and beautify goldens.
