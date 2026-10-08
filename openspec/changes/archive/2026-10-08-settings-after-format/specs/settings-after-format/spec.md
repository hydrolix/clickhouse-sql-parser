# settings-after-format Specification

## ADDED Requirements

### Requirement: SelectQuery SHALL expose FormatSettings for a SETTINGS clause after FORMAT

The `SelectQuery` struct SHALL expose `FormatSettings *SettingsClause`, placed
immediately after `Format`. It holds the `SETTINGS` clause that follows the
`FORMAT` clause. The existing `Settings` field SHALL keep holding only the
`SETTINGS` clause before `FORMAT`. No existing field SHALL be renamed,
removed or reordered.

#### Scenario: SETTINGS after FORMAT parses
- **WHEN** `SELECT 1 FORMAT JSON SETTINGS max_threads = 1` is parsed
- **THEN** `ParseStmts` returns no error AND `Format` is non-nil AND
  `FormatSettings` has one item `max_threads` AND `Settings` is nil

#### Scenario: Production query with SETTINGS after FORMAT
- **WHEN** `SELECT count() AS n FROM hydro.logs WHERE timestamp >= now() - toIntervalHour(1) GROUP BY app ORDER BY n DESC FORMAT CSVWithNames SETTINGS hdx_query_max_timerange_sec = 86400, hdx_query_max_execution_time = 60`
  is parsed
- **THEN** `ParseStmts` returns no error AND `FormatSettings` has two items

#### Scenario: SETTINGS before FORMAT is unchanged
- **WHEN** `SELECT 1 SETTINGS max_threads = 1 FORMAT JSON` is parsed
- **THEN** `Settings` has one item AND `FormatSettings` is nil

#### Scenario: Both positions are kept
- **WHEN** `SELECT 1 SETTINGS a = 1 FORMAT JSON SETTINGS b = 2` is parsed
- **THEN** `Settings` holds `a` AND `FormatSettings` holds `b`

#### Scenario: Trailing leg of a set operation owns it
- **WHEN** `SELECT 1 UNION ALL SELECT 2 FORMAT JSON SETTINGS a = 1` is parsed
- **THEN** the trailing leg's `FormatSettings` holds `a` AND the head's
  `FormatSettings` is nil

#### Scenario: SETTINGS without FORMAT still cannot repeat
- **WHEN** `SELECT 1 SETTINGS a = 1 SETTINGS b = 2` is parsed
- **THEN** `ParseStmts` returns an error

#### Scenario: Positions cover the clause
- **WHEN** `SELECT 1 FORMAT JSON SETTINGS a = 'x'` is parsed
- **THEN** `FormatSettings.SettingsPos` is the offset of the `SETTINGS`
  keyword AND `StatementEnd` is at or after the clause's last item

### Requirement: FormatSettings SHALL round-trip through Format and be traversed

`Format` and `Beautify` SHALL emit `FormatSettings` after the `FORMAT`
clause, so that parse → format is a fixed point for SQL with a `SETTINGS`
clause after `FORMAT`. `Walk` and `SelectQuery.Accept` SHALL visit
`FormatSettings`.

#### Scenario: Round trip keeps the order
- **WHEN** `SELECT 1 FORMAT JSON SETTINGS max_threads = 1` is parsed and formatted
- **THEN** the output is `SELECT 1 FORMAT JSON SETTINGS max_threads=1` AND
  formatting the re-parsed output yields the same text

#### Scenario: Walk visits the clause
- **WHEN** `parser.Walk` runs over `SELECT 1 FORMAT JSON SETTINGS a = 7`
- **THEN** the walk function receives the `*SettingsClause`, the
  `*SettingExpr` and the `*NumberLiteral` for `7`
