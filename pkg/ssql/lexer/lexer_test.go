package lexer

import (
	"testing"
)

func TestPositionTracking(t *testing.T) {
	input := `SELECT id, name 
FROM users 
WHERE id = 1;`

	l := New(input)

	// Test that positions are correctly tracked
	expected := []struct {
		tokenType TokenType
		literal   string
		line      int
		column    int
	}{
		{SELECT, "SELECT", 1, 1},
		{IDENT, "id", 1, 8},
		{COMMA, ",", 1, 10},
		{IDENT, "name", 1, 12},
		{FROM, "FROM", 2, 1},
		{IDENT, "users", 2, 6},
		{WHERE, "WHERE", 3, 1},
		{IDENT, "id", 3, 7},
		{ASSIGN, "=", 3, 10},
		{NUMBER, "1", 3, 12},
		{SEMICOLON, ";", 3, 13},
		{EOF, "", 3, 14},
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
		
		if tok.Position.Line != exp.line {
			t.Fatalf("test[%d] - line wrong. expected=%d, got=%d",
				i, exp.line, tok.Position.Line)
		}
		
		if tok.Position.Column != exp.column {
			t.Fatalf("test[%d] - column wrong. expected=%d, got=%d",
				i, exp.column, tok.Position.Column)
		}
	}
}

func TestMultilineCommentHandling(t *testing.T) {
	input := `SELECT /* This is a 
multi-line comment */ id FROM users;`

	l := New(input)

	// Skip to the ID token (after the comment)
	var tok Token
	for {
		tok = l.NextToken()
		if tok.Type == IDENT && tok.Literal == "id" {
			break
		}
		if tok.Type == EOF {
			t.Fatal("Expected to find 'id' token but reached EOF")
		}
	}

	// Verify the position of the 'id' token
	if tok.Position.Line != 2 {
		t.Errorf("Expected 'id' to be on line 2, got line %d", tok.Position.Line)
	}
}

func TestSingleLineCommentHandling(t *testing.T) {
	input := `SELECT id -- This is a comment
FROM users;`

	l := New(input)

	var tok Token
	for {
		tok = l.NextToken()
		if tok.Type == FROM {
			break
		}
		if tok.Type == EOF {
			t.Fatal("Expected to find 'FROM' token but reached EOF")
		}
	}

	// Verify the position of the 'FROM' token
	if tok.Position.Line != 2 {
		t.Errorf("Expected 'FROM' to be on line 2, got line %d", tok.Position.Line)
	}
}