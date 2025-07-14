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
					formatSQLBuilder.WriteString(stmt.String())
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

// validFormatSQL Verify that the format sql can be re-parsed with consistent results
func validFormatSQL(t *testing.T, sql string) {
	parser := NewParser(sql)
	stmts, err := parser.ParseStmts()
	require.NoError(t, err)
	var builder strings.Builder
	for _, stmt := range stmts {
		builder.WriteString(stmt.String())
		builder.WriteByte(';')
		builder.WriteByte('\n')
	}
	require.Equal(t, sql, builder.String())
}

func TestParser_InvalidSyntax(t *testing.T) {
	invalidSQLs := []string{
		"SELECT * FROM",
	}
	for _, sql := range invalidSQLs {
		parser := NewParser(sql)
		_, err := parser.ParseStmts()
		require.Error(t, err)
	}
}

func TestParser_ConditionALL_With_Variables(t *testing.T) {
	validSQLs := []string{
		//"SELECT 1 FROM table WHERE statusCode ${a} (1,2)",
		//"SELECT toString(statusCode) as HTTP_Status_Code, $__timeInterval(${timefilter}) as time, ${count} as http FROM ${table} WHERE $__timeFilter(${timefilter}) AND $__conditionalAll( statusCode IN (${statusCode:sqlstring}), $statusCode)",
		"SELECT toString(statusCode) as HTTP_Status_Code, $__timeInterval(${timefilter}) as time, ${count} as http FROM ${table} WHERE $__timeFilter(${timefilter}) AND $__conditionalAll( statusCode ${AND_statusCode} (${statusCode:sqlstring}), $statusCode)",
	}
	for _, sql := range validSQLs {
		parser := NewParser(sql)
		expr, err := parser.ParseStmts()
		marshal, err := json.Marshal(expr)
		fmt.Printf("%s", marshal)
		require.NoError(t, err)
	}
}

type selectQueryVisitor struct {
	DefaultASTVisitor
	Start int
	End   int
}

func (v *selectQueryVisitor) VisitTableExpr(expr *TableExpr) error {
	if strings.HasPrefix(expr.String(), "(") {
		v.Start = int(expr.Pos())
		v.End = int(expr.End())
	}

	return nil
}

func TestParser_With_SubSelect(t *testing.T) {
	validSQLs := map[string]string{
		"SELECT\n  bucket,\n  count()\nFROM\n  (\n    SELECT\n      toStartOfInterval(${timestamp}, INTERVAL 1 hour) AS bucket\n    FROM\n      ${table}\n    WHERE\n      $__timeFilter(${timestamp})\n      AND $__adHocFilter()\n  )\nWHERE\n  $__adHocFilter()\nGROUP BY\n  bucket":               ")",
		"SELECT\n  bucket,\n  count()\nFROM\n  (\n    SELECT\n      toStartOfInterval(${timestamp}, INTERVAL 1 hour) AS bucket\n    FROM\n      ${table}\n    WHERE\n      $__timeFilter(${timestamp})\n      AND $__adHocFilter()\n  ) as `alias1` \nWHERE\n  $__adHocFilter()\nGROUP BY\n  bucket":  "alias1",
		"SELECT\n  bucket,\n  count()\nFROM\n  (\n    SELECT\n      toStartOfInterval(${timestamp}, INTERVAL 1 hour) AS bucket\n    FROM\n      ${table}\n    WHERE\n      $__timeFilter(${timestamp})\n      AND $__adHocFilter()\n  ) as `alias 2` \nWHERE\n  $__adHocFilter()\nGROUP BY\n  bucket": "alias 2",
	}
	for sql, suffix := range validSQLs {
		println(sql)
		parser := NewParser(sql)
		expr, err := parser.ParseStmts()
		visitor := selectQueryVisitor{}
		err = expr[0].Accept(&visitor)
		require.NoError(t, err)
		require.NotNil(t, visitor.Start)
		require.NotNil(t, visitor.End)
		println(sql[visitor.Start:visitor.End])
		require.True(t, strings.HasSuffix(strings.TrimSpace(sql[visitor.Start:visitor.End]), suffix))

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
