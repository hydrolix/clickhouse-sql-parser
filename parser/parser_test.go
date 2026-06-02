package parser

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebdah/goldie/v2"
	"github.com/stretchr/testify/require"
)

var runCompatible = flag.Bool("compatible", false, "run compatible test")

func TestParser_Compatible(t *testing.T) {
	if !*runCompatible {
		t.Skip("Compatible test runs only if -compatible is set")
	}
	dir := "./testdata/query/compatible/1_stateful"
	entries, err := os.ReadDir(dir)
	if err != nil {
		require.NoError(t, err)
	}

	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		t.Run(entry.Name(), func(t *testing.T) {
			fileBytes, err := os.ReadFile(filepath.Join(dir, entry.Name()))
			require.NoError(t, err)
			parser := Parser{
				lexer: NewLexer(string(fileBytes)),
			}
			_, err = parser.ParseStmts()
			require.NoError(t, err)
		})
	}
}

func TestParser_ParseStatements(t *testing.T) {
	for _, dir := range []string{"./testdata/dml", "./testdata/ddl", "./testdata/query", "./testdata/basic"} {
		outputDir := dir + "/output"
		entries, err := os.ReadDir(dir)
		if err != nil {
			require.NoError(t, err)
		}
		for _, entry := range entries {
			if !strings.HasSuffix(entry.Name(), ".sql") {
				continue
			}
			t.Run(entry.Name(), func(t *testing.T) {
				fileBytes, err := os.ReadFile(filepath.Join(dir, entry.Name()))
				require.NoError(t, err)
				parser := Parser{
					lexer: NewLexer(string(fileBytes)),
				}
				stmts, err := parser.ParseStmts()
				require.NoError(t, err)
				outputBytes, _ := json.MarshalIndent(stmts, "", "  ")
				g := goldie.New(t,
					goldie.WithNameSuffix(".golden.json"),
					goldie.WithDiffEngine(goldie.ColoredDiff),
					goldie.WithFixtureDir(outputDir))
				g.Assert(t, entry.Name(), outputBytes)
			})
		}
	}
}

func TestParser_Format(t *testing.T) {
	for _, dir := range []string{"./testdata/dml", "./testdata/ddl", "./testdata/query", "./testdata/basic"} {
		outputDir := dir + "/format"

		entries, err := os.ReadDir(dir)
		if err != nil {
			require.NoError(t, err)
		}
		for _, entry := range entries {
			if !strings.HasSuffix(entry.Name(), ".sql") {
				continue
			}
			t.Run(entry.Name(), func(t *testing.T) {
				fileBytes, err := os.ReadFile(filepath.Join(dir, entry.Name()))
				require.NoError(t, err)
				parser := Parser{
					lexer: NewLexer(string(fileBytes)),
				}
				stmts, err := parser.ParseStmts()
				require.NoError(t, err)
				var builder strings.Builder
				builder.WriteString("-- Origin SQL:\n")
				builder.Write(fileBytes)
				builder.WriteString("\n\n-- Format SQL:\n")
				var formatSQLBuilder strings.Builder
				for _, stmt := range stmts {
					formatSQLBuilder.WriteString(Format(stmt))
					formatSQLBuilder.WriteByte(';')
					formatSQLBuilder.WriteByte('\n')
				}
				formatSQL := formatSQLBuilder.String()
				builder.WriteString(formatSQL)
				validFormatSQL(t, formatSQL)
				g := goldie.New(t,
					goldie.WithNameSuffix(""),
					goldie.WithDiffEngine(goldie.ColoredDiff),
					goldie.WithFixtureDir(outputDir))
				g.Assert(t, entry.Name(), []byte(builder.String()))
			})
		}
	}
}

