package lexer

import (
	"testing"
)

func TestFormattingTokens(t *testing.T) {
	input := `SELECT  id,   name
FROM    users;`

	l := NewWithFormatting(input, false, true) // Keep whitespace but not comments

	expected := []struct {
		tokenType TokenType
		literal   string
	}{
		{SELECT, "SELECT"},
		{SPACE, " "},
		{SPACE, " "},
		{IDENT, "id"},
		{COMMA, ","},
		{SPACE, " "},
		{SPACE, " "},
		{SPACE, " "},
		{IDENT, "name"},
		{NEWLINE, "\n"},
		{FROM, "FROM"},
		{SPACE, " "},
		{SPACE, " "},
		{SPACE, " "},
		{SPACE, " "},
		{IDENT, "users"},
		{SEMICOLON, ";"},
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

func TestCommentTokens(t *testing.T) {
	input := `SELECT id -- comment
FROM /* multi
line */ users;`

	l := NewWithFormatting(input, true, false) // Keep comments but not whitespace

	expected := []struct {
		tokenType TokenType
		literal   string
	}{
		{SELECT, "SELECT"},
		{IDENT, "id"},
		{COMMENT_SINGLE, "-- comment"},
		{FROM, "FROM"},
		{COMMENT_MULTI, "/*  multi\nline  */"},
		{IDENT, "users"},
		{SEMICOLON, ";"},
		{EOF, ""},
	}

	for i, exp := range expected {
		tok := l.NextToken()
		
		if tok.Type != exp.tokenType {
			t.Fatalf("test[%d] - token type wrong. expected=%q, got=%q, literal=%q",
				i, exp.tokenType, tok.Type, tok.Literal)
		}
		
		if tok.Literal != exp.literal {
			t.Fatalf("test[%d] - literal wrong. expected=%q, got=%q",
				i, exp.literal, tok.Literal)
		}
	}
}

func TestFormatterTokensBoth(t *testing.T) {
	input := `SELECT id -- comment

FROM users;`

	l := NewWithFormatting(input, true, true) // Keep both comments and whitespace

	// Just make sure we can tokenize without errors
	tokenCount := 0
	for {
		tok := l.NextToken()
		tokenCount++
		if tok.Type == EOF {
			break
		}
		// Make sure all tokens have proper position information
		if tok.Position.Line == 0 || tok.Position.Offset < 0 {
			t.Errorf("Token %d (%s) has invalid position: line=%d, col=%d, offset=%d", 
				tokenCount, tok.Literal, tok.Position.Line, tok.Position.Column, tok.Position.Offset)
		}
	}

	// We should have more tokens when preserving formatting
	if tokenCount < 10 { // Should have many tokens including whitespace and newlines
		t.Errorf("Expected at least 10 tokens when preserving formatting, got %d", tokenCount)
	}
}