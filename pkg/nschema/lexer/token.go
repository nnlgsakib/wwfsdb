package lexer

type TokenType string

type Token struct {
	Type    TokenType
	Literal string
}

const (
	ILLEGAL = "ILLEGAL"
	EOF     = "EOF"

	// Keywords
	PRAGMA  = "PRAGMA"
	NSCHEMA = "NSCHEMA"
	MODEL   = "MODEL"
)
