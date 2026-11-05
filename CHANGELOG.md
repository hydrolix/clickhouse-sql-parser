# Changelog

## Unreleased

- **Fix**: an argument of `extract(...)` may carry `AS alias` (`extract(arrayJoin(x) AS part, '(\d+)')`), as in ClickHouse
- **Fix**: a subquery that starts with `WITH` parses in expression position (scalar subquery, `IN (…)`, function argument, `WITH … AS name` item), like one that starts with `SELECT`

## 0.2.0

- Support the single-quote escaping rule
- Support string concatenate operator ||

## 0.1.1

- **Fix**: Fix module name in go.mod

## 0.1.0

add support for Grafana macros, $$ blocks, and other enhancements

- Added support for Grafana variables/macros (e.g., $__timeFilter)
- Added support for aliases that start with numbers
- Supported `$$` as text blocks
- Supported comments starting with `#`
- Supported `DESCRIBE` statement
- Allowed `NOT IN` after `GLOBAL`
- Fixed parsing of `extract()` function
- Fixed parsing of `EXCEPT` statement