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
		// A sorting key is a single expression, so a key over several columns
		// is written as a tuple. A comma starts the next engine clause in
		// CREATE TABLE and the next clause in ALTER TABLE.
		"CREATE TABLE t (a DateTime, b String) ENGINE = MergeTree ORDER BY a, b",
		"ALTER TABLE t MODIFY ORDER BY a, b",
		// A broken column value must report an error, not stop the parser
		"CREATE TABLE t (x String DEFAULT CAST(a +, 'String'))",
		"CREATE TABLE t (x String MATERIALIZED a +)",
		"CREATE TABLE t (x String ALIAS a +)",
		// Invalid ARRAY JOIN types (only ARRAY JOIN, LEFT ARRAY JOIN, and INNER ARRAY JOIN are valid)
		"SELECT * FROM t RIGHT ARRAY JOIN arr AS a", // RIGHT ARRAY JOIN not supported
		"SELECT * FROM t FULL ARRAY JOIN arr AS a",  // FULL ARRAY JOIN not supported
		// ARRAY JOIN reads a column list, so it has no locality — in any spelling,
		// since modifiers keep their source case
		"SELECT * FROM t GLOBAL ARRAY JOIN arr AS a",
		"SELECT * FROM t LOCAL LEFT ARRAY JOIN arr AS a",
		"SELECT * FROM t GLOBAL array JOIN arr AS a",
		"SELECT * FROM t LOCAL left array JOIN arr AS a",
		// A join takes at most one locality, and it precedes the join type
		"SELECT * FROM t1 GLOBAL LOCAL JOIN t2 ON t1.a = t2.a",
		"SELECT * FROM t1 LEFT GLOBAL JOIN t2 ON t1.a = t2.a",
		// A locality without a join is not an alias, so it can't be dropped
		"SELECT * FROM t1 GLOBAL",
		"SELECT * FROM t1 LOCAL",
		"SELECT * FROM t global",
		"SELECT * FROM t1 GLOBAL JOIN t2 ON t1.a = t2.a GLOBAL",
		"SELECT * FROM t1 GLOBAL WHERE a = 1",
		"SELECT * FROM t1 LOCAL SETTINGS max_threads = 1",
		// GLOBAL is a keyword everywhere, never an implicit alias
		"SELECT a GLOBAL FROM t",
		"SELECT a GLOBAL, b FROM t",
		// GLOBAL is an operator only before IN/NOT IN, and LOCAL is never one
		"SELECT * FROM t WHERE a GLOBAL LIKE 'x'",
		"SELECT * FROM t WHERE a GLOBAL NOT LIKE 'x'",
		"SELECT * FROM t WHERE a GLOBAL BETWEEN 1 AND 2",
		"SELECT * FROM t WHERE a NOT GLOBAL IN (SELECT 1)",
		"SELECT * FROM t WHERE a GLOBAL GLOBAL IN (SELECT 1)",
		"SELECT * FROM t WHERE a LOCAL IN (SELECT 1)",
		"SELECT * FROM t WHERE a LOCAL NOT IN (SELECT 1)",
		"CREATE--", // trailing comment pushes p.Pos() past end of input (wrapError out-of-range)
		// Inputs that previously caused a nil-pointer dereference while
		// formatting the error message at EOF (p.current() is nil).
		"ALTER ",
		"SELECT*FROM A(0A",
		"SET A=",
		// An operator as the last token of the input is missing its right
		// operand. The infix loop used to stop before it, so the operator
		// either reached the statement parser as unexpected trailing input or,
		// when it was a keyword, was read as an implicit alias.
		"SELECT a +",
		"SELECT a GLOBAL",
		"SELECT a REGEXP",
		"SELECT * FROM t WHERE a AND",
		// A parenthesized select must still be closed
		"(SELECT 1",
		// ClickHouse rejects a set operator once SETTINGS is bound to a
		// parenthesized group
		"(SELECT 1) SETTINGS max_threads=1 UNION ALL SELECT 2",
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

