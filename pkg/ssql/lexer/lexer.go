package lexer

import "strings"

// Token represents a lexical token.
type Token struct {
	Type    TokenType
	Literal string
}

type TokenType string

const (
	ILLEGAL = "ILLEGAL"
	EOF     = "EOF"

	// Identifiers & literals
	IDENT  = "IDENT"  // add, foobar, x, y, ...
	STRING = "STRING" // "foobar" or 'foobar'
	NUMBER = "NUMBER"

	// Operators
	ASSIGN   = "="
	GT       = ">"
	LT       = "<"
	GTE      = ">="
	LTE      = "<="
	NE       = "!="
	MINUS    = "-"

	// Delimiters
	COMMA     = ","
	SEMICOLON = ";"
	LPAREN    = "("
	RPAREN    = ")"
	ASTERISK  = "*"
	DOT       = "."

	// Keywords
	CREATE   = "CREATE"
	DATABASE = "DATABASE"
	TABLE    = "TABLE"
	ALTER    = "ALTER"
	ADD      = "ADD"
	COLUMN   = "COLUMN"
	RENAME   = "RENAME"
	TO       = "TO"
	DROP     = "DROP"
	SELECT   = "SELECT"
	FROM     = "FROM"
	WHERE    = "WHERE"
	INSERT   = "INSERT"
	INTO     = "INTO"
	VALUES   = "VALUES"
	UPDATE   = "UPDATE"
	SET      = "SET"
	DELETE   = "DELETE"
	AND      = "AND"
	OR       = "OR"
	LIKE     = "LIKE"
	IN       = "IN"
	INDEX    = "INDEX"
	ON       = "ON"
	TRUE     = "TRUE"
	FALSE    = "FALSE"
	BEGIN    = "BEGIN"
	COMMIT   = "COMMIT"
	ROLLBACK = "ROLLBACK"
	JOIN     = "JOIN"
	INNER    = "INNER"
	LEFT     = "LEFT"
	ORDER    = "ORDER"
	BY       = "BY"
	ASC      = "ASC"
	DESC     = "DESC"
	LIMIT    = "LIMIT"
	OFFSET   = "OFFSET"
	GROUP    = "GROUP"
	COUNT    = "COUNT"
	SUM      = "SUM"
	AVG      = "AVG"
	MIN      = "MIN"
	MAX      = "MAX"
)

var keywords = map[string]TokenType{
	"CREATE":   CREATE,
	"DATABASE": DATABASE,
	"TABLE":    TABLE,
	"ALTER":    ALTER,
	"ADD":      ADD,
	"COLUMN":   COLUMN,
	"RENAME":   RENAME,
	"TO":       TO,
	"DROP":     DROP,
	"SELECT":   SELECT,
	"FROM":     FROM,
	"WHERE":    WHERE,
	"INSERT":   INSERT,
	"INTO":     INTO,
	"VALUES":   VALUES,
	"UPDATE":   UPDATE,
	"SET":      SET,
	"DELETE":   DELETE,
	"AND":      AND,
	"OR":       OR,
	"LIKE":     LIKE,
	"IN":       IN,
	"INDEX":    INDEX,
	"ON":       ON,
	"TRUE":     TRUE,
	"FALSE":    FALSE,
	"BEGIN":    BEGIN,
	"COMMIT":   COMMIT,
	"ROLLBACK": ROLLBACK,
	"JOIN":     JOIN,
	"INNER":    INNER,
	"LEFT":     LEFT,
	"ORDER":    ORDER,
	"BY":       BY,
	"ASC":      ASC,
	"DESC":     DESC,
	"LIMIT":    LIMIT,
	"OFFSET":   OFFSET,
	"GROUP":    GROUP,
	"COUNT":    COUNT,
	"SUM":      SUM,
	"AVG":      AVG,
	"MIN":      MIN,
	"MAX":      MAX,
}

func LookupIdent(ident string) TokenType {
	if tok, ok := keywords[strings.ToUpper(ident)]; ok {
		return tok
	}
	return IDENT
}

// Lexer is the interface for SQL tokenizers.
type Lexer struct {
	input        string
	position     int  // current position in input (points to current char)
	readPosition int  // current reading position in input (after current char)
	ch           byte // current char under examination
}

func New(input string) *Lexer {
	l := &Lexer{input: input}
	l.readChar()
	return l
}

func (l *Lexer) readChar() {
	if l.readPosition >= len(l.input) {
		l.ch = 0
	} else {
		l.ch = l.input[l.readPosition]
	}
	l.position = l.readPosition
	l.readPosition++
}

