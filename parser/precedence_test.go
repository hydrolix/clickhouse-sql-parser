package parser

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// parseOneStmt parses a single statement and returns it.
func parseOneStmt(t *testing.T, sql string) Expr {
	t.Helper()
	stmts, err := NewParser(sql).ParseStmts()
	require.NoError(t, err, sql)
	require.Len(t, stmts, 1, sql)
	return stmts[0]
}

// parseSelectItemExpr parses a single-statement SELECT and returns the first
// projection expression, so tests can assert the tree structure directly.
func parseSelectItemExpr(t *testing.T, sql string) Expr {
	t.Helper()
	selectQuery, ok := parseOneStmt(t, sql).(*SelectQuery)
	require.True(t, ok, "expected *SelectQuery for %s", sql)
	require.NotEmpty(t, selectQuery.SelectItems)
	return selectQuery.SelectItems[0].Expr
}

// parseTableFunctionExpr parses a single-statement SELECT and returns the
// table function in its FROM clause.
func parseTableFunctionExpr(t *testing.T, sql string) *TableFunctionExpr {
	t.Helper()
	from := parseOneStmt(t, sql).(*SelectQuery).From.Expr
	table, ok := from.(*JoinTableExpr)
	require.True(t, ok, "%s: expected *JoinTableExpr in FROM, got %T", sql, from)
	fn, ok := table.Table.Expr.(*TableFunctionExpr)
	require.True(t, ok, "%s: expected *TableFunctionExpr, got %T", sql, table.Table.Expr)
	return fn
}

func TestSignedNumberAfterClosingBracketIsBinaryOperator(t *testing.T) {
	// A closing `)` or `]` ends an expression, so the following `+`/`-` is a
	// binary operator and not the sign of the next numeric literal.
	for _, sql := range []string{"SELECT (1)-1", "SELECT arr[1]-1", "SELECT f()-1"} {
		expr := parseSelectItemExpr(t, sql)
		op, ok := expr.(*BinaryOperation)
		require.True(t, ok, "%s: expected BinaryOperation at the top, got %T", sql, expr)
		require.Equal(t, TokenKindMinus, op.Operation, sql)
		right, ok := op.RightExpr.(*NumberLiteral)
		require.True(t, ok, "%s: right side should be the unsigned literal `1`, got %T", sql, op.RightExpr)
		require.Equal(t, "1", right.Literal, sql)
	}

	// A `+`/`-` after an opening bracket is still a sign, not an operator.
	expr := parseSelectItemExpr(t, "SELECT arr[-1]")
	require.Equal(t, "arr[-1]", Format(expr))

	// `arr[1]-1 FROM t` parses in a full statement.
	_, err := NewParser("SELECT arr[1]-1 FROM t").ParseStmts()
	require.NoError(t, err)
}

func TestTableFunctionArgAcceptsOperatorExpressions(t *testing.T) {
	// a table-function argument continues into the operator loop, so the
	// argument is the whole `1 + 1`, not just the first literal
	fn := parseTableFunctionExpr(t, "SELECT * FROM numbers(1 + 1)")
	require.Len(t, fn.Args.Args, 1)
	op, ok := fn.Args.Args[0].(*BinaryOperation)
	require.True(t, ok, "argument should be the binary operation `1 + 1`, got %T", fn.Args.Args[0])
	require.Equal(t, TokenKindPlus, op.Operation)

	// a nested call keeps its *TableFunctionExpr shape as the left operand
	fn = parseTableFunctionExpr(t, "SELECT * FROM numbers(intDiv(a, b) + 1)")
	op, ok = fn.Args.Args[0].(*BinaryOperation)
	require.True(t, ok, "argument should be a binary operation, got %T", fn.Args.Args[0])
	_, ok = op.LeftExpr.(*TableFunctionExpr)
	require.True(t, ok, "left operand should stay a *TableFunctionExpr, got %T", op.LeftExpr)

	for _, sql := range []string{
		"SELECT * FROM numbers(greatest(1, a - b))",
		"SELECT * FROM cluster('c', numbers(1 + 1))",
		"SELECT * FROM numbers(x AND y)",
	} {
		require.Equal(t, sql, Format(parseOneStmt(t, sql)), sql)
	}

	for _, sql := range []string{
		"SELECT number FROM numbers(toUInt32(dateDiff('hour', toDateTime(1), toDateTime(7200))) + 1)",
		"SELECT * FROM numbers(10 + 1)",
	} {
		parseOneStmt(t, sql)
	}
}

func TestTableFunctionArgFloatAndZeroArgumentCall(t *testing.T) {
	fn := parseTableFunctionExpr(t, "SELECT * FROM numbers(1.5)")
	num, ok := fn.Args.Args[0].(*NumberLiteral)
	require.True(t, ok, "argument should be a *NumberLiteral, got %T", fn.Args.Args[0])
	require.Equal(t, "1.5", num.Literal)

	fn = parseTableFunctionExpr(t, "SELECT * FROM numbers(now())")
	nested, ok := fn.Args.Args[0].(*TableFunctionExpr)
	require.True(t, ok, "argument should be the zero-argument call, got %T", fn.Args.Args[0])
	require.Empty(t, nested.Args.Args)
}

func TestTableFunctionArgParenthesizedExpression(t *testing.T) {
	// a leading '(' that does not open a subquery is a parenthesized
	// expression, with the shape the scalar `SELECT (1 + 1)` produces
	fn := parseTableFunctionExpr(t, "SELECT * FROM numbers((1 + 1))")
	params, ok := fn.Args.Args[0].(*ParamExprList)
	require.True(t, ok, "argument should be a *ParamExprList, got %T", fn.Args.Args[0])
	require.Len(t, params.Items.Items, 1)
	item, ok := params.Items.Items[0].(*ColumnExpr)
	require.True(t, ok, "parenthesized item should be a *ColumnExpr, got %T", params.Items.Items[0])
	_, ok = item.Expr.(*BinaryOperation)
	require.True(t, ok, "parenthesized argument should hold `1 + 1`, got %T", item.Expr)

	sql := "SELECT * FROM numbers((a + b) * c)"
	require.Equal(t, sql, Format(parseOneStmt(t, sql)))

	// a '(' that opens a subquery still parses as one
	fn = parseTableFunctionExpr(t, "SELECT * FROM remote('127.0.0.1', (SELECT 1))")
	_, ok = fn.Args.Args[1].(*SubQuery)
	require.True(t, ok, "second argument should stay a *SubQuery, got %T", fn.Args.Args[1])
}

func TestTableFunctionArgShapesUnchanged(t *testing.T) {
	// a signed literal is still one literal, not a unary operation
	fn := parseTableFunctionExpr(t, "SELECT * FROM numbers(-1)")
	num, ok := fn.Args.Args[0].(*NumberLiteral)
	require.True(t, ok, "argument should stay the signed literal `-1`, got %T", fn.Args.Args[0])
	require.Equal(t, "-1", num.Literal)

	// a qualified name is still a *NestedIdentifier, not an IndexOperation
	fn = parseTableFunctionExpr(t, "SELECT * FROM cluster('c', db.table)")
	require.Len(t, fn.Args.Args, 2)
	_, ok = fn.Args.Args[1].(*NestedIdentifier)
	require.True(t, ok, "second argument should stay a *NestedIdentifier, got %T", fn.Args.Args[1])
}