func TestParser_FormatBeautify(t *testing.T) {
	for _, dir := range []string{"./testdata/dml", "./testdata/ddl", "./testdata/query", "./testdata/basic"} {
		outputDir := dir + "/format/beautify"

		entries, err := os.ReadDir(dir)
		if err != nil {
			require.NoError(t, err)
		}
		for _, entry := range entries {
			if !strings.HasSuffix(entry.Name(), ".sql") {
				continue
			}
			t.Run(entry.Name(), func(t *testing.T) {
				fileBytes, err := os.ReadFile(filepath.Join(dir, entry.Name()))
				require.NoError(t, err)
				parser := Parser{
					lexer: NewLexer(string(fileBytes)),
				}
				stmts, err := parser.ParseStmts()
				require.NoError(t, err)
				var builder strings.Builder
				builder.WriteString("-- Origin SQL:\n")
				builder.Write(fileBytes)
				builder.WriteString("\n\n-- Beautify SQL:\n")
				for _, stmt := range stmts {
					formatter := NewFormatter()
					formatter.WithBeautify()
					formatter.WriteExpr(stmt)
					builder.WriteString(formatter.String())
					builder.WriteByte(';')
					builder.WriteByte('\n')
				}
				g := goldie.New(t,
					goldie.WithNameSuffix(""),
					goldie.WithDiffEngine(goldie.ColoredDiff),
					goldie.WithFixtureDir(outputDir))
				g.Assert(t, entry.Name(), []byte(builder.String()))
			})
		}
	}
}

// validFormatSQL Verify that the format sql can be re-parsed with consistent results
func validFormatSQL(t *testing.T, sql string) {
	parser := NewParser(sql)
	stmts, err := parser.ParseStmts()
	require.NoError(t, err)
	var builder strings.Builder
	for _, stmt := range stmts {
		builder.WriteString(Format(stmt))
		builder.WriteByte(';')
		builder.WriteByte('\n')
	}
	require.Equal(t, sql, builder.String())
}

func TestParser_InvalidSyntax(t *testing.T) {
	invalidSQLs := []string{
		"SELECT * FROM",
		// WITH FILL error cases
		"SELECT n FROM t ORDER BY n WITH",                             // WITH without FILL
		"SELECT n FROM t ORDER BY n FILL",                             // FILL without WITH
		"SELECT n FROM t ORDER BY n WITH FILL FROM",                   // FROM without value
		"SELECT n FROM t ORDER BY n WITH FILL TO",                     // TO without value
		"SELECT n FROM t ORDER BY n WITH FILL STEP",                   // STEP without value
		"SELECT n FROM t ORDER BY n WITH FILL STALENESS",              // STALENESS without value
		"SELECT n FROM t ORDER BY n WITH FILL INTERPOLATE (x",         // Missing closing paren
		"SELECT n FROM t ORDER BY n WITH FILL INTERPOLATE x AS x + 1", // Missing parens around column list
		"ALTER TABLE foo_mv MODIFY QUERY AS SELECT * FROM baz",        // MODIFY QUERY followed by an invalid query
		// Invalid ARRAY JOIN types (only ARRAY JOIN, LEFT ARRAY JOIN, and INNER ARRAY JOIN are valid)
		"SELECT * FROM t RIGHT ARRAY JOIN arr AS a", // RIGHT ARRAY JOIN not supported
		"SELECT * FROM t FULL ARRAY JOIN arr AS a", // FULL ARRAY JOIN not supported
	}
	for _, sql := range invalidSQLs {
		parser := NewParser(sql)
		_, err := parser.ParseStmts()
		require.Error(t, err, "Expected error for SQL: %s", sql)
	}
}

// The following tests are copied verbatim from origin/main (commits
// ea58695, 19d34a6, e8bf340) to surface which fork features the upstream
// parser already supports.

func TestParser_ConditionALL_With_Variables(t *testing.T) {
	validSQLs := []string{
		//"SELECT 1 FROM table WHERE statusCode ${a} (1,2)",
		//"SELECT toString(statusCode) as HTTP_Status_Code, $__timeInterval(${timefilter}) as time, ${count} as http FROM ${table} WHERE $__timeFilter(${timefilter}) AND $__conditionalAll( statusCode IN (${statusCode:sqlstring}), $statusCode)",
		"SELECT toString(statusCode) as HTTP_Status_Code, $__timeInterval(${timefilter}) as time, ${count} as http FROM ${table} WHERE $__timeFilter(${timefilter}) AND $__conditionalAll( statusCode ${AND_statusCode} (${statusCode:sqlstring}), $statusCode)",
	}
	for _, sql := range validSQLs {
		parser := NewParser(sql)
		expr, err := parser.ParseStmts()
		require.NoError(t, err, "Failed to parse: %s", sql)
		marshal, _ := json.Marshal(expr)
		fmt.Printf("%s", marshal)
	}
}

