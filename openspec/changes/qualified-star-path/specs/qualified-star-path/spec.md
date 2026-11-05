# qualified-star-path Specification

## ADDED Requirements

### Requirement: A dotted identifier chain SHALL accept `*` as its last segment

The parser SHALL accept `*` as the final segment of a dotted identifier chain of three or
more parts (`db.table.*`) in expression position, producing a `*Path` whose last `Fields`
entry is an `*Ident` named `*`.

#### Scenario: Three-part qualified star
- **WHEN** `SELECT db.t.* FROM db.t` is parsed
- **THEN** `ParseStmts` returns no error AND the select item's `Expr` is a `*Path` with
  three fields whose last field has `Name` `*`

#### Scenario: Mixed with a two-part star
- **WHEN** `SELECT db.t.*, t.*, 1 FROM db.t AS t` is parsed
- **THEN** `ParseStmts` returns no error AND the first item is a `*Path` AND the second
  item is a `*NestedIdentifier` whose `DotIdent.Name` is `*`

#### Scenario: Qualified star as a function argument
- **WHEN** `SELECT count(db.t.*) FROM db.t` is parsed
- **THEN** `ParseStmts` returns no error

#### Scenario: A trailing member access is not absorbed into the chain
- **WHEN** `SELECT db.t.*.x FROM t` is parsed
- **THEN** the chain ends at the star: the item is an `*IndexOperation` whose `Object` is
  a `*Path` with three fields ending in `*` (the generic member-access handling accepts a
  trailing `.x` after any star, as it already did for `t.*.x`; ClickHouse rejects both)

### Requirement: Dotted chains without a star SHALL parse unchanged

`a.b.c` SHALL still produce a `*Path` of identifier fields, and `t.*` SHALL still produce a
`*NestedIdentifier`.

#### Scenario: Three-part identifier unchanged
- **WHEN** `SELECT a.b.c FROM t` is parsed
- **THEN** the select item is a `*Path` with three identifier fields, none named `*`

### Requirement: A qualified star SHALL round-trip through Format

`Format` SHALL emit the chain as `db.t.*` so that parse → format is a fixed point.

#### Scenario: Round trip keeps the qualified star
- **WHEN** `SELECT db.t.* FROM db.t` is parsed and formatted
- **THEN** the output is `SELECT db.t.* FROM db.t` AND formatting the re-parsed output
  yields the same text

### Requirement: Existing goldens SHALL be byte-identical

This change SHALL NOT alter the lexer, SHALL NOT add or rename an AST node or field, and
SHALL NOT change the two-part star or the integer-index branches. Every golden file that
existed before the change SHALL match without `-update`.

#### Scenario: Pre-existing goldens remain green
- **WHEN** goldens are regenerated after the change
- **THEN** no existing JSON, format or beautify golden file changes
