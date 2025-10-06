package lexer

// Package lexer defines tokenizer interfaces for SQL.
// For complex SQL, we will wire this to a robust SQL tokenizer (e.g., Vitess).

// Token represents a lexical token.
type Token struct {
    Type TokenType
    Lit  string
}

type TokenType int

const (
    // Minimal placeholder tokens. Real implementation will be replaced.
    ILLEGAL TokenType = iota
    EOF
    IDENT
    STRING
)

// Lexer is the interface for SQL tokenizers.
type Lexer interface {
    Next() Token
}

// Simple is a trivial placeholder lexer. Not suitable for complex SQL.
type Simple struct{ s string; i int }

func NewSimple(input string) *Simple { return &Simple{s: input} }

func (l *Simple) Next() Token {
    // Placeholder: immediately return EOF to avoid unused warnings.
    return Token{Type: EOF}
}

