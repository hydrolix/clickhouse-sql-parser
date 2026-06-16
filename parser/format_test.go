package parser

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFormatter_WithBeautify_Chaining(t *testing.T) {
	// Test that WithBeautify returns the formatter for chaining
	formatter := NewFormatter().WithBeautify()
	require.NotNil(t, formatter)
	require.Equal(t, FormatModeBeautify, formatter.mode)
}

func TestFormatter_WithIndent_Chaining(t *testing.T) {
	// Test that WithIndent returns the formatter for chaining
	formatter := NewFormatter().WithIndent("    ")
	require.NotNil(t, formatter)
	require.Equal(t, "    ", formatter.indent)
}

func TestFormatter_ChainedMethods(t *testing.T) {
	// Test that methods can be chained together
	formatter := NewFormatter().WithBeautify().WithIndent("\t")
	require.NotNil(t, formatter)
	require.Equal(t, FormatModeBeautify, formatter.mode)
	require.Equal(t, "\t", formatter.indent)
}

func TestFormatter_WithIndent_CustomIndentation(t *testing.T) {
	// Test actual formatting with custom indent using parsed SQL
	sql := "SELECT col1, col2 FROM table1 WHERE col1 > 10"

	parser := NewParser(sql)
	stmts, err := parser.ParseStmts()
	require.NoError(t, err)
	require.Len(t, stmts, 1)

	// Test with default 2-space indent
	formatter1 := NewFormatter().WithBeautify()
	formatter1.WriteExpr(stmts[0])
	result1 := formatter1.String()

	// Test with 4-space indent
	formatter2 := NewFormatter().WithBeautify().WithIndent("    ")
	formatter2.WriteExpr(stmts[0])
	result2 := formatter2.String()

	// Test with tab indent
	formatter3 := NewFormatter().WithBeautify().WithIndent("\t")
	formatter3.WriteExpr(stmts[0])
	result3 := formatter3.String()

	// Verify all results are different (due to different indentation)
	require.NotEqual(t, result1, result2)
	require.NotEqual(t, result1, result3)
	require.NotEqual(t, result2, result3)

	// Verify they all contain the basic SQL keywords
	require.Contains(t, result1, "SELECT")
	require.Contains(t, result2, "SELECT")
	require.Contains(t, result3, "SELECT")
	require.Contains(t, result1, "FROM")
	require.Contains(t, result2, "FROM")
	require.Contains(t, result3, "FROM")
}

func TestFormatter_DefaultIndent(t *testing.T) {
	// Test that default indent is 2 spaces
	formatter := NewFormatter()
	require.Equal(t, "  ", formatter.indent)
}

// Dollar-quoted `$$ … $$` literals are a lexer-only extension: the formatter
// normalises them to single-quoted form. This exercises the full pipeline
// (parse → format → parse → format) and asserts the second format is a fixed
// point — the canonical idempotence property already used elsewhere in the
// suite via validFormatSQL.
func TestFormatter_DollarQuotedRoundtrip(t *testing.T) {
	testCases := []struct {
		name   string
		input  string
		format string
	}{
		{
			name:   "Simple text block",
			input:  "SELECT $$hello world$$",
			format: "SELECT 'hello world';\n",
		},
		{
			name:   "Numeric text block",
			input:  "SELECT $$123$$",
			format: "SELECT '123';\n",
		},
		{
			name:   "Empty text block",
			input:  "SELECT $$$$",
			format: "SELECT '';\n",
		},
		{
			name:   "Brace-wrapped placeholder is preserved verbatim",
			input:  "SELECT $$${variable:format}$$",
			format: "SELECT '${variable:format}';\n",
		},
		{
			name:   "Comment-like content is preserved verbatim",
			input:  "SELECT $$-- not a comment$$",
			format: "SELECT '-- not a comment';\n",
		},
		{
			name:   "Block-comment-like content is preserved verbatim",
			input:  "SELECT $$/* nor this */$$",
			format: "SELECT '/* nor this */';\n",
		},
		{
			name:   "Single dollar still flows into identifier path",
			input:  "SELECT $col FROM t",
			format: "SELECT $col FROM t;\n",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			parser := NewParser(tc.input)
			stmts, err := parser.ParseStmts()
			require.NoError(t, err, "Failed to parse: %s", tc.input)

			var builder strings.Builder
			for _, stmt := range stmts {
				builder.WriteString(Format(stmt))
				builder.WriteByte(';')
				builder.WriteByte('\n')
			}
			formatted := builder.String()
			require.Equal(t, tc.format, formatted)

			// Second iteration must be a fixed point: re-parsing and
			// re-formatting `formatted` yields `formatted` byte-for-byte.
			validFormatSQL(t, formatted)
		})
	}
}
