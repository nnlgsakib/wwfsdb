package ssql

import (
	"testing"
)

func TestParseMultilineSQL(t *testing.T) {
	input := `SELECT id, 
	name
FROM users 
WHERE id = 1;`

	stmt, err := Parse(input)
	if err != nil {
		t.Fatalf("Error parsing multiline SQL: %v", err)
	}
	
	if stmt == nil {
		t.Fatal("Expected statement but got nil")
	}
}

func TestParseWithComments(t *testing.T) {
	input := `SELECT id -- This is a comment
FROM users;`

	// This should work with the original parser (comments are skipped)
	stmt, err := Parse(input)
	if err != nil {
		t.Fatalf("Error parsing SQL with comments: %v", err)
	}
	
	if stmt == nil {
		t.Fatal("Expected statement but got nil")
	}
}

func TestParseMultipleStatements(t *testing.T) {
	input := `SELECT id FROM users; SELECT name FROM users;`

	stmts, err := ParseMultiple(input)
	if err != nil {
		t.Fatalf("Error parsing multiple statements: %v", err)
	}
	
	if len(stmts) != 2 {
		t.Fatalf("Expected 2 statements, got %d", len(stmts))
	}
}

func TestParseSimple(t *testing.T) {
	input := `SELECT id FROM users;`

	stmt, err := Parse(input)
	if err != nil {
		t.Fatalf("Error parsing simple statement: %v", err)
	}
	
	if stmt == nil {
		t.Fatal("Expected statement but got nil")
	}
}