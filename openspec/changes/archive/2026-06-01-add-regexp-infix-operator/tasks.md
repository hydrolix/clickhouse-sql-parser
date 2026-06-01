## 1. Baseline

- [x] 1.1 Confirm `parser/parser_test.go` contains `TestParser_REGEXP_Bare` exercising bare `REGEXP` in WHERE clauses. Starting state: FAIL.
- [x] 1.2 Confirm `parser/parser_test.go` contains `TestParser_With_REGEXP_Operators` exercising `REGEXP` alongside template variables and SETTINGS. Starting state: FAIL.
- [x] 1.3 Capture the baseline: `go test ./parser/... -run 'TestParser_REGEXP_Bare|TestParser_With_REGEXP_Operators|TestParser_InvalidSyntax|TestParser_ParseStatements|TestParser_Format|TestParser_FormatBeautify' -count=1`. The two REGEXP tests must FAIL; the rest must PASS.

## 2. Extend inline tests with NOT REGEXP

- [x] 2.1 In `parser/parser_test.go`, extend `TestParser_REGEXP_Bare`'s `validSQLs` slice with one additional SQL: `"SELECT * FROM t WHERE x NOT REGEXP 'foo'"`. Starting state of the new case: FAIL.

## 3. Implementation in `parser/parser_column.go`

- [x] 3.1 In `func (p *Parser) getNextPrecedence()`, locate the arm `case p.matchKeyword(KeywordBetween), p.matchKeyword(KeywordLike), p.matchKeyword(KeywordIlike):` (around line 70). Add `p.matchKeyword(KeywordRegexp)` to the predicate list so `REGEXP` returns `PrecedenceBetweenLike`.
- [x] 3.2 In `func (p *Parser) parseInfix(...)`, locate the long-form binary-operator arm that lists `TokenKind*` and `Keyword*` predicates (around lines 84-92). Add `p.matchKeyword(KeywordRegexp)` to the predicate list.
- [x] 3.3 In the same `parseInfix`, locate `case p.matchKeyword(KeywordNot):` (around line 171). Add `case p.matchKeyword(KeywordRegexp):` as a fourth case in the inner switch (empty body — the shared body below builds the operator string as `"NOT REGEXP"`).
- [x] 3.4 Update the default-arm error message in the same switch from `"expected IN, LIKE or ILIKE after NOT, got %s"` to `"expected IN, LIKE, ILIKE or REGEXP after NOT, got %s"`.
- [x] 3.5 `go build ./parser/...` to confirm compilation.

## 4. Add `.sql` fixture inputs

- [x] 4.1 Create `parser/testdata/query/select_regexp.sql` with the single line:
  ```
  SELECT a, b FROM t WHERE name REGEXP '^foo'
  ```
- [x] 4.2 Create `parser/testdata/query/select_case_when_regexp.sql` with the multi-line content (matching the existing fixture style — `SELECT` on its own line, indented body):
  ```
  SELECT
      CASE WHEN col REGEXP '^[0-9]+$' THEN toInt32(col) ELSE 0 END AS num_value
  FROM t
  ```
- [x] 4.3 Create `parser/testdata/query/select_not_regexp.sql` with the single line:
  ```
  SELECT a, b FROM t WHERE name NOT REGEXP '^foo'
  ```

## 5. Generate and inspect the goldens

- [x] 5.1 Run `go test ./parser/... -run 'TestParser_ParseStatements' -count=1 -update`. This creates `parser/testdata/query/output/select_regexp.sql.golden.json`, `select_case_when_regexp.sql.golden.json`, and `select_not_regexp.sql.golden.json`. **Visually inspect each generated JSON** to confirm the AST has the expected shape — `BinaryOperation` with `Operation: "REGEXP"` (or `"NOT REGEXP"`) and the literal pattern as the right-hand operand.
- [x] 5.2 Run `go test ./parser/... -run 'TestParser_Format' -count=1 -update`. This creates `parser/testdata/query/format/select_regexp.sql`, etc. **Visually inspect each generated `.sql`** to confirm the formatter renders `REGEXP` and `NOT REGEXP` correctly (with surrounding whitespace, no missing operands).
- [x] 5.3 Run `go test ./parser/... -run 'TestParser_FormatBeautify' -count=1 -update`. This creates the `format/beautify/` counterparts. **Visually inspect** them too — the beautifier may use a different layout than `Format`.
- [x] 5.4 Re-run all three commands without `-update`: `go test ./parser/... -run 'TestParser_ParseStatements|TestParser_Format|TestParser_FormatBeautify' -count=1`. All three SHOULD pass against the just-generated goldens. If anything fails immediately after generation, something is wrong with the fixture content — investigate before proceeding.

## 6. Verify the feature contract

- [x] 6.1 `go test ./parser/... -run 'TestParser_REGEXP_Bare' -v -count=1` → expect PASS for every case, including the `NOT REGEXP` case from task 2.1.
- [x] 6.2 `go test ./parser/... -run 'TestParser_With_REGEXP_Operators' -v -count=1` → expect PASS.

## 7. Verify regression guards

- [x] 7.1 `go test ./parser/... -run 'TestParser_InvalidSyntax' -v -count=1` → must PASS.
- [x] 7.2 `go test ./parser/... -run 'TestParser_ParseStatements' -count=1` → all goldens match (including the three new ones from section 5).
- [x] 7.3 `go test ./parser/... -run 'TestParser_Format' -count=1` → all formatter goldens match.
- [x] 7.4 `go test ./parser/... -run 'TestParser_FormatBeautify' -count=1` → all beautify goldens match.

## 8. Close out

- [x] 8.1 `go test ./parser/... -count=1`. Confirm the deltas from the previous full-suite baseline: `TestParser_REGEXP_Bare` (with the new NOT REGEXP case) and `TestParser_With_REGEXP_Operators` both transition FAIL → PASS; the three new golden sub-tests under `TestParser_ParseStatements`, `TestParser_Format`, `TestParser_FormatBeautify` are PASS. Nothing previously passing moves to fail.
- [x] 8.2 `go vet ./parser/...` produces no new warnings (the pre-existing `WriteByte` notice in `parser/format.go` is acceptable).
- [x] 8.3 `openspec validate add-regexp-infix-operator` reports the change as valid.
