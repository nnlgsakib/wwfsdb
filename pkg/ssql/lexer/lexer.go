package lexer

// Lexer is the interface for SQL tokenizers.
type Lexer struct {
	input          string
	position       int  // current position in input (points to current char)
	readPosition   int  // current reading position in input (after current char)
	ch             byte // current char under examination
	line           int  // current line number (1-based)
	column         int  // current column number (1-based)
	keepComments   bool // whether to preserve comments as tokens instead of skipping them
	keepWhitespace bool // whether to preserve whitespace as tokens instead of skipping them
}

func New(input string) *Lexer {
	l := &Lexer{
		input:          input,
		line:           1,
		column:         0, // Start at column 0, will be incremented to 1 when first character is read
		keepComments:   false,
		keepWhitespace: false,
	}
	l.readChar()
	return l
}

func NewWithFormatting(input string, keepComments, keepWhitespace bool) *Lexer {
	l := &Lexer{
		input:          input,
		line:           1,
		column:         0, // Start at column 0, will be incremented to 1 when first character is read
		keepComments:   keepComments,
		keepWhitespace: keepWhitespace,
	}
	l.readChar()
	return l
}

func (l *Lexer) readChar() {
	// Update position before reading character
	l.position = l.readPosition

	if l.readPosition >= len(l.input) {
		l.ch = 0
	} else {
		l.ch = l.input[l.readPosition]
	}

	// Update position tracking based on the character we're reading
	if l.ch == '\n' {
		l.line++
		l.column = 0 // Will be incremented to 1 after this
	} else {
		l.column++ // Always increment column after reading a character
	}

	l.readPosition++
}