func (l *Lexer) NextToken() Token {
	var tok Token

	l.skipWhitespace()

	switch l.ch {
	case '=':
		tok = newToken(ASSIGN, l.ch)
	case '>':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok = Token{Type: GTE, Literal: string(ch) + string(l.ch)}
		} else {
			tok = newToken(GT, l.ch)
		}
	case '<':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok = Token{Type: LTE, Literal: string(ch) + string(l.ch)}
		} else {
			tok = newToken(LT, l.ch)
		}
	case '!':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok = Token{Type: NE, Literal: string(ch) + string(l.ch)}
		} else {
			tok = newToken(ILLEGAL, l.ch)
		}
	case ';':
		tok = newToken(SEMICOLON, l.ch)
	case '(':
		tok = newToken(LPAREN, l.ch)
	case ')':
		tok = newToken(RPAREN, l.ch)
	case ',':
		tok = newToken(COMMA, l.ch)
	case '*':
		tok = newToken(ASTERISK, l.ch)
	case '.':
		tok = newToken(DOT, l.ch)
	case '-':
		tok = newToken(MINUS, l.ch)
	case '\'':
		tok.Type = STRING
		tok.Literal = l.readString()
	case 0:
		tok.Literal = ""
		tok.Type = EOF
	default:
		if isLetter(l.ch) {
			tok.Literal = l.readIdentifier()
			tok.Type = LookupIdent(tok.Literal)
			return tok
		} else if isDigit(l.ch) {
			tok.Type = NUMBER
			tok.Literal = l.readNumber()
			return tok
		} else {
			tok = newToken(ILLEGAL, l.ch)
		}
	}

	l.readChar()
	return tok
}

func (l *Lexer) peekChar() byte {
	if l.readPosition >= len(l.input) {
		return 0
	} else {
		return l.input[l.readPosition]
	}
}

func (l *Lexer) skipWhitespace() {
	for l.ch == ' ' || l.ch == '\t' || l.ch == '\n' || l.ch == '\r' {
		l.readChar()
	}
	
	// Handle SQL comments
	for l.ch == '-' || l.ch == '/' {
		if l.ch == '-' && l.peekChar() == '-' {
			// Skip single-line comment (-- comment)
			for l.ch != 0 && l.ch != '\n' {
				l.readChar()
			}
			// Skip the newline character too
			if l.ch == '\n' {
				l.readChar()
			}
			// Check for more whitespace after the comment
			for l.ch == ' ' || l.ch == '\t' || l.ch == '\n' || l.ch == '\r' {
				l.readChar()
			}
		} else if l.ch == '/' && l.peekChar() == '*' {
			// Skip multi-line comment (/* comment */)
			l.readChar() // consume first '/'
			l.readChar() // consume '*'
			for {
				if l.ch == 0 {
					// Reached EOF while in a comment, break to avoid infinite loop
					break
				}
				if l.ch == '*' && l.peekChar() == '/' {
					l.readChar() // consume '*'
					l.readChar() // consume '/'
					break
				}
				l.readChar()
			}
			// Check for more whitespace after the comment
			for l.ch == ' ' || l.ch == '\t' || l.ch == '\n' || l.ch == '\r' {
				l.readChar()
			}
		} else {
			// Not a comment, exit the loop
			break
		}
	}
}

func (l *Lexer) readIdentifier() string {
	position := l.position
	for isLetter(l.ch) || isDigit(l.ch) {
		l.readChar()
	}
	return l.input[position:l.position]
}

func (l *Lexer) readNumber() string {
	position := l.position
	for isDigit(l.ch) {
		l.readChar()
	}
	if l.ch == '.' {
		l.readChar()
		for isDigit(l.ch) {
			l.readChar()
		}
	}
	return l.input[position:l.position]
}

func (l *Lexer) readString() string {
	position := l.position + 1
	for {
		l.readChar()
		if l.ch == '\'' || l.ch == 0 {
			break
		}
	}
	return l.input[position:l.position]
}

func isLetter(ch byte) bool {
	return 'a' <= ch && ch <= 'z' || 'A' <= ch && ch <= 'Z' || ch == '_'
}

func isDigit(ch byte) bool {
	return '0' <= ch && ch <= '9'
}

func newToken(tokenType TokenType, ch byte) Token {
	return Token{Type: tokenType, Literal: string(ch)}
}