// Fork feature: an argument of extract(...) may carry AS alias, as in ClickHouse.
func TestParser_With_ExtractArgAlias(t *testing.T) {
	parseFirst := func(t *testing.T, sql string) *SelectQuery {
		t.Helper()
		stmts, err := NewParser(sql).ParseStmts()
		require.NoError(t, err)
		require.NotEmpty(t, stmts)
		q, ok := stmts[0].(*SelectQuery)
		require.True(t, ok)
		return q
	}
	extractOf := func(t *testing.T, q *SelectQuery, item int) *ExtractExpr {
		t.Helper()
		expr := q.SelectItems[item].Expr
		for {
			switch e := expr.(type) {
			case *ExtractExpr:
				return e
			case *ColumnExpr:
				expr = e.Expr
			case *FunctionExpr:
				require.NotEmpty(t, e.Params.Items.Items)
				expr = e.Params.Items.Items[0]
			default:
				require.FailNow(t, "no ExtractExpr found", "reached %T", expr)
			}
		}
	}

	const nestedSQL = `SELECT avg(toUInt32OrNull(extract(arrayJoin(splitByString('//', s)) AS part, '(\d+)'))) AS v, part FROM t`

	tests := []struct {
		name  string
		sql   string
		check func(t *testing.T, q *SelectQuery)
	}{
		{
			name: "Alias on a regex extract argument",
			sql:  nestedSQL,
			check: func(t *testing.T, q *SelectQuery) {
				params := extractOf(t, q, 0).Parameters
				require.Len(t, params, 2)
				col, ok := params[0].(*ColumnExpr)
				require.True(t, ok)
				require.Equal(t, "part", col.Alias.Name)
				_, ok = params[1].(*StringLiteral)
				require.True(t, ok)
			},
		},
		{
			name: "Alias on the haystack argument",
			sql:  `SELECT extract(s AS src, '\d+') AS n, src FROM t`,
			check: func(t *testing.T, q *SelectQuery) {
				col, ok := extractOf(t, q, 0).Parameters[0].(*ColumnExpr)
				require.True(t, ok)
				require.Equal(t, "src", col.Alias.Name)
			},
		},
		{
			name: "Alias inside EXTRACT unit FROM expr",
			sql:  `SELECT EXTRACT(YEAR FROM d AS day), day FROM t`,
			check: func(t *testing.T, q *SelectQuery) {
				params := extractOf(t, q, 0).Parameters
				require.Len(t, params, 1)
				col, ok := params[0].(*ColumnExpr)
				require.True(t, ok)
				require.Equal(t, "day", col.Alias.Name)
				_, ok = col.Expr.(*IntervalFrom)
				require.True(t, ok)
			},
		},
		{
			name: "Unaliased parameters unchanged",
			sql:  `SELECT extract(s, '\d+'), EXTRACT(MONTH FROM d) FROM t`,
			check: func(t *testing.T, q *SelectQuery) {
				for i := range q.SelectItems {
					for _, param := range extractOf(t, q, i).Parameters {
						_, isCol := param.(*ColumnExpr)
						require.False(t, isCol)
					}
				}
			},
		},
		{
			name: "Round trip keeps the alias",
			sql:  nestedSQL,
			check: func(t *testing.T, q *SelectQuery) {
				formatted := Format(q)
				require.Contains(t, formatted, "AS part")
				require.Equal(t, formatted, Format(parseFirst(t, formatted)))
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.check(t, parseFirst(t, tt.sql))
		})

	}
}

func TestParser_With_QualifiedStarPath(t *testing.T) {
	parseFirstSelect := func(t *testing.T, sql string) *SelectQuery {
		t.Helper()
		stmts, err := NewParser(sql).ParseStmts()
		require.NoError(t, err)
		require.NotEmpty(t, stmts)
		sel, ok := stmts[0].(*SelectQuery)
		require.True(t, ok)
		return sel
	}

	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "Three-part qualified star",
			run: func(t *testing.T) {
				sel := parseFirstSelect(t, "SELECT db.t.* FROM db.t")
				path, ok := sel.SelectItems[0].Expr.(*Path)
				require.True(t, ok)
				require.Len(t, path.Fields, 3)
				require.Equal(t, "*", path.Fields[2].Name)
			},
		},
		{
			name: "Mixed with a two-part star",
			run: func(t *testing.T) {
				sel := parseFirstSelect(t, "SELECT db.t.*, t.*, 1 FROM db.t AS t")
				_, ok := sel.SelectItems[0].Expr.(*Path)
				require.True(t, ok)
				nested, ok := sel.SelectItems[1].Expr.(*NestedIdentifier)
				require.True(t, ok)
				require.Equal(t, "*", nested.DotIdent.Name)
			},
		},
		{
			name: "Qualified star as a function argument",
			run: func(t *testing.T) {
				_, err := NewParser("SELECT count(db.t.*) FROM db.t").ParseStmts()
				require.NoError(t, err)
			},
		},
		{
			name: "Three-part identifier unchanged",
			run: func(t *testing.T) {
				sel := parseFirstSelect(t, "SELECT a.b.c FROM t")
				path, ok := sel.SelectItems[0].Expr.(*Path)
				require.True(t, ok)
				require.Len(t, path.Fields, 3)
				for _, f := range path.Fields {
					require.NotEqual(t, "*", f.Name)
				}
			},
		},
		{
			name: "A trailing member access is not absorbed into the chain",
			run: func(t *testing.T) {
				// The generic postfix ".x" operator wraps the path (same as the
				// pre-existing "t.*.x"); the star stays the last Path field.
				sel := parseFirstSelect(t, "SELECT db.t.*.x FROM t")
				idx, ok := sel.SelectItems[0].Expr.(*IndexOperation)
				require.True(t, ok)
				path, ok := idx.Object.(*Path)
				require.True(t, ok)
				require.Len(t, path.Fields, 3)
				require.Equal(t, "*", path.Fields[2].Name)
			},
		},
		{
			name: "Round trip keeps the qualified star",
			run: func(t *testing.T) {
				const sql = "SELECT db.t.* FROM db.t"
				got := Format(parseFirstSelect(t, sql))
				require.Equal(t, sql, got)
				require.Equal(t, got, Format(parseFirstSelect(t, got)))
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, tt.run)
	}
}

