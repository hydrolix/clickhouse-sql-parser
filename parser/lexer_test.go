package parser

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConsumeComment(t *testing.T) {
	comments := []string{
		"-- hello world",
		"-- hello world\n",
		"-- hello world\r\n",
		"-- hello world\r",
		"/* hello world */",
		"/* hello world */\n",
		"/* hello world */\r\n",
		"/* hello world */\r",
		"/* hello world */ /* hello world */",
		"/* hello world */ /* hello world */\n",
		"/* hello world */ /* hello world */\r\n",
		"/* hello world */ /* hello world */\r",
	}
	for _, c := range comments {
		lexer := NewLexer(c)
		err := lexer.consumeToken()
		require.NoError(t, err)
	}

}

// Coverage gap from porting analysis: fork's lexer.skipComments treats `#` as
// a single-line comment trigger, but no test from origin/main exercises it.
// To actually exercise the comment behavior (not just "doesn't error on #"),
// we feed `# … \nSELECT` and assert the parser reaches the SELECT.
func TestConsumeHashComment(t *testing.T) {
	validSQLs := []string{
		"# leading comment\nSELECT 1",
		"SELECT 1 # trailing comment",
		"# c1\n# c2\nSELECT 1",
	}
	for _, sql := range validSQLs {
		parser := NewParser(sql)
		_, err := parser.ParseStmts()
		require.NoError(t, err, "Failed to parse with # comment: %q", sql)
	}
}

func TestConsumeString(t *testing.T) {
	t.Run("Simple strings", func(t *testing.T) {
		strs := []string{
			"'hello world'",
			"'123'",
		}
		for _, s := range strs {
			lexer := NewLexer(s)
			err := lexer.consumeToken()
			require.NoError(t, err)
			require.Equal(t, TokenKindString, lexer.lastToken.Kind)
			require.Equal(t, strings.Trim(s, "'"), lexer.lastToken.String)
			require.True(t, lexer.isEOF())
		}
	})

	t.Run("Strings with backslash-escaped quotes", func(t *testing.T) {
		testCases := []struct {
			input    string
			expected string
		}{
			{`'hello\'world'`, `hello\'world`},
			{`'test\''`, `test\'`},
			{`'\'abc\''`, `\'abc\'`},
		}
		for _, tc := range testCases {
			lexer := NewLexer(tc.input)
			err := lexer.consumeToken()
			require.NoError(t, err, "Failed to parse: %s", tc.input)
			require.Equal(t, TokenKindString, lexer.lastToken.Kind)
			require.Equal(t, tc.expected, lexer.lastToken.String)
			require.True(t, lexer.isEOF())
		}
	})

	t.Run("Strings with double single quotes", func(t *testing.T) {
		testCases := []struct {
			input    string
			expected string
		}{
			{`'hello''world'`, `hello''world`},
			{`'test''123'`, `test''123`},
			{`'abc''def''ghi'`, `abc''def''ghi`},
		}
		for _, tc := range testCases {
			lexer := NewLexer(tc.input)
			err := lexer.consumeToken()
			require.NoError(t, err, "Failed to parse: %s", tc.input)
			require.Equal(t, TokenKindString, lexer.lastToken.Kind)
			require.Equal(t, tc.expected, lexer.lastToken.String)
			require.True(t, lexer.isEOF())
		}
	})

	t.Run("Strings with backslash-escaped backslashes", func(t *testing.T) {
		testCases := []struct {
			input    string
			expected string
		}{
			{`'a\\b'`, `a\\b`},
			{`'test\\123'`, `test\\123`},
		}
		for _, tc := range testCases {
			lexer := NewLexer(tc.input)
			err := lexer.consumeToken()
			require.NoError(t, err, "Failed to parse: %s", tc.input)
			require.Equal(t, TokenKindString, lexer.lastToken.Kind)
			require.Equal(t, tc.expected, lexer.lastToken.String)
			require.True(t, lexer.isEOF())
		}
	})

	// Fork tests copied from origin/main: Grafana-style mixed quote variants.
	t.Run("Fork: Grafana-style mixed quotes", func(t *testing.T) {
		strs := []string{
			"'hello \\'perfect\\' world'",
			"'hello ''perfect'' world'",
			"'hello \\'''perfect'' world'",
			"'hello world'''",
			"'hello world\\''''",
		}
		for _, s := range strs {
			lexer := NewLexer(s)
			err := lexer.consumeToken()
			require.NoError(t, err)
			require.Equal(t, TokenKindString, lexer.lastToken.Kind)
			require.Equal(t, s[1:len(s)-1], lexer.lastToken.String)
			require.True(t, lexer.isEOF())
		}
	})
}

func TestConsumeTextBlock(t *testing.T) {
	strs := []string{
		"$$hello world$$",
		"$$123$$",
		"$$${variable:format} and 'string' $$",
	}
	for _, s := range strs {
		lexer := NewLexer(s)
		err := lexer.consumeToken()
		require.NoError(t, err)
		require.Equal(t, TokenKindString, lexer.lastToken.Kind)
		require.Equal(t, s[2:len(s)-2], lexer.lastToken.String)
		require.True(t, lexer.isEOF())
	}
}

