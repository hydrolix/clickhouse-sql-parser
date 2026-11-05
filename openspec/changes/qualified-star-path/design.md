# Design: Three-Part Qualified Star `db.table.*`

## Context

`parseIdentOrFunction` (`parser/parser_table.go:402`) parses an identifier and then
dispatches on the next token. After a dot it switches on the following token:

- identifier: loop `parseIdent`, append to `fields`, continue while another dot follows;
  return `*Path{Fields: fields}`;
- `*`: `parseColumnStar`, return `*NestedIdentifier{Ident, DotIdent}`;
- integer: `*IndexOperation`.

So `t.*` works and `a.b.c` works, but `db.t.*` enters the identifier loop and fails when
`parseIdent` meets `*`. `parseColumnStar` returns an `*Ident{Name: "*"}`.
`Path.FormatSQL` joins the fields with `.`; `Walk` / `Accept` visit each field.

ClickHouse 26.8 (`clickhouse local`):

| Input | Result |
| --- | --- |
| `SELECT db.t.* FROM db.t` | ok |
| `SELECT db.t.* EXCEPT a FROM db.t` | ok |
| `SELECT t.*, 3 FROM … AS t`, `count(t.*)`, `(t.*)`, `t.* + 1` | ok |

## Goals / Non-Goals

**Goals:**

- `db.t.*` (and any longer chain ending in `*`) parses and round-trips.
- `a.b.c` and `t.*` keep their nodes.

**Non-Goals:**

- Changing the two-part `t.*` from `*NestedIdentifier` to `*Path`; that would churn
  goldens and consumers for no gain.
- Rejecting a member access after the star. `db.t.*.x` is accepted by the generic postfix
  handling in `parseColumnExpr` as an `*IndexOperation` over the `Path`, exactly as
  `t.*.x` already is; ClickHouse rejects both. Tightening that is a separate change.

## Decisions

### D1 — Accept `*` as the last segment of the `Path` loop

Inside the identifier branch of the dot switch, after consuming a dot, check for `*`
before calling `parseIdent`:

```go
for {
    if p.matchTokenKind("*") {
        star, err := p.parseColumnStar(p.Pos())
        if err != nil {
            return nil, err
        }
        fields = append(fields, star)
        break
    }
    child, err := p.parseIdent()
    …
}
```

- **Why `Path`:** it is the node the three-part chain already produces; the star is just
  its last field, represented by the same `*Ident{Name:"*"}` the two-part form uses as
  `DotIdent`. No formatter, visitor or walker change.
- **Why break after the star:** nothing may follow a star in a qualified name; a trailing
  dot then fails in the caller, as today for other malformed items.
- **Alternative — a new `QualifiedStar` node:** cleaner conceptually but adds an AST type
  that every consumer must learn, for a form the corpus uses three times. Rejected.

## Risks / Trade-offs

- [Two shapes for qualified stars (`NestedIdentifier` for two parts, `Path` for three or
  more)] → pre-existing asymmetry of the parser for dotted names in general; consumers that
  need "is this a star item" check both, as `fix-except-after-star` does. Accepted.

## Golden footprint

- Existing fixtures: no change in any golden family.
- New fixture `parser/testdata/query/select_qualified_star_three_part.sql` with JSON,
  format and beautify goldens.