// Fork feature: a WITH-prefixed subquery in expression position parses like a SELECT-prefixed one.
func TestParser_With_WithPrefixedSubquery(t *testing.T) {
	firstSelect := func(t *testing.T, sql string) *SelectQuery {
		t.Helper()
		stmts, err := NewParser(sql).ParseStmts()
		require.NoError(t, err, "Failed to parse: %s", sql)
		require.NotEmpty(t, stmts)
		sel, ok := stmts[0].(*SelectQuery)
		require.True(t, ok, "expected *SelectQuery, got %T", stmts[0])
		return sel
	}

	// Function arguments are wrapped in a *ColumnExpr; unwrap to the inner expression.
	firstArg := func(t *testing.T, sel *SelectQuery) Expr {
		t.Helper()
		fn, ok := sel.SelectItems[0].Expr.(*FunctionExpr)
		require.True(t, ok, "expected *FunctionExpr, got %T", sel.SelectItems[0].Expr)
		require.Len(t, fn.Params.Items.Items, 1)
		arg := fn.Params.Items.Items[0]
		if ce, ok := arg.(*ColumnExpr); ok {
			return ce.Expr
		}
		return arg
	}

	tests := []struct {
		name  string
		check func(t *testing.T)
	}{
		{
			name: "Scalar subquery with a constant alias",
			check: func(t *testing.T) {
				sel := firstSelect(t, "SELECT (WITH 1000 AS latest SELECT max(x) FROM t WHERE y > latest) AS m")
				item := sel.SelectItems[0]
				sub, ok := item.Expr.(*SubQuery)
				require.True(t, ok, "expected *SubQuery, got %T", item.Expr)
				require.True(t, sub.HasParen)
				require.NotNil(t, sub.Select.With)
				require.Equal(t, "m", item.Alias.Name)
			},
		},
		{
			name: "Scalar subquery with a CTE",
			check: func(t *testing.T) {
				sel := firstSelect(t, "SELECT (WITH x AS (SELECT 1 AS a) SELECT a FROM x)")
				require.IsType(t, &SubQuery{}, sel.SelectItems[0].Expr)
			},
		},
		{
			name: "WITH-prefixed subquery as IN operand",
			check: func(t *testing.T) {
				firstSelect(t, "SELECT 1 IN (WITH 1 AS a SELECT a)")
			},
		},
		{
			name: "WITH-prefixed subquery as a WITH item",
			check: func(t *testing.T) {
				sel := firstSelect(t, "WITH (WITH 1 AS a SELECT a) AS b SELECT b")
				require.NotNil(t, sel.With)
				require.Len(t, sel.With.CTEs, 1)
			},
		},
		{
			name: "WITH-prefixed subquery as a function argument",
			check: func(t *testing.T) {
				sel := firstSelect(t, "SELECT exists(WITH 1 AS a SELECT a)")
				arg, ok := firstArg(t, sel).(*SelectQuery)
				require.True(t, ok, "expected *SelectQuery")
				require.NotNil(t, arg.With)
			},
		},
		{
			name: "SELECT-prefixed subqueries unchanged",
			check: func(t *testing.T) {
				sel := firstSelect(t, "SELECT (SELECT 1)")
				sub, ok := sel.SelectItems[0].Expr.(*SubQuery)
				require.True(t, ok, "expected *SubQuery, got %T", sel.SelectItems[0].Expr)
				require.True(t, sub.HasParen)
				firstSelect(t, "SELECT 1 IN (SELECT 1)")
				sel = firstSelect(t, "SELECT exists(SELECT 1)")
				require.IsType(t, &SelectQuery{}, firstArg(t, sel))
			},
		},
		{
			name: "Round trip is a fixed point",
			check: func(t *testing.T) {
				sel := firstSelect(t, "SELECT (WITH 1000 AS latest SELECT max(x) FROM t WHERE y > latest) AS m")
				formatted := Format(sel)
				require.Equal(t, formatted, Format(firstSelect(t, formatted)))
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, tt.check)
	}
}

// EXCEPT directly after `*` or COLUMNS(...) is a column modifier whatever the next
// token is; after any other select item it is a set operator (HDX-12821).
func TestParser_With_ExceptAfterStar(t *testing.T) {
	selectOf := func(t *testing.T, sql string) *SelectQuery {
		t.Helper()
		stmts, err := NewParser(sql).ParseStmts()
		require.NoError(t, err, "Failed to parse: %s", sql)
		require.Len(t, stmts, 1)
		q, ok := stmts[0].(*SelectQuery)
		require.True(t, ok, "not a SelectQuery: %s", sql)
		return q
	}
	paramsOf := func(m *FunctionExpr) []string {
		names := make([]string, 0)
		for _, item := range m.Params.Items.Items {
			names = append(names, Format(item))
		}
		return names
	}

	t.Run("Keyword-named column", func(t *testing.T) {
		q := selectOf(t, "SELECT timestamp, * EXCEPT timestamp FROM t")
		require.Len(t, q.SelectItems, 2)
		require.Empty(t, q.SelectItems[0].Modifiers)
		mods := q.SelectItems[1].Modifiers
		require.Len(t, mods, 1)
		require.Equal(t, "EXCEPT", mods[0].Name.Name)
		require.Equal(t, []string{"timestamp"}, paramsOf(mods[0]))

		for _, sql := range []string{"SELECT * EXCEPT date FROM t", "SELECT * EXCEPT key FROM t"} {
			q := selectOf(t, sql)
			require.Len(t, q.SelectItems[0].Modifiers, 1, sql)
			require.Equal(t, "EXCEPT", q.SelectItems[0].Modifiers[0].Name.Name, sql)
			require.Len(t, paramsOf(q.SelectItems[0].Modifiers[0]), 1, sql)
		}
	})

	t.Run("STRICT bare and list", func(t *testing.T) {
		tests := []struct {
			sql  string
			want []string
		}{
			{"SELECT * EXCEPT STRICT timestamp FROM t", []string{"timestamp"}},
			{"SELECT * EXCEPT STRICT (timestamp, key) FROM t", []string{"timestamp", "key"}},
		}
		for _, tt := range tests {
			sql := tt.sql
			item := selectOf(t, sql).SelectItems[0]
			require.Nil(t, item.Alias, sql)
			require.Len(t, item.Modifiers, 1, sql)
			m := item.Modifiers[0]
			require.Equal(t, "EXCEPT STRICT", m.Name.Name, sql)
			require.Equal(t, Pos(strings.Index(sql, "STRICT")+len("STRICT")), m.Name.NameEnd, sql)
			require.Equal(t, tt.want, paramsOf(m), sql)
		}
	})

	t.Run("COLUMNS then EXCEPT", func(t *testing.T) {
		item := selectOf(t, "SELECT COLUMNS('^a') EXCEPT timestamp FROM t").SelectItems[0]
		require.Len(t, item.Modifiers, 1)
		require.Equal(t, "EXCEPT", item.Modifiers[0].Name.Name)
	})

	t.Run("Chained EXCEPT modifiers", func(t *testing.T) {
		item := selectOf(t, "SELECT * EXCEPT timestamp EXCEPT b FROM t").SelectItems[0]
		require.Len(t, item.Modifiers, 2)
		require.Equal(t, "EXCEPT", item.Modifiers[0].Name.Name)
		require.Equal(t, "EXCEPT", item.Modifiers[1].Name.Name)
	})

	t.Run("Set operator matrix still parses as set operation", func(t *testing.T) {
		for _, sql := range []string{
			"SELECT 1 EXCEPT SELECT 2",
			"SELECT 1 EXCEPT ALL SELECT 2",
			"SELECT 1 EXCEPT DISTINCT SELECT 2",
		} {
			q := selectOf(t, sql)
			require.NotNil(t, q.Except, sql)
			require.Empty(t, q.SelectItems[0].Modifiers, sql)
		}
	})

	t.Run("Parenthesised subquery after a non-star item is a set operation", func(t *testing.T) {
		q := selectOf(t, "SELECT 1 EXCEPT (SELECT 2)")
		require.NotNil(t, q.Except)
		require.Empty(t, q.SelectItems[0].Modifiers)
	})

	t.Run("Modifier then set operation after FROM", func(t *testing.T) {
		q := selectOf(t, "SELECT * EXCEPT timestamp FROM t EXCEPT ALL SELECT 3")
		require.Len(t, q.SelectItems[0].Modifiers, 1)
		require.NotNil(t, q.Except)
		require.Equal(t, ExceptModeAll, q.ExceptMode)
	})

	t.Run("Non-star item rejects EXCEPT ident", func(t *testing.T) {
		_, err := NewParser("SELECT a EXCEPT b FROM t").ParseStmts()
		require.Error(t, err)
	})

	t.Run("STRICT round-trips through Format", func(t *testing.T) {
		want := "SELECT * EXCEPT STRICT(timestamp) FROM t"
		got := Format(selectOf(t, "SELECT * EXCEPT STRICT timestamp FROM t"))
		require.Equal(t, want, got)
		require.Equal(t, want, Format(selectOf(t, got)))
	})
}

// A qualified star takes EXCEPT as a column modifier like a bare star does
// (`db.t.*` is covered by the same check once the three-part form parses).
func TestParser_With_ExceptAfterQualifiedStar(t *testing.T) {
	stmts, err := NewParser("SELECT t.* EXCEPT a FROM t").ParseStmts()
	require.NoError(t, err)
	require.Len(t, stmts, 1)
	sel, ok := stmts[0].(*SelectQuery)
	require.True(t, ok, "expected *SelectQuery, got %T", stmts[0])
	require.Len(t, sel.SelectItems[0].Modifiers, 1)
	require.Equal(t, "EXCEPT", sel.SelectItems[0].Modifiers[0].Name.Name)
	require.Nil(t, sel.Except)
}

// `interval` is a keyword but must still work as an identifier (alias, column)
// wherever it does not start an INTERVAL <expr> <unit> literal.
func TestParser_With_IntervalIdentifier(t *testing.T) {
	requireIntervalIdent := func(t *testing.T, expr Expr) {
		t.Helper()
		if wrapped, isWrapped := expr.(*ColumnExpr); isWrapped {
			expr = wrapped.Expr
		}
		ident, ok := expr.(*Ident)
		require.True(t, ok, "expected *Ident, got %T", expr)
		require.Equal(t, "interval", ident.Name)
	}

	tests := []struct {
		name  string
		sql   string
		check func(t *testing.T, q *SelectQuery)
	}{
		{
			name: "interval alias in GROUP BY and ORDER BY",
			sql:  "SELECT toStartOfInterval(ts, INTERVAL 1 MINUTE) AS interval, count() FROM t GROUP BY interval ORDER BY interval",
			check: func(t *testing.T, q *SelectQuery) {
				groupBy, ok := q.GroupBy.Expr.(*ColumnExprList)
				require.True(t, ok, "expected *ColumnExprList, got %T", q.GroupBy.Expr)
				require.Len(t, groupBy.Items, 1)
				requireIntervalIdent(t, groupBy.Items[0])
				require.Len(t, q.OrderBy.Items, 1)
				order, ok := q.OrderBy.Items[0].(*OrderExpr)
				require.True(t, ok, "expected *OrderExpr, got %T", q.OrderBy.Items[0])
				requireIntervalIdent(t, order.Expr)
			},
		},
		{
			name: "interval in WHERE and ORDER BY DESC",
			sql:  "SELECT interval FROM t WHERE interval > 0 ORDER BY interval DESC",
			check: func(t *testing.T, q *SelectQuery) {
				requireIntervalIdent(t, q.SelectItems[0].Expr)
			},
		},
		{
			name: "interval in select list and parentheses",
			sql:  "SELECT (interval), interval + 1 FROM t",
		},
		{
			name: "Interval literals unchanged",
			sql:  "SELECT now() - INTERVAL 1 HOUR, INTERVAL (1) MINUTE, INTERVAL x DAY FROM t",
			check: func(t *testing.T, q *SelectQuery) {
				require.Len(t, q.SelectItems, 3)
				binary, ok := q.SelectItems[0].Expr.(*BinaryOperation)
				require.True(t, ok, "expected *BinaryOperation, got %T", q.SelectItems[0].Expr)
				require.IsType(t, &IntervalExpr{}, binary.RightExpr)
				require.IsType(t, &IntervalExpr{}, q.SelectItems[1].Expr)
				require.IsType(t, &IntervalExpr{}, q.SelectItems[2].Expr)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := parseSelectForTest(t, tt.sql)
			if tt.check != nil {
				tt.check(t, q)
			}
		})
	}

	t.Run("Malformed literal still errors", func(t *testing.T) {
		_, err := NewParser("SELECT INTERVAL 1 FOO").ParseStmts()
		require.Error(t, err)
	})

	t.Run("Round trip keeps the alias", func(t *testing.T) {
		sql := "SELECT 1 AS interval FROM t GROUP BY interval ORDER BY interval"
		got := Format(parseSelectForTest(t, sql))
		require.Equal(t, sql, got)
		require.Equal(t, got, Format(parseSelectForTest(t, got)))
	})
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

// Covers the full 3x3 matrix of set operators {UNION, EXCEPT, INTERSECT} times
// {bare, ALL, DISTINCT}, plus SETTINGS clauses combined with set ops.
func TestParser_With_SetOperators(t *testing.T) {
	validSQLs := []string{
		"SELECT 1 UNION SELECT 2",
		"SELECT 1 UNION ALL SELECT 2",
		"SELECT 1 UNION DISTINCT SELECT 2",
		"SELECT 1 EXCEPT SELECT 2",
		"SELECT 1 EXCEPT ALL SELECT 2",
		"SELECT 1 EXCEPT DISTINCT SELECT 2",
		"SELECT 1 INTERSECT SELECT 2",
		"SELECT 1 INTERSECT ALL SELECT 2",
		"SELECT 1 INTERSECT DISTINCT SELECT 2",
		"SELECT 1 SETTINGS max_threads=1 UNION SELECT 2 SETTINGS max_threads=2",
		"SELECT 1 INTERSECT ALL SELECT 2 SETTINGS max_threads=2",
	}
	for _, sql := range validSQLs {
		parser := NewParser(sql)
		_, err := parser.ParseStmts()
		require.NoError(t, err, "Failed to parse: %s", sql)
	}
}

// Disambiguates the four placements of SETTINGS around a paren-wrapped set-op
// leg (inside parens, outside parens, top-level wrapped chain, both coexist),
// plus the no-SETTINGS round-trip case. Locks in the per-node invariant
// (OuterSettings != nil ⇒ HasParen == true) and the no-parens fallback.
func TestParser_With_ChainSettingsDisambiguation(t *testing.T) {
	type expect struct {
		// outer = head SelectQuery returned by ParseStmts.
		outerHasParen      bool
		outerHasSettings   bool
		outerHasOuterSetts bool
		// inner = outer.Union when non-nil (all five SQLs use UNION ALL).
		// nil if the SQL has no set-op chain (the top-level wrapped chain case
		// still has Union; only future bare top-level SQLs would not).
		innerHasParen      bool
		innerHasSettings   bool
		innerHasOuterSetts bool
	}
	cases := []struct {
		name string
		sql  string
		want expect
	}{
		{
			name: "inside parens — attaches to inner Settings",
			sql:  "SELECT 1 UNION ALL (SELECT 2 SETTINGS max_threads=1)",
			want: expect{
				innerHasParen:    true,
				innerHasSettings: true,
			},
		},
		{
			name: "outside parens — attaches to inner OuterSettings",
			sql:  "SELECT 1 UNION ALL (SELECT 2) SETTINGS max_threads=1",
			want: expect{
				innerHasParen:      true,
				innerHasOuterSetts: true,
			},
		},
		{
			name: "top-level wrapped chain — attaches to outer OuterSettings",
			sql:  "(SELECT 1 UNION ALL SELECT 2) SETTINGS max_threads=1",
			want: expect{
				outerHasParen:      true,
				outerHasOuterSetts: true,
			},
		},
		{
			name: "parens preserved, no SETTINGS",
			sql:  "SELECT 1 UNION ALL (SELECT 2)",
			want: expect{
				innerHasParen: true,
			},
		},
		{
			name: "both placements coexist on the same leg",
			sql:  "SELECT 1 UNION ALL (SELECT 2 SETTINGS max_threads=1) SETTINGS max_threads=2",
			want: expect{
				innerHasParen:      true,
				innerHasSettings:   true,
				innerHasOuterSetts: true,
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stmts, err := NewParser(tc.sql).ParseStmts()
			require.NoError(t, err, "parse failed for: %s", tc.sql)
			require.Len(t, stmts, 1, "expected exactly one statement")
			outer, ok := stmts[0].(*SelectQuery)
			require.True(t, ok, "expected *SelectQuery, got %T", stmts[0])

			require.Equal(t, tc.want.outerHasParen, outer.HasParen, "outer.HasParen")
			require.Equal(t, tc.want.outerHasSettings, outer.Settings != nil, "outer.Settings non-nil")
			require.Equal(t, tc.want.outerHasOuterSetts, outer.OuterSettings != nil, "outer.OuterSettings non-nil")

			require.NotNil(t, outer.Union, "all five SQLs have a UNION leg")
			inner := outer.Union
			require.Equal(t, tc.want.innerHasParen, inner.HasParen, "inner.HasParen")
			require.Equal(t, tc.want.innerHasSettings, inner.Settings != nil, "inner.Settings non-nil")
			require.Equal(t, tc.want.innerHasOuterSetts, inner.OuterSettings != nil, "inner.OuterSettings non-nil")

			// Per-node invariant: OuterSettings != nil ⇒ HasParen == true.
			if outer.OuterSettings != nil {
				require.True(t, outer.HasParen, "invariant violated on outer: OuterSettings non-nil but HasParen false")
			}
			if inner.OuterSettings != nil {
				require.True(t, inner.HasParen, "invariant violated on inner: OuterSettings non-nil but HasParen false")
			}
		})
	}

	// Bare no-parens fallback: SETTINGS lands on inner.Settings, not OuterSettings.
	t.Run("no-parens trailing SETTINGS stays on inner Settings", func(t *testing.T) {
		stmts, err := NewParser("SELECT 1 UNION ALL SELECT 2 SETTINGS max_threads=1").ParseStmts()
		require.NoError(t, err)
		outer, ok := stmts[0].(*SelectQuery)
		if !ok {
			require.Failf(t, "Type coarse fail.", "expected SelectQuery, got %T", stmts[0])
		}
		require.False(t, outer.HasParen)
		require.Nil(t, outer.Settings)
		require.Nil(t, outer.OuterSettings)
		require.NotNil(t, outer.Union)
		require.False(t, outer.Union.HasParen)
		require.NotNil(t, outer.Union.Settings)
		require.Nil(t, outer.Union.OuterSettings)
	})
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
// body starts with `$` (e.g. “ `$col` “) must be treated as an ordinary
// identifier, not as a Grafana template variable. Before the QuoteType
// guard was added to matchVariable, the lexer-stripped String field made
// “ `$col` “ look like a `$`-prefixed variable, which threw it onto the
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
// for “ `$col` “ and parseInfix swallowed it as the operator token,
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
			// Prime the lexer so p.current() points at the first token.
			require.NoError(t, p.lexer.consumeToken())
			require.Equal(t, tc.want, p.matchVariable(), "matchVariable() mismatch for %q", tc.sql)
		})
	}
}

