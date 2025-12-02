# Changelog

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