func TestParser_With_String_Concat_Operators(t *testing.T) {
	validSQLs := []string{
		"SELECT\n  'buc' + 'ket' \n    FROM\n      ${table}\n    WHERE\n      $__timeFilter(${timestamp})",
		"SELECT\n  'buc' || 'ket' \n    FROM\n      ${table}\n    WHERE\n      $__timeFilter(${timestamp})",
	}
	for _, sql := range validSQLs {
		println(sql)
		parser := NewParser(sql)
		expr, err := parser.ParseStmts()
		require.NoError(t, err, "Failed to parse: %s", sql)
		require.NotNil(t, expr)
		marshal, _ := json.Marshal(expr)
		fmt.Printf("%s\n", marshal)
	}
}

func TestParser_With_REGEXP_Operators(t *testing.T) {
	validSQLs := []string{
		"SELECT toString(statusCode) as HTTP_Status_Code, $__timeInterval(reqTimeSec) as time, count(*) as http\nFROM ${table}\nWHERE $__timeFilter(${timestamp})\nAND $__adHocFilter()\nAND UA REGEXP '(AI2Bot|Amazon-Q-Bot|anthropic-ai|Applebot-Extended|Bytespider|ChatGPT-User|Claude(Bot|-Web)|cohere-ai|DatabricksBot|Google-CloudVertexBot|Google-Extended|GPTBot|Meta-ExternalAgent|meta-externalagent|MistralBot|OAI-SearchBot|PerplexityBot|Quora-Bot|SeekrBot|xAI-Bot|YandexTMCore|YouBot)'\nGROUP BY HTTP_Status_Code, time ORDER BY time\nSETTINGS hdx_query_max_execution_time=60, hdx_query_admin_comment='akamai - statuscode - ${__user.login}'",
	}
	for _, sql := range validSQLs {
		println(sql)
		parser := NewParser(sql)
		expr, err := parser.ParseStmts()
		require.NoError(t, err, "Failed to parse: %s", sql)
		marshal, _ := json.Marshal(expr)
		fmt.Printf("%s\n", marshal)
	}
}

// The following tests fill in coverage gaps surfaced by the porting analysis
// in .claude/PORTING_NOTES.md. They are NOT verbatim copies from origin/main —
// they're synthesised positive tests to give per-feature signal about what
// upstream supports vs. what still needs porting.

// Fork feature: DESCRIBE … SETTINGS …  (DescribeQuery node carries optional Settings).
// Upstream parses bare `DESCRIBE TABLE foo` but the SETTINGS suffix is fork-only.
func TestParser_With_DescribeSettings(t *testing.T) {
	validSQLs := []string{
		"DESCRIBE TABLE foo SETTINGS describe_compact_output=1",
		"DESCRIBE foo SETTINGS describe_compact_output=1, describe_include_subcolumns=1",
	}
	for _, sql := range validSQLs {
		parser := NewParser(sql)
		_, err := parser.ParseStmts()
		require.NoError(t, err, "Failed to parse: %s", sql)
	}
}

// Fork feature: GLOBAL NOT IN — origin/main relaxed the parseInfix GLOBAL branch
// to accept NOT IN in addition to IN. Upstream's testdata only covers GLOBAL IN.
func TestParser_With_GlobalNotIn(t *testing.T) {
	validSQLs := []string{
		"SELECT * FROM t WHERE x GLOBAL NOT IN (SELECT y FROM remote('127.0.0.1', s))",
		"SELECT * FROM t WHERE x GLOBAL IN (SELECT y FROM remote('127.0.0.1', s))",
	}
	for _, sql := range validSQLs {
		parser := NewParser(sql)
		_, err := parser.ParseStmts()
		require.NoError(t, err, "Failed to parse: %s", sql)
	}
}

// Fork feature: SELECT * EXCEPT col  (single-ident form, as opposed to EXCEPT (col)).
// Upstream supports the parenthesized form (testdata/query/select_item_with_modifiers.sql)
// but not the bare-ident form added by parseExceptExpr.
func TestParser_With_ExceptIdent(t *testing.T) {
	validSQLs := []string{
		"SELECT * EXCEPT col FROM t",
		"SELECT * EXCEPT (col) FROM t",
		"SELECT * REPLACE(i + 1 AS i) EXCEPT colX APPLY(sum) FROM t",
	}
	for _, sql := range validSQLs {
		parser := NewParser(sql)
		_, err := parser.ParseStmts()
		require.NoError(t, err, "Failed to parse: %s", sql)
	}
}

