package lexer

import (
	"testing"
)

func TestDoubleQuotedStrings(t *testing.T) {
	input := `"identifier" "another one" "special.chars_123"`

	l := New(input)

	expected := []struct {
		tokenType TokenType
		literal   string
	}{
		{DOUBLE_QUOTE, "identifier"},
		{SPACE, " "},
		{DOUBLE_QUOTE, "another one"},
		{SPACE, " "},
		{DOUBLE_QUOTE, "special.chars_123"},
		{EOF, ""},
	}

	for i, exp := range expected {
		tok := l.NextToken()
		
		if tok.Type != exp.tokenType {
			t.Fatalf("test[%d] - token type wrong. expected=%q, got=%q",
				i, exp.tokenType, tok.Type)
		}
		
		if tok.Literal != exp.literal {
			t.Fatalf("test[%d] - literal wrong. expected=%q, got=%q",
				i, exp.literal, tok.Literal)
		}
	}
}

func TestScientificNotationNumbers(t *testing.T) {
	input := `1.23e-4 1E+5 2.5e10 3E-2`

	l := New(input)

	expected := []struct {
		tokenType TokenType
		literal   string
	}{
		{NUMBER, "1.23e-4"},
		{SPACE, " "},
		{NUMBER, "1E+5"},
		{SPACE, " "},
		{NUMBER, "2.5e10"},
		{SPACE, " "},
		{NUMBER, "3E-2"},
		{EOF, ""},
	}

	for i, exp := range expected {
		tok := l.NextToken()
		
		if tok.Type != exp.tokenType {
			t.Fatalf("test[%d] - token type wrong. expected=%q, got=%q",
				i, exp.tokenType, tok.Type)
		}
		
		if tok.Literal != exp.literal {
			t.Fatalf("test[%d] - literal wrong. expected=%q, got=%q",
				i, exp.literal, tok.Literal)
		}
	}
}

func TestHexLiterals(t *testing.T) {
	input := `0x1A2F 0XFF 0x0 0xDEADBEEF`

	l := NewWithFormatting(input, false, true) // Keep whitespace but not comments

	expected := []struct {
		tokenType TokenType
		literal   string
	}{
		{HEX_LITERAL, "0x1A2F"},
		{SPACE, " "},
		{HEX_LITERAL, "0XFF"},
		{SPACE, " "},
		{HEX_LITERAL, "0x0"},
		{SPACE, " "},
		{HEX_LITERAL, "0xDEADBEEF"},
		{EOF, ""},
	}

	for i, exp := range expected {
		tok := l.NextToken()
		
		if tok.Type != exp.tokenType {
			t.Fatalf("test[%d] - token type wrong. expected=%q, got=%q",
				i, exp.tokenType, tok.Type)
		}
		
		if tok.Literal != exp.literal {
			t.Fatalf("test[%d] - literal wrong. expected=%q, got=%q",
				i, exp.literal, tok.Literal)
		}
	}
}

func TestBinaryLiterals(t *testing.T) {
	input := `0b1010 0B1111 0b0 0b10101010`

	l := New(input)

	expected := []struct {
		tokenType TokenType
		literal   string
	}{
		{BINARY_LITERAL, "0b1010"},
		{SPACE, " "},
		{BINARY_LITERAL, "0B1111"},
		{SPACE, " "},
		{BINARY_LITERAL, "0b0"},
		{SPACE, " "},
		{BINARY_LITERAL, "0b10101010"},
		{EOF, ""},
	}

	for i, exp := range expected {
		tok := l.NextToken()
		
		if tok.Type != exp.tokenType {
			t.Fatalf("test[%d] - token type wrong. expected=%q, got=%q",
				i, exp.tokenType, tok.Type)
		}
		
		if tok.Literal != exp.literal {
			t.Fatalf("test[%d] - literal wrong. expected=%q, got=%q",
				i, exp.literal, tok.Literal)
		}
	}
}

func TestUnicodeIdentifiers(t *testing.T) {
	input := "αβγ_δ εζη_θ ñame_ü"

	l := New(input)

	expected := []struct {
		tokenType TokenType
		literal   string
	}{
		{IDENT, "αβγ_δ"},
		{SPACE, " "},
		{IDENT, "εζη_θ"},
		{SPACE, " "},
		{IDENT, "ñame_ü"},
		{EOF, ""},
	}

	for i, exp := range expected {
		tok := l.NextToken()
		
		if tok.Type != exp.tokenType {
			t.Fatalf("test[%d] - token type wrong. expected=%q, got=%q",
				i, exp.tokenType, tok.Type)
		}
		
		if tok.Literal != exp.literal {
			t.Fatalf("test[%d] - literal wrong. expected=%q, got=%q",
				i, exp.literal, tok.Literal)
		}
	}
}

func TestKeywordsWithDateLiterals(t *testing.T) {
	input := "DATE TIME TIMESTAMP"

	l := New(input)

	expected := []struct {
		tokenType TokenType
		literal   string
	}{
		{DATE, "DATE"},
		{SPACE, " "},
		{TIME, "TIME"},
		{SPACE, " "},
		{TIMESTAMP, "TIMESTAMP"},
		{EOF, ""},
	}

	for i, exp := range expected {
		tok := l.NextToken()
		
		if tok.Type != exp.tokenType {
			t.Fatalf("test[%d] - token type wrong. expected=%q, got=%q",
				i, exp.tokenType, tok.Type)
		}
		
		if tok.Literal != exp.literal {
			t.Fatalf("test[%d] - literal wrong. expected=%q, got=%q",
				i, exp.literal, tok.Literal)
		}
	}
}