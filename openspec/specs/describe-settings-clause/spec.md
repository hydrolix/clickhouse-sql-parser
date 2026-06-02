## Purpose

Recognise a trailing `SETTINGS key1=value1, key2=value2, …` clause on `DESCRIBE`/`DESC` statements (for example `DESCRIBE TABLE foo SETTINGS describe_compact_output=1`), so that dashboard tools and observability layers that introspect ClickHouse schemas with these settings — to ask for compact output, to include sub-columns, or to control type display — parse successfully instead of being rejected with `"<EOF> or ';' was expected, but got: SETTINGS"`. This change reuses the existing `SettingsClause` AST node and `tryParseSettingsClause` helper that already serve `SelectQuery`, `AlterTable`, and other statements; it adds one optional `Settings *SettingsClause` field to `DescribeStmt` and threads it through `End()`, `Accept()`, and `parseDescribeStmt`, without introducing any new token, keyword, or visitor method.

## Requirements

### Requirement: Parser SHALL accept a trailing `SETTINGS` clause on `DESCRIBE`

`parseDescribeStmt` SHALL consume an optional `SETTINGS key1=value1, key2=value2, …` clause after the table identifier. If `SETTINGS` is present, the resulting `*DescribeStmt` SHALL have its `Settings` field populated with the parsed `*SettingsClause`. If `SETTINGS` is absent, `Settings` SHALL be nil and the statement SHALL parse exactly as it does without this change.

#### Scenario: DESCRIBE TABLE with a single setting
- **WHEN** `DESCRIBE TABLE foo SETTINGS describe_compact_output=1` is parsed
- **THEN** `ParseStmts` returns no error AND the resulting `*DescribeStmt`'s `Settings` field is non-nil

#### Scenario: DESCRIBE (without TABLE) with multiple settings
- **WHEN** `DESCRIBE foo SETTINGS describe_compact_output=1, describe_include_subcolumns=1` is parsed
- **THEN** `ParseStmts` returns no error AND the resulting `*DescribeStmt`'s `Settings` field is non-nil and contains both setting entries

#### Scenario: DESCRIBE without SETTINGS still parses
- **WHEN** `DESCRIBE TABLE mytable` is parsed
- **THEN** `ParseStmts` returns no error AND the resulting `*DescribeStmt`'s `Settings` field is nil

### Requirement: `DescribeStmt.End()` SHALL reflect the SETTINGS clause when present

`(*DescribeStmt).End()` SHALL return `Settings.End()` when `Settings` is non-nil, and `Target.End()` otherwise. This mirrors the pattern used by other statements with optional trailing clauses (e.g. `SelectQuery.End()` for an optional FORMAT clause).

#### Scenario: End() after SETTINGS includes the settings clause
- **WHEN** `DESCRIBE foo SETTINGS k=v` is parsed and `DescribeStmt.End()` is read
- **THEN** the returned position is at or after the end of `v`, not at the end of `foo`

#### Scenario: End() without SETTINGS unchanged
- **WHEN** `DESCRIBE foo` is parsed and `DescribeStmt.End()` is read
- **THEN** the returned position is the end of `foo` (same as today)

### Requirement: `DescribeStmt.Accept()` SHALL traverse the SETTINGS sub-tree when present

`(*DescribeStmt).Accept(visitor)` SHALL invoke `Settings.Accept(visitor)` after `Target.Accept(visitor)` when `Settings` is non-nil, before calling `visitor.VisitDescribeExpr(d)`. When `Settings` is nil, the traversal proceeds as it does today (Target only, then VisitDescribeExpr).

#### Scenario: Visitor sees settings nodes when present
- **WHEN** a visitor traverses a `DescribeStmt` whose `Settings` is populated
- **THEN** the visitor's `VisitSettingsClause` (or any visit method on settings sub-nodes) is invoked at least once before `VisitDescribeExpr`

#### Scenario: Visitor traversal unchanged when SETTINGS absent
- **WHEN** a visitor traverses a `DescribeStmt` whose `Settings` is nil
- **THEN** no `VisitSettingsClause` is invoked, and `VisitDescribeExpr` is still called

### Requirement: SETTINGS in DESCRIBE SHALL be exercised end-to-end through the golden fixture suite

Two `.sql` fixtures SHALL be added under `parser/testdata/ddl/` covering single-setting and multi-setting DESCRIBE forms. Each fixture SHALL be exercised by all three golden test functions — `TestParser_ParseStatements` (JSON AST), `TestParser_Format` (compact SQL re-rendering), and `TestParser_FormatBeautify` (beautified SQL) — and the corresponding golden files SHALL be committed under `output/`, `format/`, and `format/beautify/` respectively.

#### Scenario: Single-setting fixture flows through all three goldens
- **WHEN** `parser/testdata/ddl/describe_table_with_settings.sql` containing `DESCRIBE TABLE foo SETTINGS describe_compact_output=1` is added
- **THEN** the corresponding three golden files exist and match without `-update`

#### Scenario: Multi-setting fixture flows through all three goldens
- **WHEN** `parser/testdata/ddl/describe_settings_multiple.sql` containing `DESCRIBE foo SETTINGS describe_compact_output=1, describe_include_subcolumns=1` is added
- **THEN** the corresponding three golden files exist and match without `-update`

### Requirement: Pre-existing DESCRIBE goldens SHALL be regenerated with a single-line shift

The two existing JSON goldens at `parser/testdata/ddl/output/describe_table_with_table_keyword.sql.golden.json` and `describe_table_without_table_keyword.sql.golden.json` SHALL gain exactly one new line each (`"Settings": null`) as a direct consequence of adding the new struct field. The accompanying `format/` and `format/beautify/` goldens for those two fixtures SHALL remain byte-identical (the formatter does not emit anything for a nil `Settings` field).

#### Scenario: JSON goldens shift by exactly one line
- **WHEN** `TestParser_ParseStatements/describe_table_with_table_keyword.sql` is run against the post-change parser
- **THEN** the only diff against the pre-change golden is exactly one added line containing `"Settings": null`

#### Scenario: Format and beautify goldens unchanged
- **WHEN** `TestParser_Format/describe_table_with_table_keyword.sql` and `TestParser_FormatBeautify/describe_table_with_table_keyword.sql` are run against the post-change parser
- **THEN** both goldens match byte-for-byte without `-update`

### Requirement: Existing parser, AST, and unrelated golden behaviour SHALL be preserved

This change SHALL NOT alter the lexer, SHALL NOT rename `DescribeStmt` or any of its existing fields, SHALL NOT introduce or rename any visitor method, SHALL NOT modify `parseTableIdentifier` or `tryParseSettingsClause`, and SHALL NOT cause any golden-file fixture outside the DESCRIBE family to drift.

#### Scenario: Non-DESCRIBE goldens unchanged
- **WHEN** the full golden suite is run after this change
- **THEN** every golden file outside `parser/testdata/ddl/describe_*` and `parser/testdata/ddl/{format,format/beautify,output}/describe_*` matches byte-for-byte without `-update`

#### Scenario: TestParser_InvalidSyntax unchanged
- **WHEN** `TestParser_InvalidSyntax` is run after this change
- **THEN** the test passes with the same set of error inputs that pass today
