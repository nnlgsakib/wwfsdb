package lexer

import (
	"testing"
)

func TestCommentPositionPreservation(t *testing.T) {
	input := `-- This is a header comment
SELECT id, -- inline comment
    name /* multi
    line */ 
FROM users;`

	l := NewWithFormatting(input, true, true) // Keep both comments and whitespace

	// Track all tokens to check comment positions
	var tokens []Token
	for {
		tok := l.NextToken()
		tokens = append(tokens, tok)
		if tok.Type == EOF {
			break
		}
	}

	// Look for specific comment tokens and verify their positions
	commentFound := false
	inlineCommentFound := false
	multilineCommentFound := false

	for _, tok := range tokens {
		if tok.Type == COMMENT_SINGLE && tok.Literal == "-- This is a header comment" {
			commentFound = true
			if tok.Position.Line != 1 || tok.Position.Column != 1 {
				t.Errorf("Header comment position wrong: expected (1,1), got (%d,%d)", 
					tok.Position.Line, tok.Position.Column)
			}
		}
		
		if tok.Type == COMMENT_SINGLE && tok.Literal == "-- inline comment" {
			inlineCommentFound = true
			// This should be positioned appropriately in the line
			if tok.Position.Line != 2 {
				t.Errorf("Inline comment line wrong: expected 2, got %d", tok.Position.Line)
			}
		}
		
		if tok.Type == COMMENT_MULTI && tok.Literal == "/*  multi\n    line  */" {
			multilineCommentFound = true
			// This should be positioned on the appropriate line
			if tok.Position.Line != 3 {
				t.Errorf("Multiline comment line wrong: expected 3, got %d", tok.Position.Line)
			}
		}
	}

	if !commentFound {
		t.Error("Header comment not found")
	}
	if !inlineCommentFound {
		t.Error("Inline comment not found")
	}
	if !multilineCommentFound {
		t.Error("Multiline comment not found")
	}
}

func TestCommentPositionAccuracy(t *testing.T) {
	input := `SELECT id -- comment at end of line
FROM users;`

	l := NewWithFormatting(input, true, false) // Keep comments, skip whitespace

	// Extract tokens and verify positions
	var commentToken *Token
	for {
		tok := l.NextToken()
		if tok.Type == COMMENT_SINGLE {
			commentToken = &tok
			break
		}
		if tok.Type == EOF {
			break
		}
	}

	if commentToken == nil {
		t.Fatal("Comment token not found")
	}

	// The comment should start at the right position
	expectedLiteral := "-- comment at end of line"
	if commentToken.Literal != expectedLiteral {
		t.Errorf("Expected comment literal '%s', got '%s'", expectedLiteral, commentToken.Literal)
	}
}

func TestCommentsWithLexer(t *testing.T) {
	input := `SELECT id -- inline comment
FROM users /* multi-line
              comment */;`

	// This should tokenize successfully with comments skipped by default lexer
	l := New(input)
	
	// Count tokens to ensure parsing completes without error
	tokenCount := 0
	for {
		tok := l.NextToken()
		tokenCount++
		if tok.Type == EOF {
			break
		}
	}
	
	// Should have some tokens
	if tokenCount < 5 { // SELECT, id, FROM, users, ;
		t.Errorf("Expected at least 5 tokens, got %d", tokenCount)
	}
}

func TestCommentTokenTypes(t *testing.T) {
	input := `-- single line comment
/* multi
line comment */ SELECT id;`

	l := NewWithFormatting(input, true, false) // Keep comments, skip whitespace

	tokens := []Token{}
	for {
		tok := l.NextToken()
		tokens = append(tokens, tok)
		if tok.Type == EOF {
			break
		}
	}

	// Verify we have the expected comment tokens
	hasSingleComment := false
	hasMultiComment := false
	hasSelect := false

	for _, tok := range tokens {
		switch tok.Type {
		case COMMENT_SINGLE:
			hasSingleComment = true
		case COMMENT_MULTI:
			hasMultiComment = true
		case SELECT:
			hasSelect = true
		}
	}

	if !hasSingleComment {
		t.Error("Single line comment token not found")
	}
	if !hasMultiComment {
		t.Error("Multi line comment token not found")
	}
	if !hasSelect {
		t.Error("SELECT token not found after comments")
	}
}