func parseSelectForTest(t *testing.T, sql string) *SelectQuery {
	t.Helper()
	stmts, err := NewParser(sql).ParseStmts()
	require.NoError(t, err, sql)
	require.Len(t, stmts, 1, sql)
	query, ok := stmts[0].(*SelectQuery)
	require.True(t, ok, "expected *SelectQuery for %s, got %T", sql, stmts[0])
	return query
}

func settingNames(clause *SettingsClause) []string {
	names := make([]string, 0, len(clause.Items))
	for _, item := range clause.Items {
		names = append(names, item.Name.Name)
	}
	return names
}

// Covers the settings-after-format spec: a SETTINGS clause after FORMAT lands
// in SelectQuery.FormatSettings, never in Settings.
func TestParser_With_FormatSettings(t *testing.T) {
	t.Run("after format", func(t *testing.T) {
		q := parseSelectForTest(t, "SELECT 1 FORMAT JSON SETTINGS max_threads = 1")
		require.NotNil(t, q.Format)
		require.NotNil(t, q.FormatSettings)
		require.Equal(t, []string{"max_threads"}, settingNames(q.FormatSettings))
		require.Nil(t, q.Settings)
	})

	t.Run("production query", func(t *testing.T) {
		q := parseSelectForTest(t, "SELECT count() AS n FROM hydro.logs WHERE timestamp >= now() - toIntervalHour(1) GROUP BY app ORDER BY n DESC FORMAT CSVWithNames SETTINGS hdx_query_max_timerange_sec = 86400, hdx_query_max_execution_time = 60")
		require.NotNil(t, q.FormatSettings)
		require.Len(t, q.FormatSettings.Items, 2)
	})

	t.Run("before format is unchanged", func(t *testing.T) {
		q := parseSelectForTest(t, "SELECT 1 SETTINGS max_threads = 1 FORMAT JSON")
		require.NotNil(t, q.Settings)
		require.Len(t, q.Settings.Items, 1)
		require.Nil(t, q.FormatSettings)
	})

	t.Run("both positions are kept", func(t *testing.T) {
		q := parseSelectForTest(t, "SELECT 1 SETTINGS a = 1 FORMAT JSON SETTINGS b = 2")
		require.Equal(t, []string{"a"}, settingNames(q.Settings))
		require.Equal(t, []string{"b"}, settingNames(q.FormatSettings))
	})

	t.Run("trailing leg of a set operation owns it", func(t *testing.T) {
		q := parseSelectForTest(t, "SELECT 1 UNION ALL SELECT 2 FORMAT JSON SETTINGS a = 1")
		require.Nil(t, q.FormatSettings)
		require.NotNil(t, q.Union)
		require.Equal(t, []string{"a"}, settingNames(q.Union.FormatSettings))
	})

	t.Run("settings without format cannot repeat", func(t *testing.T) {
		_, err := NewParser("SELECT 1 SETTINGS a = 1 SETTINGS b = 2").ParseStmts()
		require.Error(t, err)
	})

	t.Run("settings cannot repeat after format", func(t *testing.T) {
		_, err := NewParser("SELECT 1 FORMAT JSON SETTINGS a = 1 SETTINGS b = 2").ParseStmts()
		require.Error(t, err)
	})

	t.Run("positions cover the clause", func(t *testing.T) {
		sql := "SELECT 1 FORMAT JSON SETTINGS a = 'x'"
		q := parseSelectForTest(t, sql)
		require.Equal(t, Pos(strings.Index(sql, "SETTINGS")), q.FormatSettings.SettingsPos)
		last := q.FormatSettings.Items[len(q.FormatSettings.Items)-1]
		require.GreaterOrEqual(t, q.StatementEnd, last.End())
		require.Equal(t, q.FormatSettings.End(), q.StatementEnd)
	})

	t.Run("round trip keeps the order", func(t *testing.T) {
		sql := "SELECT 1 FORMAT JSON SETTINGS max_threads = 1"
		want := "SELECT 1 FORMAT JSON SETTINGS max_threads=1"
		got := Format(parseSelectForTest(t, sql))
		require.Equal(t, want, got)
		require.Equal(t, want, Format(parseSelectForTest(t, got)))
		beautifier := NewFormatter().WithBeautify()
		beautifier.WriteExpr(parseSelectForTest(t, sql))
		require.Contains(t, beautifier.String(), "FORMAT JSON\nSETTINGS\n  max_threads=1")
	})

	t.Run("walk visits the clause", func(t *testing.T) {
		// The settings value grammar accepts literals only (not function
		// calls), so the walk is checked down to the value literal.
		q := parseSelectForTest(t, "SELECT 1 FORMAT JSON SETTINGS a = 7")
		var sawClause, sawItem, sawValue bool
		Walk(q, func(node Expr) bool {
			switch n := node.(type) {
			case *SettingsClause:
				sawClause = true
			case *SettingExpr:
				sawItem = true
			case *NumberLiteral:
				if n.Literal == "7" {
					sawValue = true
				}
			}
			return true
		})
		require.True(t, sawClause, "Walk should visit the *SettingsClause")
		require.True(t, sawItem, "Walk should visit the *SettingExpr")
		require.True(t, sawValue, "Walk should visit the setting value")
	})

	t.Run("accept visits the clause", func(t *testing.T) {
		q := parseSelectForTest(t, "SELECT 1 FORMAT JSON SETTINGS a = 1")
		settings := 0
		v := &DefaultASTVisitor{Visit: func(expr Expr) error {
			if _, ok := expr.(*SettingsClause); ok {
				settings++
			}
			return nil
		}}
		require.NoError(t, q.Accept(v))
		require.Equal(t, 1, settings)
	})
}