// Fork feature: $variable as a value in SETTINGS clause
// (origin/main added a `case p.matchVariable()` branch in parseSettingsExprList).
func TestParser_With_VariableInSettings(t *testing.T) {
	validSQLs := []string{
		"SELECT 1 FROM t SETTINGS max_threads=$threads",
		"SELECT 1 FROM t SETTINGS max_threads=${threads}, max_memory_usage=${mem}",
	}
	for _, sql := range validSQLs {
		parser := NewParser(sql)
		_, err := parser.ParseStmts()
		require.NoError(t, err, "Failed to parse: %s", sql)
	}
}

// Regression guard against the fork's deletion of the inline EXTRACT case from
// parseColumnExpr. Both the function-call form (extract(col, regex)) and the
// SQL special form (EXTRACT(unit FROM expr)) must remain parseable.
// If this test starts failing after porting parser_column.go from origin/main,
// the deletion of the `case p.matchKeyword(KeywordExtract)` arm has regressed
// upstream's existing fixtures (select_extract_with_regex.sql,
// select_window_comprehensive.sql).
func TestParser_ExtractStillParses(t *testing.T) {
	validSQLs := []string{
		"SELECT extract('foo bar', 'b.*') FROM t",
		"SELECT EXTRACT(HOUR FROM ts) FROM t",
		"SELECT EXTRACT(DAY FROM ts) FROM t",
	}
	for _, sql := range validSQLs {
		parser := NewParser(sql)
		_, err := parser.ParseStmts()
		require.NoError(t, err, "EXTRACT form regressed: %s", sql)
	}
}

// Bare per-feature tests — these decompose the four conflated copied tests
// above (ConditionALL_With_Variables, With_SubSelect, With_REGEXP_Operators,
// With_String_Concat_Operators) into one-feature-per-test versions. The
// conflated tests are kept as integration smoke tests; the bare ones below
// give a clean pass/fail signal for each fork delta.

// `${var}` used as an expression operand (top-level position, not inside a
// function-call argument list).
func TestParser_Var_TopLevel(t *testing.T) {
	validSQLs := []string{
		"SELECT 1 FROM t WHERE x = ${y}",
		"SELECT ${a} FROM t",
	}
	for _, sql := range validSQLs {
		_, err := NewParser(sql).ParseStmts()
		require.NoError(t, err, "Failed to parse: %s", sql)
	}
}

// `${var}` used as a table name in the FROM clause.
func TestParser_Var_InFromClause(t *testing.T) {
	validSQLs := []string{
		"SELECT 1 FROM ${tbl}",
		"SELECT 1 FROM ${tbl} WHERE x = 1",
	}
	for _, sql := range validSQLs {
		_, err := NewParser(sql).ParseStmts()
		require.NoError(t, err, "Failed to parse: %s", sql)
	}
}

// `${var:format}` — variable with a format suffix (e.g. `:sqlstring`, `:json`).
func TestParser_Var_FormatSuffix(t *testing.T) {
	validSQLs := []string{
		"SELECT 1 FROM t WHERE x = ${y:sqlstring}",
		"SELECT ${a:json} FROM t",
	}
	for _, sql := range validSQLs {
		_, err := NewParser(sql).ParseStmts()
		require.NoError(t, err, "Failed to parse: %s", sql)
	}
}

// `${var}` used as an argument inside a function call. This is the
// failure mode that breaks all four conflated tests on upstream — the
// parser bails at `{` inside the argument list even though top-level
// `${var}` already lexes correctly.
func TestParser_Var_InFunctionArg(t *testing.T) {
	validSQLs := []string{
		"SELECT foo(${a}) FROM t",
		"SELECT toStartOfInterval(${ts}, INTERVAL 1 hour) FROM t",
		"SELECT $__timeFilter(${ts}) FROM t",
	}
	for _, sql := range validSQLs {
		_, err := NewParser(sql).ParseStmts()
		require.NoError(t, err, "Failed to parse: %s", sql)
	}
}

// `${VAR}` used as an infix operator between two operands —
// e.g. `statusCode ${AND_statusCode} (1,2)` in Grafana templating.
func TestParser_Var_AsInfixOperator(t *testing.T) {
	validSQLs := []string{
		"SELECT 1 FROM t WHERE a ${OP} b",
		"SELECT 1 FROM t WHERE statusCode ${AND_statusCode} (1, 2)",
	}
	for _, sql := range validSQLs {
		_, err := NewParser(sql).ParseStmts()
		require.NoError(t, err, "Failed to parse: %s", sql)
	}
}

