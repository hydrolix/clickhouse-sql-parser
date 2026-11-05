package parser

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// asType asserts that v has the type T and returns it, failing the test with
// the actual type otherwise.
func asType[T any](t *testing.T, v any) T {
	t.Helper()
	typed, ok := v.(T)
	require.True(t, ok, "expected %T, got %T", typed, v)
	return typed
}

func parseOneStmt(t *testing.T, sql string) Expr {
	t.Helper()
	stmts, err := NewParser(sql).ParseStmts()
	require.NoError(t, err)
	require.Len(t, stmts, 1)
	return stmts[0]
}

func TestRatioExprPositions(t *testing.T) {
	sql := "SELECT * FROM t SAMPLE 1/2"
	stmt := asType[*SelectQuery](t, parseOneStmt(t, sql))
	sample := asType[*JoinTableExpr](t, stmt.From.Expr).SampleRatio
	require.NotNil(t, sample)
	ratio := sample.Ratio
	require.NotNil(t, ratio.Denominator)
	// numerator "1" at offset 23, denominator "2" at offset 25 — the spans
	// must not overlap (the denominator used to inherit the numerator's pos)
	require.Equal(t, Pos(23), ratio.Numerator.Pos())
	require.Equal(t, Pos(24), ratio.Numerator.End())
	require.Equal(t, Pos(25), ratio.Denominator.Pos())
	require.Equal(t, Pos(26), ratio.Denominator.End())
}

func TestGroupByClauseEnd(t *testing.T) {
	sql := "SELECT a FROM t GROUP BY a WITH TOTALS"
	stmt := asType[*SelectQuery](t, parseOneStmt(t, sql))
	require.NotNil(t, stmt.GroupBy)
	// the clause ends at the TOTALS keyword, not at the next token's start
	require.Equal(t, Pos(len(sql)), stmt.GroupBy.End())

	sql = "SELECT a FROM t GROUP BY a"
	stmt = asType[*SelectQuery](t, parseOneStmt(t, sql))
	require.Equal(t, Pos(len(sql)), stmt.GroupBy.End())

	sql = "SELECT a FROM t GROUP BY ALL"
	stmt = asType[*SelectQuery](t, parseOneStmt(t, sql))
	require.Equal(t, Pos(len(sql)), stmt.GroupBy.End())
}

func TestIsNullExprPositions(t *testing.T) {
	sql := "SELECT a IS NULL"
	stmt := asType[*SelectQuery](t, parseOneStmt(t, sql))
	isNull := asType[*IsNullExpr](t, stmt.SelectItems[0].Expr)
	// the node spans `a IS NULL`: from the operand to the end of NULL
	require.Equal(t, Pos(7), isNull.Pos())
	require.Equal(t, Pos(len(sql)), isNull.End())
	require.Equal(t, Pos(9), isNull.IsPos) // position of IS

	sql = "SELECT a IS NOT NULL"
	stmt = asType[*SelectQuery](t, parseOneStmt(t, sql))
	isNotNull := asType[*IsNotNullExpr](t, stmt.SelectItems[0].Expr)
	require.Equal(t, Pos(7), isNotNull.Pos())
	require.Equal(t, Pos(len(sql)), isNotNull.End())
	require.Equal(t, Pos(9), isNotNull.IsPos)
}

func TestAlterDetachPartitionPos(t *testing.T) {
	sql := "ALTER TABLE t DETACH PARTITION p"
	stmt := asType[*AlterTable](t, parseOneStmt(t, sql))
	detach := asType[*AlterTableDetachPartition](t, stmt.AlterExprs[0])
	// the clause starts at the DETACH keyword, like every sibling clause
	require.Equal(t, Pos(14), detach.Pos())
}

func TestAlterDropPartitionEndIncludesSettings(t *testing.T) {
	sql := "ALTER TABLE t DROP PARTITION p SETTINGS mutations_sync=1"
	stmt := asType[*AlterTable](t, parseOneStmt(t, sql))
	drop := asType[*AlterTableDropPartition](t, stmt.AlterExprs[0])
	require.NotNil(t, drop.Settings)
	// End() used to discard the Settings end and stop at the partition
	require.Equal(t, drop.Settings.End(), drop.End())
	require.Greater(t, drop.End(), drop.Partition.End())
}

func TestDictionaryAttributeEnd(t *testing.T) {
	sql := "CREATE DICTIONARY d (user_id UInt64 IS_OBJECT_ID) PRIMARY KEY user_id SOURCE(CLICKHOUSE()) LAYOUT(FLAT()) LIFETIME(300)"
	stmt := asType[*CreateDictionary](t, parseOneStmt(t, sql))
	attrs := stmt.Schema.Attributes
	require.Len(t, attrs, 1)
	// the attribute ends at IS_OBJECT_ID; the old implementation returned
	// NamePos + len("IS_OBJECT_ID"), landing in the middle of the type
	require.Equal(t, "IS_OBJECT_ID", sql[36:48])
	require.Equal(t, Pos(48), attrs[0].End())
}

func TestKeywordArgFunctionPositions(t *testing.T) {
	// The keyword forms reuse UnaryExpr/BinaryOperation, so their spans must match
	// what the same nodes report anywhere else.
	sql := "SELECT trim(BOTH ' ' FROM s)"
	stmt := asType[*SelectQuery](t, parseOneStmt(t, sql))
	fn := asType[*FunctionExpr](t, stmt.SelectItems[0].Expr)
	require.Equal(t, Pos(7), fn.Pos())
	require.Equal(t, Pos(len(sql)-1), fn.End(), "ends on the closing paren, as any call does")

	from := asType[*BinaryOperation](t, asType[*ColumnExpr](t, fn.Params.Items.Items[0]).Expr)
	require.Equal(t, "BOTH", sql[12:16])
	require.Equal(t, "s", sql[26:27])
	// the operation spans its operands: BOTH ... s
	require.Equal(t, Pos(12), from.Pos())
	require.Equal(t, Pos(27), from.End())

	// the characters literal runs from inside the opening quote to the closing one
	modifier := asType[*UnaryExpr](t, from.LeftExpr)
	require.Equal(t, Pos(12), modifier.Pos())
	require.Equal(t, modifier.Expr.End(), modifier.End())
	require.Equal(t, Pos(18), modifier.Expr.Pos())
	require.Equal(t, Pos(19), modifier.Expr.End())
}

func TestJoinLocalityIsInsideTheJoinSpan(t *testing.T) {
	sql := "SELECT * FROM t1 GLOBAL LEFT JOIN t2 ON t1.a = t2.a"
	stmt := asType[*SelectQuery](t, parseOneStmt(t, sql))
	join := asType[*JoinExpr](t, asType[*JoinExpr](t, stmt.From.Expr).Right)
	// the span starts at GLOBAL, so a source-rewriting consumer keeps the locality
	require.Equal(t, "GLOBAL", sql[17:23])
	require.Equal(t, Pos(17), join.Pos())
	require.Equal(t, Pos(len(sql)), join.End())
}
