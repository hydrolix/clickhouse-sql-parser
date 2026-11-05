# Design: Aliases in `extract(...)` Arguments

## Context

`parseColumnExpr` dispatches `case p.matchKeyword(KeywordExtract)` to
`parseColumnExtractExpr` (`parser/parser_column.go:304`). That routine consumes `EXTRACT (`
and loops: `parseExpr` for a parameter; if the result is an `*Ident` naming an interval
unit and the next token is `FROM`, `parseExtractFrom` builds an `*IntervalFrom{Interval,
FromPos, FromExpr}`; the parameter is appended; a comma continues the loop. It returns
`*ExtractExpr{ExtractPos, ExtractEnd, Parameters}`. `ExtractExpr.FormatSQL` prints
`EXTRACT(` then the parameters joined by `, `.

Ordinary function parameters are parsed by `parseColumnsExpr`, which is `parseExpr` plus
an optional `AS` + `parseIdent`, returning `*ColumnExpr{Expr, Alias}`.
`ColumnExpr.FormatSQL` prints `expr AS alias`.

ClickHouse 26.8 (`clickhouse local`):

| Input | Result |
| --- | --- |
| `SELECT extract(arrayJoin(['a1','b2']) AS p, '(\d+)'), p` | `1  a1` |
| `SELECT plus(1 AS y, 2), y` | `3  1` |
| `SELECT EXTRACT(YEAR FROM toDate('2026-11-05') AS d), d` | `2026  2026-11-05` |
| `SELECT extract('a12', '\d+')` | `12` |
| `SELECT extract(month FROM now())` | ok |

## Goals / Non-Goals

**Goals:**

- `AS alias` after any `extract(...)` parameter parses and round-trips.
- No change for parameters without an alias.

**Non-Goals:**

- Changing how `EXTRACT` is dispatched, or merging it with the generic function path.
- Attaching the alias of `unit FROM expr AS alias` to the inner expression; it is attached
  to the whole parameter, which formats to the same text.

## Decisions

### D1 — Accept an optional alias after each parameter and wrap in `ColumnExpr`

In the loop of `parseColumnExtractExpr`, once the parameter (`param` for the `FROM` form or
`expr` otherwise) is known:

```go
if p.tryConsumeKeywords(KeywordAs) {
    alias, err := p.parseIdent()
    if err != nil {
        return nil, err
    }
    param = &ColumnExpr{Expr: param, Alias: alias}
}
```

- **Why `ColumnExpr`:** it is exactly what `parseColumnsExpr` produces for an aliased
  ordinary function argument, so formatter, visitor and walker already handle it and the
  AST stays uniform across `f(x AS y)` and `extract(x AS y, p)`.
- **Why not reuse `parseColumnsExpr` for the whole parameter:** the `unit FROM` detection
  needs the raw `*Ident`; wrapping first would hide it.
- **Alternative — route `extract(` to the generic function path unless the first
  parameter is `<unit> FROM`:** larger change to dispatch, different node kind
  (`FunctionExpr`) for the regex form, and golden churn for the existing
  `select_extract_with_regex` fixture. Rejected.

## Risks / Trade-offs

- [Alias attached to the `IntervalFrom` parameter rather than its inner expression] →
  Formats identically; no consumer inspects the inner alias. Accepted.
- [Upstream sync touches the loop] → Six lines; worth offering upstream.

## Golden footprint

- Existing fixtures: no change in any golden family.
- New fixture `parser/testdata/query/select_extract_with_alias.sql` with JSON, format and
  beautify goldens.
