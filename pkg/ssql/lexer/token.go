package lexer

// Position represents the position of a token in the source code
type Position struct {
	Line   int // 1-based line number
	Column int // 1-based column number within the line
	Offset int // 0-based offset from the start of input
}

// Token represents a lexical token with position information
type Token struct {
	Type     TokenType
	Literal  string
	Position Position
}

// String returns a string representation of the token
func (t Token) String() string {
	return t.Literal
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
	PLUS     = "+"
	SLASH    = "/"
	PERCENT  = "%"

	// Delimiters
	COMMA     = ","
	SEMICOLON = ";"
	LPAREN    = "("
	RPAREN    = ")"
	ASTERISK  = "*"
	DOT       = "."
	COLON     = ":"

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

	// New tokens for Phase 0
	NEWLINE  = "NEWLINE"
	SPACE    = "SPACE"
	TAB      = "TAB"
	WHITESPACE = "WHITESPACE"
	COMMENT_SINGLE = "COMMENT_SINGLE"
	COMMENT_MULTI = "COMMENT_MULTI"
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
	if tok, ok := keywords[stringToUpper(ident)]; ok {
		return tok
	}
	return IDENT
}

func stringToUpper(s string) string {
	// Simple implementation of ToUpper without importing strings package
	var result []byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' {
			result = append(result, c-'a'+'A')
		} else {
			result = append(result, c)
		}
	}
	return string(result)
}