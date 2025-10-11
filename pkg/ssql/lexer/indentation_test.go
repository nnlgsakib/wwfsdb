package lexer

import (
	"testing"
)

func TestIndentationStyles(t *testing.T) {
	// Test different indentation styles
	testCases := []struct {
		name  string
		input string
	}{
		{
			name: "Spaces indentation",
			input: `SELECT id,
    name,
        email
FROM users;`,
		},
		{
			name: "Tabs indentation",
			input: "SELECT id,\n\tname,\n\t\temail\nFROM users;",
		},
		{
			name: "Mixed indentation",
			input: `SELECT id,
 name,
  email
FROM users;`,
		},
		{
			name: "Deep indentation",
			input: `SELECT 
        id,
        name,
        email
    FROM 
        users
    WHERE 
            id = 1;`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			l := NewWithFormatting(tc.input, false, true) // Keep whitespace

			// Count total tokens to ensure we can parse correctly
			tokenCount := 0
			var lastPos int
			for {
				tok := l.NextToken()
				tokenCount++
				
				// Check that positions are valid and increasing in offset
				if tok.Position.Offset < lastPos && tok.Type != EOF {
					t.Errorf("Position offset should be non-decreasing, got %d after %d", 
						tok.Position.Offset, lastPos)
				}
				lastPos = tok.Position.Offset
				
				if tok.Type == EOF {
					break
				}
			}

			// Should have more tokens when preserving whitespace
			if tokenCount < 10 { // Basic check - should have many tokens including whitespace
				t.Errorf("Expected many tokens for formatted input, got %d", tokenCount)
			}
		})
	}
}

func TestIndentationPositionTracking(t *testing.T) {
	input := `SELECT 
    id,
        name
FROM users;`

	l := New(input)

	// Track specific tokens and their positions
	tokens := []Token{}
	for {
		tok := l.NextToken()
		tokens = append(tokens, tok)
		if tok.Type == EOF {
			break
		}
	}

	// Find the 'name' identifier token
	nameFound := false
	for _, tok := range tokens {
		if tok.Type == IDENT && tok.Literal == "name" {
			nameFound = true
			// 'name' should be on line 3 with indentation
			if tok.Position.Line != 3 {
				t.Errorf("Expected 'name' to be on line 3, got line %d", tok.Position.Line)
			}
			if tok.Position.Column < 9 { // Should be at least column 9 with the indentation
				t.Errorf("Expected 'name' to be at column >= 9, got column %d", tok.Position.Column)
			}
			break
		}
	}

	if !nameFound {
		t.Error("Could not find 'name' identifier in tokens")
	}
}

func TestNewlineHandling(t *testing.T) {
	input := "SELECT id FROM users;\n\nSELECT name FROM users;"

	l := New(input)

	// Should be able to process the input without errors
	tokenCount := 0
	for {
		tok := l.NextToken()
		tokenCount++
		if tok.Type == EOF {
			break
		}
	}

	// Should have many tokens
	if tokenCount < 10 {
		t.Errorf("Expected many tokens for multi-line input, got %d", tokenCount)
	}
}