// `$ident` — bare dollar-prefixed identifier, no braces (e.g. `$statusCode`).
// Upstream already supports this through its existing `case '$'` in
// `consumeToken` that falls through to `consumeIdent`.
func TestParser_Var_BareDollarIdent(t *testing.T) {
	validSQLs := []string{
		"SELECT $col FROM t",
		"SELECT $col FROM t WHERE $other = 1",
	}
	for _, sql := range validSQLs {
		_, err := NewParser(sql).ParseStmts()
		require.NoError(t, err, "Failed to parse: %s", sql)
	}
}

// `REGEXP` as an infix operator, isolated from Grafana macros.
func TestParser_REGEXP_Bare(t *testing.T) {
	validSQLs := []string{
		"SELECT * FROM t WHERE x REGEXP 'foo'",
		"SELECT * FROM t WHERE x REGEXP '(a|b)'",
		"SELECT count() FROM t WHERE name REGEXP 'Bot' GROUP BY name",
		"SELECT * FROM t WHERE x NOT REGEXP 'foo'",
	}
	for _, sql := range validSQLs {
		_, err := NewParser(sql).ParseStmts()
		require.NoError(t, err, "Failed to parse: %s", sql)
	}
}

// `+` used as a string-concatenation operator. (ClickHouse interprets `+`
// on strings as concat; the parser side just needs to accept it as a normal
// infix operator on string literals, which upstream already does.)
func TestParser_StringConcat_Plus(t *testing.T) {
	validSQLs := []string{
		"SELECT 'a' + 'b' FROM t",
		"SELECT 'foo' + 'bar' + 'baz' FROM t",
	}
	for _, sql := range validSQLs {
		_, err := NewParser(sql).ParseStmts()
		require.NoError(t, err, "Failed to parse: %s", sql)
	}
}

// `||` as a string-concatenation operator. Needs the fork's `TokenKindConcat`
// addition to the lexer and the matching `parseInfix` branch.
func TestParser_StringConcat_DoublePipe(t *testing.T) {
	validSQLs := []string{
		"SELECT 'a' || 'b' FROM t",
		"SELECT 'foo' || 'bar' || 'baz' FROM t",
	}
	for _, sql := range validSQLs {
		_, err := NewParser(sql).ParseStmts()
		require.NoError(t, err, "Failed to parse: %s", sql)
	}
}

func TestParser_Dashboard_Queries(t *testing.T) {
	t.Skip() //skip test
	fail := 0
	success := 0
	err := filepath.WalkDir("path to folder with sql files", func(path string, d fs.DirEntry, e error) error {
		if e != nil {
			t.Fail()
		}

		if strings.HasSuffix(path, ".sql") {
			t.Run(path, func(t *testing.T) {
				content, err := os.ReadFile(path)
				require.NoError(t, err)
				parser := NewParser(string(content))
				println(string(content))
				_, err = parser.ParseStmts()
				if err != nil {
					fail++
				} else {
					success++
				}
				require.NoError(t, err)

			})
		}
		return nil
	})
	require.NoError(t, err)
	println("success", success)
	println("fail", fail)
}

// Positive coverage for the broadened DESCRIBE-target grammar. Each SQL flips
// from FAIL on the previous narrow `parseTableIdentifier` path to PASS on the
// `parseTableExpr` path. Ordering matches design.md's "Acceptance surface"
// table so a reviewer reading test output sees one category before moving to
// the next. Three regression cases (`DESCRIBE TABLE foo`, `DESCRIBE foo
// SETTINGS …`, `DESCRIBE db.foo`) lock the shapes that already parsed today.
func TestParser_Describe_RichArguments(t *testing.T) {
	validSQLs := []string{
		"DESCRIBE (SELECT 1)",
		"DESCRIBE (SELECT a, b FROM inner_table) AS subq",
		"DESCRIBE foo AS f",
		"DESCRIBE db.foo AS f",
		"DESCRIBE numbers(10)",
		"DESCRIBE remote('host', db.foo)",
		"DESCRIBE foo FINAL",
		"DESCRIBE foo SETTINGS describe_compact_output=1",
		"DESCRIBE TABLE foo",
		"DESCRIBE db.foo",
		"DESCRIBE (SELECT 1) SETTINGS describe_compact_output=1",
		"DESCRIBE (SELECT * FROM foo JOIN bar ON foo.x = bar.x)",
	}
	for _, sql := range validSQLs {
		_, err := NewParser(sql).ParseStmts()
		require.NoError(t, err, "Failed to parse: %s", sql)
	}
}

