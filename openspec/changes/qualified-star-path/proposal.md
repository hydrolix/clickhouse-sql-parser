# Three-Part Qualified Star `db.table.*`

## Why

`SELECT db.t.* FROM db.t` fails with `expected the last token kind is: <ident>, but got *`.
Three queries in the dashboard SQL corpus (EAA Apps / Origins / Username Observability)
select `akamai.eaa_deviceinventory.*`, and ClickHouse runs them.

The two-part form `t.*` already parses: `parseIdentOrFunction` sees `ident . *` and builds a
`*NestedIdentifier{Ident, DotIdent: *}`. A longer dotted chain (`a.b.c`) is built by a loop
that calls `parseIdent` after every dot and returns a `*Path{Fields}`; that loop never
checks for `*`, so `db.t.*` dies on the third segment. ClickHouse accepts `db.t.*` wherever
`t.*` is accepted (verified on ClickHouse 26.8).

## What Changes

- **A dotted chain MAY end in `*`.** After a dot inside the `Path` loop, a `*` token is
  consumed as the final segment, producing a `*Path` whose last `Fields` entry is an
  `*Ident` named `*`, the same identifier node `parseColumnStar` already produces.
- Chains without a star, and the two-part `t.*`, are unchanged.
- `Format` re-emits `db.t.*` through the existing `Path` formatter.

## Capabilities

### New Capabilities

- `qualified-star-path`: three-or-more-part qualified star `db.table.*`.

### Modified Capabilities

None.

## Impact

- **AST:** no new node or field. `db.t.*` is a `*Path` with fields `db`, `t`, `*`.
- **Goldens:** every existing JSON, format and beautify golden is byte-identical. New
  fixtures add new golden files only.
- **Consumers:** chproxy and the Grafana plugin need only a dependency bump. The three
  corpus queries in `clickhouse-sql-parser-dashboards/plain-sql` turn green.
- **Related:** the `fix-except-after-star` change treats a `*Path` ending in `*` as
  star-like so `db.t.* EXCEPT a` works once both land.