func settingNumber(t *testing.T, item *SettingExpr, literal string) {
	t.Helper()
	num, ok := item.Expr.(*NumberLiteral)
	require.True(t, ok, "expected *NumberLiteral, got %T", item.Expr)
	require.Equal(t, literal, num.Literal)
}

// Covers the settings-float-values spec: decimal float literals are accepted
// as SETTINGS values and keep their source text.
func TestParser_With_FloatSettings(t *testing.T) {
	t.Run("Float value in SELECT SETTINGS", func(t *testing.T) {
		q := parseSelectForTest(t, "SELECT 1 SETTINGS max_bytes_ratio_before_external_group_by = 0.5")
		require.Len(t, q.Settings.Items, 1)
		settingNumber(t, q.Settings.Items[0], "0.5")
	})

	t.Run("Float forms keep their source text", func(t *testing.T) {
		for _, value := range []string{"-0.5", "+0.5", ".5", "1.", "1.5e-3"} {
			t.Run(value, func(t *testing.T) {
				q := parseSelectForTest(t, "SELECT 1 SETTINGS x = "+value)
				require.Len(t, q.Settings.Items, 1)
				settingNumber(t, q.Settings.Items[0], value)
			})
		}
	})

	t.Run("Float mixed with other values", func(t *testing.T) {
		q := parseSelectForTest(t, "SELECT 1 SETTINGS a = 0.5, b = 1, c = 'x'")
		require.Len(t, q.Settings.Items, 3)
		settingNumber(t, q.Settings.Items[0], "0.5")
	})

	t.Run("Float after FORMAT", func(t *testing.T) {
		q := parseSelectForTest(t, "SELECT 1 FORMAT JSON SETTINGS x = 0.5")
		require.Len(t, q.FormatSettings.Items, 1)
		settingNumber(t, q.FormatSettings.Items[0], "0.5")
	})

	t.Run("Float in SET", func(t *testing.T) {
		stmts, err := NewParser("SET x = 0.5").ParseStmts()
		require.NoError(t, err)
		require.Len(t, stmts, 1)
		set, ok := stmts[0].(*SetStmt)
		require.True(t, ok, "got %T", stmts[0])
		require.Equal(t, []string{"x"}, settingNames(set.Settings))
		settingNumber(t, set.Settings.Items[0], "0.5")
	})

	t.Run("Float in DDL settings lists", func(t *testing.T) {
		stmts, err := NewParser("CREATE TABLE t (a Int32) ENGINE = MergeTree ORDER BY a SETTINGS x = 0.5").ParseStmts()
		require.NoError(t, err)
		require.Len(t, stmts, 1)
		create, ok := stmts[0].(*CreateTable)
		require.True(t, ok, "got %T", stmts[0])
		require.NotNil(t, create.Engine.Settings)
		settingNumber(t, create.Engine.Settings.Items[0], "0.5")

		stmts, err = NewParser("ALTER TABLE t MODIFY SETTING x = 0.5").ParseStmts()
		require.NoError(t, err)
		require.Len(t, stmts, 1)
		alter, ok := stmts[0].(*AlterTable)
		require.True(t, ok, "got %T", stmts[0])
		require.Len(t, alter.AlterExprs, 1)
		modify, ok := alter.AlterExprs[0].(*AlterTableModifySetting)
		require.True(t, ok, "got %T", alter.AlterExprs[0])
		require.Len(t, modify.Settings, 1)
		settingNumber(t, modify.Settings[0], "0.5")
	})

	t.Run("Positions cover the float token", func(t *testing.T) {
		sql := "SELECT 1 SETTINGS x = -0.5"
		q := parseSelectForTest(t, sql)
		value := q.Settings.Items[0].Expr
		require.Equal(t, Pos(strings.Index(sql, "-0.5")), value.Pos())
		require.Equal(t, Pos(len(sql)), value.End())
		require.Equal(t, value.End(), q.Settings.ListEnd)
	})

	t.Run("Leading-dot position starts at the dot", func(t *testing.T) {
		sql := "SELECT 1 SETTINGS x = .5"
		q := parseSelectForTest(t, sql)
		value := q.Settings.Items[0].Expr
		require.Equal(t, Pos(strings.Index(sql, ".5")), value.Pos())
		require.Equal(t, Pos(len(sql)), value.End())
	})

	t.Run("Integer value is unchanged", func(t *testing.T) {
		q := parseSelectForTest(t, "SELECT 1 SETTINGS x = -1")
		settingNumber(t, q.Settings.Items[0], "-1")
	})

	t.Run("Non-number value still errors", func(t *testing.T) {
		_, err := NewParser("SELECT 1 SETTINGS x = (1)").ParseStmts()
		require.Error(t, err)
	})

	t.Run("Round trip keeps the literal", func(t *testing.T) {
		sql := "SELECT 1 SETTINGS a = 0.5, b = .5, c = 1.5e-3"
		want := "SELECT 1 SETTINGS a=0.5, b=.5, c=1.5e-3"
		got := Format(parseSelectForTest(t, sql))
		require.Equal(t, want, got)
		require.Equal(t, want, Format(parseSelectForTest(t, got)))
	})
}