func (l *Lexer) NextToken() Token {
	var tok Token

	// Store the position where a potential token starts
	startLine := l.line
	startColumn := l.column
	startPosition := l.position

	if !l.keepWhitespace {
		l.skipWhitespace()
		// Update the start position after skipping whitespace
		startLine = l.line
		startColumn = l.column
		startPosition = l.position
	} else {
		// If keeping whitespace, process whitespace tokens first
		if l.ch == ' ' || l.ch == '\t' {
			tok = Token{
				Type:    SPACE,
				Literal: string(l.ch),
				Position: Position{
					Line:   startLine,
					Column: startColumn,
					Offset: startPosition,
				},
			}
			l.readChar()
			return tok
		} else if l.ch == '\n' || l.ch == '\r' {
			// Handle newlines
			newlineChar := l.ch
			if l.ch == '\r' && l.peekChar() == '\n' {
				// Handle Windows-style CRLF
				l.readChar() // consume \r
			}
			tok = Token{
				Type:    NEWLINE,
				Literal: string(newlineChar),
				Position: Position{
					Line:   startLine,
					Column: startColumn,
					Offset: startPosition,
				},
			}
			l.readChar()
			return tok
		}
	}

	// After handling whitespace, check for comments and process appropriately
	if l.ch == '-' && l.peekChar() == '-' {
		if l.keepComments {
			return l.readSingleLineComment(startLine, startColumn, startPosition)
		} else {
			l.skipComments() // Skip the comment
			// Update position and continue to get next real token
			startLine = l.line
			startColumn = l.column
			startPosition = l.position
		}
	} else if l.ch == '/' && l.peekChar() == '*' {
		if l.keepComments {
			return l.readMultiLineComment(startLine, startColumn, startPosition)
		} else {
			l.skipComments() // Skip the comment
			// Update position and continue to get next real token
			startLine = l.line
			startColumn = l.column
			startPosition = l.position
		}
	}

	switch l.ch {
	case '=':
		tok = newTokenWithPosition(ASSIGN, l.ch, startLine, startColumn, startPosition)
	case '>':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok = Token{
				Type:    GTE,
				Literal: string(ch) + string(l.ch),
				Position: Position{
					Line:   startLine,
					Column: startColumn,
					Offset: startPosition,
				},
			}
		} else {
			tok = newTokenWithPosition(GT, l.ch, startLine, startColumn, startPosition)
		}
	case '<':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok = Token{
				Type:    LTE,
				Literal: string(ch) + string(l.ch),
				Position: Position{
					Line:   startLine,
					Column: startColumn,
					Offset: startPosition,
				},
			}
		} else if l.peekChar() == '>' {
			ch := l.ch
			l.readChar()
			tok = Token{
				Type:    NE,
				Literal: string(ch) + string(l.ch),
				Position: Position{
					Line:   startLine,
					Column: startColumn,
					Offset: startPosition,
				},
			}
		} else {
			tok = newTokenWithPosition(LT, l.ch, startLine, startColumn, startPosition)
		}
	case '!':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok = Token{
				Type:    NE,
				Literal: string(ch) + string(l.ch),
				Position: Position{
					Line:   startLine,
					Column: startColumn,
					Offset: startPosition,
				},
			}
		} else {
			tok = newTokenWithPosition(ILLEGAL, l.ch, startLine, startColumn, startPosition)
		}
	case ';':
		tok = newTokenWithPosition(SEMICOLON, l.ch, startLine, startColumn, startPosition)
	case '(':
		tok = newTokenWithPosition(LPAREN, l.ch, startLine, startColumn, startPosition)
	case ')':
		tok = newTokenWithPosition(RPAREN, l.ch, startLine, startColumn, startPosition)
	case ',':
		tok = newTokenWithPosition(COMMA, l.ch, startLine, startColumn, startPosition)
	case '*':
		tok = newTokenWithPosition(ASTERISK, l.ch, startLine, startColumn, startPosition)
	case '.':
		tok = newTokenWithPosition(DOT, l.ch, startLine, startColumn, startPosition)
	case '-':
		tok = newTokenWithPosition(MINUS, l.ch, startLine, startColumn, startPosition)
	case '+':
		tok = newTokenWithPosition(PLUS, l.ch, startLine, startColumn, startPosition)
	case '/':
		tok = newTokenWithPosition(SLASH, l.ch, startLine, startColumn, startPosition)
	case '%':
		tok = newTokenWithPosition(PERCENT, l.ch, startLine, startColumn, startPosition)
	case '\'':
		tok.Type = STRING
		tok.Literal = l.readString()
		tok.Position = Position{
			Line:   startLine,
			Column: startColumn,
			Offset: startPosition,
		}
	case '"':
		tok.Type = IDENT
		tok.Literal = l.readDoubleQuotedString()
		tok.Position = Position{
			Line:   startLine,
			Column: startColumn,
			Offset: startPosition,
		}
	case 0:
		tok.Literal = ""
		tok.Type = EOF
		tok.Position = Position{
			Line:   startLine,
			Column: startColumn,
			Offset: startPosition,
		}
	default:
		if isLetter(l.ch) {
			// Check if this is a raw string literal (r"..." or r'...')
			identifier := l.readIdentifier()
			// Check if it's a single 'r' followed by a quote
			if identifier == "r" && (l.ch == '"' || l.ch == '\'') {
				tok.Type = RAW_STRING
				tok.Literal = "r" + l.readRawString()
			} else {
				tok.Literal = identifier
				tok.Type = LookupIdent(tok.Literal)
			}
			tok.Position = Position{
				Line:   startLine,
				Column: startColumn,
				Offset: startPosition,
			}
			return tok
		} else if isDigit(l.ch) {
			// Check for hex or binary literals starting with '0x', '0X', '0b', or '0B'
			if l.ch == '0' && (l.peekChar() == 'x' || l.peekChar() == 'X') {
				tok.Type = HEX_LITERAL
				tok.Literal = l.readHexLiteral()
				tok.Position = Position{
					Line:   startLine,
					Column: startColumn,
					Offset: startPosition,
				}
				return tok
			} else if l.ch == '0' && (l.peekChar() == 'b' || l.peekChar() == 'B') {
				tok.Type = BINARY_LITERAL
				tok.Literal = l.readBinaryLiteral()
				tok.Position = Position{
					Line:   startLine,
					Column: startColumn,
					Offset: startPosition,
				}
				return tok
			} else {
				tok.Type = NUMBER
				tok.Literal = l.readNumber()
				tok.Position = Position{
					Line:   startLine,
					Column: startColumn,
					Offset: startPosition,
				}
				return tok
			}
		} else {
			tok = newTokenWithPosition(ILLEGAL, l.ch, startLine, startColumn, startPosition)
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
}

func (l *Lexer) skipComments() {
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
	}
}

func (l *Lexer) readSingleLineComment(startLine, startColumn, startPosition int) Token {
	// We start at the first '-' of '--'
	// Process the second dash and move past it
	l.readChar()  // Move past first dash, now at second dash
	l.readChar()  // Move past second dash
	
	// Check if there's a space after '--' and consume it
	if l.ch == ' ' {
		l.readChar()  // Now positioned after the space
	}

	// At this point, we're at the beginning of the actual comment content
	commentStart := l.position
	
	// Read characters for the comment content, but don't consume a newline if we encounter one
	for {
		if l.ch == 0 || l.ch == '\n' || l.ch == '\r' {
			// We've reached the end of the comment content without consuming the line ending
			break
		}
		// Save current position, then read next character
		prevPos := l.position
		l.readChar()
		// Check if we've just read a line ending character - if so, we need to back up
		if l.ch == '\n' || l.ch == '\r' || l.ch == 0 {
			// Back to the previous position that contains the last actual comment character
			// In our implementation, we can't really "back up", so we need to work differently
			// l.position currently points to the last character we read (which is the line ending)
			// So the content is commentStart to the previous position (prevPos)
			commentContent := l.input[commentStart:prevPos+1]
			return Token{
				Type:    COMMENT_SINGLE,
				Literal: "-- " + commentContent,
				Position: Position{
					Line:   startLine,
					Column: startColumn,
					Offset: startPosition,
				},
			}
		}
	}
	
	// This happens if we exit the loop normally (e.g., at EOF)
	// In this case, l.position points to the position of the last char we read
	commentContent := l.input[commentStart:l.position+1]
	return Token{
		Type:    COMMENT_SINGLE,
		Literal: "-- " + commentContent,
		Position: Position{
			Line:   startLine,
			Column: startColumn,
			Offset: startPosition,
		},
	}
}