func TestConsumeNumber(t *testing.T) {
	t.Run("Integer number", func(t *testing.T) {
		integers := []string{
			"123",
			"123e+10",
			"123e-10",
			"123e10",
			"123E10",
			"123E+10",
			"123E-10",
		}
		for _, i := range integers {
			lexer := NewLexer(i)
			err := lexer.consumeToken()
			require.NoError(t, err)
			require.Equal(t, TokenKindInt, lexer.lastToken.Kind)
			require.Equal(t, 10, lexer.lastToken.Base)
			require.Equal(t, i, lexer.lastToken.String)
			require.True(t, lexer.isEOF())
		}
	})

	t.Run("Hexadecimal number", func(t *testing.T) {
		numbers := []string{
			"0x123",
			"0x1",
		}
		for _, n := range numbers {
			lexer := NewLexer(n)
			err := lexer.consumeToken()
			require.NoError(t, err)
			require.Equal(t, TokenKindInt, lexer.lastToken.Kind)
			require.Equal(t, 16, lexer.lastToken.Base)
			require.Equal(t, n, lexer.lastToken.String)
			require.True(t, lexer.isEOF())
		}
	})

	t.Run("Float number", func(t *testing.T) {
		floats := []string{
			"123.456",
			"123.456e+10",
			"123.456e-10",
			"123.456e10",
			"123.456E10",
			"123.456E+10",
			"123.456E-10",
		}
		for _, f := range floats {
			lexer := NewLexer(f)
			err := lexer.consumeToken()
			require.NoError(t, err)
			require.Equal(t, TokenKindFloat, lexer.lastToken.Kind)
			require.Equal(t, f, lexer.lastToken.String)
			require.True(t, lexer.isEOF())
		}
	})

	t.Run("Name", func(t *testing.T) {
		idents := []string{
			"`CASE`",
			"`TEST`",
			"`WHEN`",
			"hello",
			"hello_world",
			"hello123",
			"hello_123",
			"hello_123_world",
			"hello_123_world_456",
			"hello_123_world_456_789",
			"hello_123_world_456_789_abc",
			"hello_123_world_456_789_abc_def",
			"hello_123_world_456_789_abc_def_ghi",
			"hello_123_world_456_789_abc_def_ghi_jkl",
			"hello_123_world_456_789_abc_def_ghi_jkl_mno",
			"hello_123_world_456_789_abc_def_ghi_jkl_mno_pqr",
		}
		for _, i := range idents {
			lexer := NewLexer(i)
			err := lexer.consumeToken()
			require.NoError(t, err)
			require.Equal(t, TokenKindIdent, lexer.lastToken.Kind)
			require.Equal(t, strings.Trim(i, "`"), lexer.lastToken.String)
			require.True(t, lexer.isEOF())
		}
	})

	// Fork tests copied from origin/main: invalid numbers should not error,
	// they should fall back to a non-Int/non-Float token kind.
	t.Run("Fork: Invalid number returns non-number kind without error", func(t *testing.T) {
		invalidNumbers := []string{
			"123e",
			"123e+",
			"123e-",
			"123E",
			"123E+",
			"123E-",
			"0x",
			"0xg",
		}
		for _, n := range invalidNumbers {
			lexer := NewLexer(n)
			err := lexer.consumeToken()
			require.NoError(t, err)
			require.NotEqual(t, lexer.lastToken.Kind, TokenKindInt)
			require.NotEqual(t, lexer.lastToken.Kind, TokenKindFloat)
		}
	})

	// Fork tests copied from origin/main: invalid floats should not error,
	// they should fall back to a non-Int/non-Float token kind.
	t.Run("Fork: Invalid float returns non-number kind without error", func(t *testing.T) {
		invalidFloats := []string{
			"123.456b",
			"123.456e",
			"123.456e+",
			"123.456e-",
			"123.456e+10e",
			"123.456e-10e",
			"123.456e10e",
			"123.456E10e",
			"123.456E+10e",
			"123.456E-10e",
			"123.456e+10e+10",
		}
		for _, f := range invalidFloats {
			lexer := NewLexer(f)
			err := lexer.consumeToken()
			require.NoError(t, err)
			require.NotEqual(t, lexer.lastToken.Kind, TokenKindInt)
			require.NotEqual(t, lexer.lastToken.Kind, TokenKindFloat)
		}
	})

	// Fork tests copied from origin/main: identifier allowed to start with a digit.
	t.Run("Fork: Identifier with leading digit", func(t *testing.T) {
		lexer := NewLexer("1hello_123_world_456_789_abc_def_ghi_jkl_mno_pqr")
		err := lexer.consumeToken()
		require.NoError(t, err)
		require.Equal(t, TokenKindIdent, lexer.lastToken.Kind)
		require.Equal(t, "1hello_123_world_456_789_abc_def_ghi_jkl_mno_pqr", lexer.lastToken.String)
		require.True(t, lexer.isEOF())
	})

	t.Run("Keyword", func(t *testing.T) {
		for _, k := range keywords.Members() {
			lexer := NewLexer(k)
			err := lexer.consumeToken()
			require.NoError(t, err)
			require.Equal(t, TokenKindKeyword, lexer.lastToken.Kind)
			require.Equal(t, k, lexer.lastToken.String)
			require.True(t, lexer.isEOF())
		}
	})
}