// Covers the settings-reserved-keyword-names spec: a SETTINGS item name may be
// any identifier or keyword, reserved or not.
func TestParser_With_ReservedKeywordSettingNames(t *testing.T) {
	t.Run("limit", func(t *testing.T) {
		q := parseSelectForTest(t, "SELECT 1 SETTINGS limit = 5")
		require.Equal(t, []string{"limit"}, settingNames(q.Settings))
		settingNumber(t, q.Settings.Items[0], "5")
	})

	t.Run("offset", func(t *testing.T) {
		q := parseSelectForTest(t, "SELECT 1 SETTINGS offset = 10")
		require.Equal(t, []string{"offset"}, settingNames(q.Settings))
		settingNumber(t, q.Settings.Items[0], "10")
	})

	t.Run("final", func(t *testing.T) {
		q := parseSelectForTest(t, "SELECT 1 SETTINGS final = 1")
		require.Equal(t, []string{"final"}, settingNames(q.Settings))
		settingNumber(t, q.Settings.Items[0], "1")
	})

	t.Run("mixed with plain names", func(t *testing.T) {
		q := parseSelectForTest(t, "SELECT 1 SETTINGS max_threads = 4, limit = 5, offset = 10")
		require.Equal(t, []string{"max_threads", "limit", "offset"}, settingNames(q.Settings))
	})

	t.Run("after FORMAT", func(t *testing.T) {
		q := parseSelectForTest(t, "SELECT 1 FORMAT JSON SETTINGS limit = 5")
		require.Nil(t, q.Settings)
		require.Equal(t, []string{"limit"}, settingNames(q.FormatSettings))
	})

	t.Run("in SET", func(t *testing.T) {
		stmts, err := NewParser("SET limit = 5").ParseStmts()
		require.NoError(t, err)
		require.Len(t, stmts, 1)
		set, ok := stmts[0].(*SetStmt)
		require.True(t, ok, "got %T", stmts[0])
		require.Equal(t, []string{"limit"}, settingNames(set.Settings))
		settingNumber(t, set.Settings.Items[0], "5")
	})

	t.Run("in DDL settings lists", func(t *testing.T) {
		stmts, err := NewParser("ALTER TABLE t MODIFY SETTING limit = 5").ParseStmts()
		require.NoError(t, err)
		alter, ok := stmts[0].(*AlterTable)
		require.True(t, ok, "got %T", stmts[0])
		modify, ok := alter.AlterExprs[0].(*AlterTableModifySetting)
		require.True(t, ok, "got %T", alter.AlterExprs[0])
		require.Equal(t, "limit", modify.Settings[0].Name.Name)
	})

	t.Run("non-reserved keyword name", func(t *testing.T) {
		require.False(t, reservedKeywords.Contains(KeywordDate))
		q := parseSelectForTest(t, "SELECT 1 SETTINGS date = 1")
		require.Equal(t, []string{"date"}, settingNames(q.Settings))
	})

	t.Run("quoted reserved keyword name", func(t *testing.T) {
		q := parseSelectForTest(t, "SELECT 1 SETTINGS `limit` = 5")
		require.Equal(t, []string{"limit"}, settingNames(q.Settings))
		require.Equal(t, BackTicks, q.Settings.Items[0].Name.QuoteType)
	})

	t.Run("name positions cover the token", func(t *testing.T) {
		sql := "SELECT 1 SETTINGS limit = 5"
		q := parseSelectForTest(t, sql)
		name := q.Settings.Items[0].Name
		require.Equal(t, Pos(strings.Index(sql, "limit")), name.Pos())
		require.Equal(t, Pos(strings.Index(sql, "limit")+len("limit")), name.End())
	})

	t.Run("still invalid", func(t *testing.T) {
		for _, sql := range []string{
			"SELECT 1 SETTINGS = 5",
			"SELECT 1 SETTINGS 5 = 5",
			"SELECT 1 SETTINGS limit",
			"SELECT 1 SETTINGS 'limit' = 5",
		} {
			t.Run(sql, func(t *testing.T) {
				_, err := NewParser(sql).ParseStmts()
				require.Error(t, err)
			})
		}
	})
}