func (l *Lexer) readMultiLineComment(startLine, startColumn, startPosition int) Token {
	l.readChar() // consume '/'
	l.readChar() // consume '*'

	commentStart := l.position

	for {
		if l.ch == 0 {
			// Reached EOF while in a comment, return what we have
			break
		}
		if l.ch == '*' && l.peekChar() == '/' {
			l.readChar() // consume '*'
			l.readChar() // consume '/'
			break
		}
		l.readChar()
	}

	// The original code used l.position-2 to exclude the '*/' from content
	// This is tricky to get right - we want content between /* and */
	// The l.position at this point is the position of the '/' that was consumed
	// So content from commentStart to position of consumed '/' minus 2 chars ('*/')
	commentText := l.input[commentStart : l.position-2]

	return Token{
		Type:    COMMENT_MULTI,
		Literal: "/* " + commentText + " */",
		Position: Position{
			Line:   startLine,
			Column: startColumn,
			Offset: startPosition,
		},
	}
}

func (l *Lexer) readIdentifier() string {
	position := l.position
	for isLetter(l.ch) || isDigit(l.ch) {
		l.readChar()
	}
	return l.input[position:l.position]
}

// peekIdentifier looks ahead to determine what identifier we're dealing with
// without consuming characters
func (l *Lexer) peekIdentifier() string {
	position := l.position
	ch := l.ch
	// Read the identifier without advancing the lexer
	for isLetter(ch) || isDigit(ch) {
		position++
		if position >= len(l.input) {
			break
		}
		ch = l.input[position]
	}
	return l.input[l.position:position]
}

func (l *Lexer) readNumber() string {
	position := l.position
	
	// Read initial digits
	for isDigit(l.ch) {
		l.readChar()
	}
	
	// Handle decimal point and fractional part
	if l.ch == '.' {
		l.readChar()
		for isDigit(l.ch) {
			l.readChar()
		}
	}
	
	// Handle scientific notation (e.g., 1.23e-4, 1E+5)
	if l.ch == 'e' || l.ch == 'E' {
		l.readChar() // consume 'e' or 'E'
		
		// Handle optional sign (+ or -)
		if l.ch == '+' || l.ch == '-' {
			l.readChar()
		}
		
		// Read exponent digits
		if isDigit(l.ch) {
			for isDigit(l.ch) {
				l.readChar()
			}
		} else {
			// If there's no digit after e/E(+/-), we have an invalid scientific notation
			// In this case, we should not include the e/E part in the number
			// For now, we'll just let it be and the parser can handle invalid formats
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

func (l *Lexer) readDoubleQuotedString() string {
	position := l.position + 1
	for {
		l.readChar()
		if l.ch == '"' || l.ch == 0 {
			break
		}
	}
	return l.input[position:l.position]
}

func (l *Lexer) readRawString() string {
	// We're at the quote character (either " or ') after 'r'
	quote := l.ch
	position := l.position + 1
	for {
		l.readChar()
		if l.ch == quote || l.ch == 0 {
			break
		}
	}
	return l.input[position:l.position]
}

func (l *Lexer) readHexLiteral() string {
	// We're at '0', read 'x' or 'X'
	position := l.position  // starting position of '0'
	l.readChar() // consume '0', now l.ch is 'x' or 'X'
	l.readChar() // consume 'x' or 'X'
	
	// Read hexadecimal digits
	for isHexDigit(l.ch) {
		l.readChar()
	}
	
	return l.input[position:l.position]
}

func (l *Lexer) readBinaryLiteral() string {
	// We're at '0', read 'b' or 'B'
	position := l.position  // starting position of '0'
	l.readChar() // consume '0', now l.ch is 'b' or 'B'
	l.readChar() // consume 'b' or 'B'
	
	// Read binary digits (0 or 1)
	for isBinaryDigit(l.ch) {
		l.readChar()
	}
	
	return l.input[position:l.position]
}

func isHexDigit(ch byte) bool {
	return isDigit(ch) || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')
}

func isBinaryDigit(ch byte) bool {
	return ch == '0' || ch == '1'
}

func isLetter(ch byte) bool {
	return 'a' <= ch && ch <= 'z' || 'A' <= ch && ch <= 'Z' || ch == '_' || ch >= 0x80
}

func isDigit(ch byte) bool {
	return '0' <= ch && ch <= '9'
}

func newTokenWithPosition(tokenType TokenType, ch byte, line, col, offset int) Token {
	return Token{
		Type:    tokenType,
		Literal: string(ch),
		Position: Position{
			Line:   line,
			Column: col,
			Offset: offset,
		},
	}
}