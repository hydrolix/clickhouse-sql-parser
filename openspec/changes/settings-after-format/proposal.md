# SETTINGS After FORMAT, and Two Upstream Expression Fixes

## Why

chproxy (the Hydrolix http-proxy) parses every cacheable query with this parser. It does
this to build cache keys, to remove proxy-only settings (`use_query_cache`,
`hdx_use_query_cache`) before forwarding (HDX-12511), and to detect non-deterministic
functions (HDX-12521). A production corpus captured on 2026-10-02 (642 queries) has 22
queries the parser rejects although ClickHouse ran them:

- **4 put `SETTINGS` after `FORMAT`.** For example:
  `SELECT … ORDER BY n DESC FORMAT CSVWithNames SETTINGS hdx_query_max_timerange_sec = 86400`.
  ClickHouse accepts a `SETTINGS` clause after the output `FORMAT`. Neither this fork nor
  upstream `AfterShip/clickhouse-sql-parser` (master `3d7c9d2`, 2026-09-24) parses it.
- **9 use an operator expression inside a table-function argument.** For example:
  `FROM numbers(toUInt32(dateDiff('hour', a, b) + 1))`. Upstream fixed this in `0d9ef25`
  (#298).
- **9 are cut off at 4,000 characters** by the logging that captured them. These are
  correctly rejected.

When a query fails to parse, chproxy can't strip the proxy-only settings from it. The
cluster then rejects `hdx_use_query_cache` as an unknown setting.

A full upstream sync (54 upstream commits; conflicts in 6 code files and about 100
goldens) is tracked separately. This change takes only what chproxy needs now.

## What Changes

- **`SETTINGS` after `FORMAT`.** `SelectQuery` gains an additive field `FormatSettings
  *SettingsClause`: the `SETTINGS` clause that follows the `FORMAT` clause. `Format`
  re-emits it after `FORMAT`, so round-tripping keeps the original order. `Walk` and
  `Accept` visit it. The existing `Settings` field keeps meaning "`SETTINGS` before
  `FORMAT`".
- **Cherry-pick upstream `0d9ef25`** "Fix parsing of operator expressions in table function
  arguments (#298)": `numbers(x + 1)`, `numbers(toUInt32(f(…) + 1))` and similar now parse.
- **Cherry-pick upstream `dd867a1`** "Fix signed number after ')' or ']' being lexed as a
  literal (#286/#287)": `(1)-1` and `arr[1]-1` lex `-` as a binary operator.
- **Release a new fork tag that doesn't collide with upstream tag names.** Upstream already
  uses `v0.5.1` through `v0.5.6`, and the fork's `v0.5.1`/`v0.5.2` point at different
  commits. The name is decided in tasks.

## Capabilities

### New Capabilities

- `settings-after-format`: parsing, formatting and traversal of a `SETTINGS` clause that
  follows `FORMAT`.
- `upstream-expression-fixes`: the two cherry-picked upstream fixes, pinned by tests in
  the fork.

### Modified Capabilities

None.

## Impact

- **AST:** one additive field. Every JSON golden that renders a `SelectQuery` gains one
  line (`"FormatSettings": null`). Goldens are regenerated with `make update_test`.
- **Consumers:** chproxy must also read `FormatSettings` when selecting the chain-wide
  clause. That is part of the chproxy change that bumps the dependency, not this repo.
- **Upstream sync:** the sync ticket must drop the two cherry-picks (they're already
  upstream) and carry `FormatSettings` forward.