// ClickHouse rejects bare JOIN at the DESCRIBE position — JOIN must be
// wrapped in a subquery. This test locks the parser's matching rejection so
// a future "let's just use parseJoinExpr" refactor cannot silently
// re-introduce a parser-vs-server gap.
func TestParser_Describe_RejectsJoin(t *testing.T) {
	invalidSQLs := []string{
		"DESCRIBE foo JOIN bar ON foo.x = bar.x",
		"DESCRIBE foo LEFT JOIN bar ON foo.x = bar.x",
	}
	for _, sql := range invalidSQLs {
		_, err := NewParser(sql).ParseStmts()
		require.Error(t, err, "Expected DESCRIBE-with-JOIN to fail: %s", sql)
	}
}

// Regression guard for matchVariable(): a backtick-quoted identifier whose
// body starts with `$` (e.g. `` `$col` ``) must be treated as an ordinary
// identifier, not as a Grafana template variable. Before the QuoteType
// guard was added to matchVariable, the lexer-stripped String field made
// `` `$col` `` look like a `$`-prefixed variable, which threw it onto the
// infix-operator branch in parseInfix.
func TestParser_Var_BacktickQuotedIdent_IsNotVariable(t *testing.T) {
	stmts, err := NewParser("SELECT `$col` FROM t").ParseStmts()
	require.NoError(t, err)
	require.Len(t, stmts, 1)

	sel, ok := stmts[0].(*SelectQuery)
	require.True(t, ok, "expected *SelectQuery, got %T", stmts[0])
	require.Len(t, sel.SelectItems, 1)

	ident, ok := sel.SelectItems[0].Expr.(*Ident)
	require.True(t, ok, "expected projected expression to be *Ident, got %T (matchVariable misclassified the backtick-quoted ident as a variable)", sel.SelectItems[0].Expr)
	require.Equal(t, "$col", ident.Name)
	require.Equal(t, BackTicks, ident.QuoteType)
}

// Companion test: a backtick-quoted `$`-prefixed ident on the left side of a
// comparison must be the LeftExpr of a BinaryOperation, not consumed as the
// binary operator itself. Pre-fix, getNextPrecedence returned PrecedenceIdent
// for `` `$col` `` and parseInfix swallowed it as the operator token,
// producing a malformed AST.
func TestParser_Var_BacktickQuotedIdent_InComparison(t *testing.T) {
	stmts, err := NewParser("SELECT 1 FROM t WHERE `$col` = 1").ParseStmts()
	require.NoError(t, err)
	require.Len(t, stmts, 1)

	sel, ok := stmts[0].(*SelectQuery)
	require.True(t, ok, "expected *SelectQuery, got %T", stmts[0])
	require.NotNil(t, sel.Where)

	bin, ok := sel.Where.Expr.(*BinaryOperation)
	require.True(t, ok, "expected WHERE to be *BinaryOperation, got %T", sel.Where.Expr)
	require.Equal(t, TokenKind("="), bin.Operation)

	left, ok := bin.LeftExpr.(*Ident)
	require.True(t, ok, "expected LeftExpr to be *Ident, got %T", bin.LeftExpr)
	require.Equal(t, "$col", left.Name)
	require.Equal(t, BackTicks, left.QuoteType)
}

// Direct-unit coverage for matchVariable() across the five quoting modes
// that flow through the lexer. We drive each token manually so the check
// is isolated from any downstream parser branch.
func TestMatchVariable_QuotingMatrix(t *testing.T) {
	cases := []struct {
		name string
		sql  string
		want bool
	}{
		{"bare $ident", "$col", true},
		{"braced ${tbl}", "${tbl}", true},
		{"plain ident", "col", false},
		{"backtick `$col`", "`$col`", false},
		{"double-quoted \"$col\"", `"$col"`, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := NewParser(tc.sql)
			// Prime the lexer so p.last() points at the first token.
			require.NoError(t, p.lexer.consumeToken())
			require.Equal(t, tc.want, p.matchVariable(), "matchVariable() mismatch for %q", tc.sql)
		})
	}
}
