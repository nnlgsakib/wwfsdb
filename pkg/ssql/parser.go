package ssql

import (
	"fmt"
	"strconv"
	"github.com/nnlgsakib/wwfsdb/pkg/ssql/ast"
	"github.com/nnlgsakib/wwfsdb/pkg/ssql/lexer"
	"strings"
)

type Parser struct {
	l      *lexer.Lexer
	errors []string

	curToken  lexer.Token
	peekToken lexer.Token

	prefixParseFns map[lexer.TokenType]prefixParseFn
	infixParseFns  map[lexer.TokenType]infixParseFn
}

const (
	_ int = iota
	LOWEST
	EQUALS      // ==
	LESSGREATER // > or <
	SUM         // +
	PRODUCT     // *
	PREFIX      // -X or !X
	CALL        // myFunction(X)
)

var precedences = map[lexer.TokenType]int{
	lexer.ASSIGN:   EQUALS,
	lexer.NE:       EQUALS,
	lexer.LT:       LESSGREATER,
	lexer.GT:       LESSGREATER,
	lexer.AND:      EQUALS,
	lexer.OR:       EQUALS,
	lexer.LIKE:     EQUALS,
	lexer.IN:       EQUALS,
}

type (
	prefixParseFn func() ast.Expression
	infixParseFn  func(ast.Expression) ast.Expression
)

func (p *Parser) registerPrefix(tokenType lexer.TokenType, fn prefixParseFn) {
	p.prefixParseFns[tokenType] = fn
}

func (p *Parser) registerInfix(tokenType lexer.TokenType, fn infixParseFn) {
	p.infixParseFns[tokenType] = fn
}

func NewParser(l *lexer.Lexer) *Parser {
	p := &Parser{
		l:      l,
		errors: []string{},
	}

	p.prefixParseFns = make(map[lexer.TokenType]prefixParseFn)
	p.registerPrefix(lexer.IDENT, p.parseIdentifier)
	p.registerPrefix(lexer.STRING, p.parseStringLiteral)
	p.registerPrefix(lexer.NUMBER, p.parseNumberLiteral)
	p.registerPrefix(lexer.TRUE, p.parseBooleanLiteral)
	p.registerPrefix(lexer.FALSE, p.parseBooleanLiteral)

	p.infixParseFns = make(map[lexer.TokenType]infixParseFn)
	p.registerInfix(lexer.ASSIGN, p.parseInfixExpression)
	p.registerInfix(lexer.NE, p.parseInfixExpression)
	p.registerInfix(lexer.LT, p.parseInfixExpression)
	p.registerInfix(lexer.GT, p.parseInfixExpression)
	p.registerInfix(lexer.AND, p.parseInfixExpression)
	p.registerInfix(lexer.OR, p.parseInfixExpression)
	p.registerInfix(lexer.LIKE, p.parseLikeExpression)
	p.registerInfix(lexer.IN, p.parseInExpression)

	// Read two tokens, so curToken and peekToken are both set
	p.nextToken()
	p.nextToken()

	return p
}

func (p *Parser) parseIdentifier() ast.Expression {
	return &ast.Identifier{Name: p.curToken.Literal}
}

func (p *Parser) parseStringLiteral() ast.Expression {
	return &ast.Literal{Value: p.curToken.Literal}
}

func (p *Parser) parseNumberLiteral() ast.Expression {
	lit := &ast.NumberLiteral{}
	val, err := strconv.ParseFloat(p.curToken.Literal, 64)
	if err != nil {
		msg := fmt.Sprintf("could not parse %q as float", p.curToken.Literal)
		p.errors = append(p.errors, msg)
		return nil
	}
	lit.Value = val
	return lit
}

func (p *Parser) parseBooleanLiteral() ast.Expression {
	return &ast.BooleanLiteral{Value: p.curToken.Type == lexer.TRUE}
}

func (p *Parser) parseExpression(precedence int) ast.Expression {
	prefix := p.prefixParseFns[p.curToken.Type]
	if prefix == nil {
		p.noPrefixParseFnError(p.curToken.Type)
		return nil
	}
	leftExp := prefix()

	for !p.peekTokenIs(lexer.SEMICOLON) && precedence < p.peekPrecedence() {
		infix := p.infixParseFns[p.peekToken.Type]
		if infix == nil {
			return leftExp
		}

		p.nextToken()

		leftExp = infix(leftExp)
	}

	return leftExp
}

func (p *Parser) parseInfixExpression(left ast.Expression) ast.Expression {
	if p.curToken.Type == lexer.AND || p.curToken.Type == lexer.OR {
		expression := &ast.BinaryExpr{
			Left:     left,
			Operator: p.curToken.Literal,
		}
		precedence := p.curPrecedence()
		p.nextToken()
		expression.Right = p.parseExpression(precedence)
		return expression
	}

	expression := &ast.ComparisonExpr{
		Left:     left,
		Operator: p.curToken.Literal,
	}

	precedence := p.curPrecedence()
	p.nextToken()
	expression.Right = p.parseExpression(precedence)

	return expression
}

func (p *Parser) parseLikeExpression(left ast.Expression) ast.Expression {
	expression := &ast.LikeExpr{
		Left: left,
	}
	precedence := p.curPrecedence()
	p.nextToken()
	expression.Pattern = p.parseExpression(precedence)
	return expression
}

func (p *Parser) parseInExpression(left ast.Expression) ast.Expression {
	expression := &ast.InExpr{Left: left}
	if !p.expectPeek(lexer.LPAREN) {
		return nil
	}
	expression.Values = p.parseInExpressionList()
	return expression
}

func (p *Parser) parseInExpressionList() []ast.Expression {
	list := []ast.Expression{}
	if p.peekTokenIs(lexer.RPAREN) {
		p.nextToken()
		return list
	}
	p.nextToken()
	list = append(list, p.parseExpression(LOWEST))
	for p.peekTokenIs(lexer.COMMA) {
		p.nextToken()
		p.nextToken()
		list = append(list, p.parseExpression(LOWEST))
	}
	if !p.expectPeek(lexer.RPAREN) {
		return nil
	}
	return list
}

func (p *Parser) peekPrecedence() int {
	if p, ok := precedences[p.peekToken.Type]; ok {
		return p
	}
	return LOWEST
}

func (p *Parser) curPrecedence() int {
	if p, ok := precedences[p.curToken.Type]; ok {
		return p
	}
	return LOWEST
}

func (p *Parser) noPrefixParseFnError(t lexer.TokenType) {
	msg := fmt.Sprintf("no prefix parse function for %s found", t)
	p.errors = append(p.errors, msg)
}

func (p *Parser) Errors() []string {
	return p.errors
}

func (p *Parser) nextToken() {
	p.curToken = p.peekToken
	p.peekToken = p.l.NextToken()
}

func (p *Parser) ParseStatement() ast.Statement {
	switch p.curToken.Type {
	case lexer.CREATE:
		return p.parseCreateStatement()
	case lexer.DROP:
		return p.parseDropStatement()
	case lexer.SELECT:
		return p.parseSelectStatement()
	case lexer.INSERT:
		return p.parseInsertStatement()
	case lexer.UPDATE:
		return p.parseUpdateStatement()
	case lexer.DELETE:
		return p.parseDeleteStatement()
	default:
		return nil
	}
}

func (p *Parser) parseCreateStatement() ast.Statement {
	p.nextToken() // consume CREATE

	switch p.curToken.Type {
	case lexer.DATABASE:
		return p.parseCreateDatabaseStatement()
	case lexer.TABLE:
		return p.parseCreateTableStatement()
	case lexer.INDEX:
		return p.parseCreateIndexStatement()
	default:
		p.errors = append(p.errors, fmt.Sprintf("expected DATABASE, TABLE, or INDEX after CREATE, got %s", p.curToken.Literal))
		return nil
	}
}

func (p *Parser) parseCreateDatabaseStatement() *ast.CreateDatabaseStmt {
	stmt := &ast.CreateDatabaseStmt{}

	if !p.expectPeek(lexer.IDENT) {
		p.errors = append(p.errors, fmt.Sprintf("expected identifier for database name, got %s", p.curToken.Literal))
		return nil
	}

	stmt.Name = p.curToken.Literal

	// Optional semicolon
	if p.peekTokenIs(lexer.SEMICOLON) {
		p.nextToken()
	}

	return stmt
}

func (p *Parser) parseCreateTableStatement() *ast.CreateTableStmt {
	stmt := &ast.CreateTableStmt{}

	if !p.expectPeek(lexer.IDENT) {
		return nil
	}
	stmt.Name = p.curToken.Literal

	if !p.expectPeek(lexer.LPAREN) {
		return nil
	}

	stmt.Schema.Columns = p.parseColumnDefinitions()

	if !p.expectPeek(lexer.RPAREN) {
		return nil
	}

	// Optional semicolon
	if p.peekTokenIs(lexer.SEMICOLON) {
		p.nextToken()
	}

	return stmt
}

func (p *Parser) parseCreateIndexStatement() *ast.CreateIndexStmt {
	stmt := &ast.CreateIndexStmt{}

	if !p.expectPeek(lexer.ON) {
		return nil
	}

	if !p.expectPeek(lexer.IDENT) {
		return nil
	}
	stmt.Table = p.curToken.Literal

	if !p.expectPeek(lexer.LPAREN) {
		return nil
	}

	if !p.expectPeek(lexer.IDENT) {
		return nil
	}
	stmt.Column = p.curToken.Literal

	if !p.expectPeek(lexer.RPAREN) {
		return nil
	}

	// Optional semicolon
	if p.peekTokenIs(lexer.SEMICOLON) {
		p.nextToken()
	}

	return stmt
}

func (p *Parser) parseColumnDefinitions() []ast.Column {
	columns := []ast.Column{}

	if p.peekTokenIs(lexer.RPAREN) {
		return columns
	}

	p.nextToken()

	col := ast.Column{}
	if p.curToken.Type != lexer.IDENT {
		p.errors = append(p.errors, fmt.Sprintf("expected identifier for column name, got %s", p.curToken.Literal))
		return nil
	}
	col.Name = p.curToken.Literal

	if !p.expectPeek(lexer.IDENT) { // for type
		return nil
	}
	col.Type = p.curToken.Literal

	if p.peekTokenIs(lexer.LPAREN) {
		p.nextToken() // consume '('
		col.Type += "("
		if p.peekTokenIs(lexer.IDENT) || p.peekTokenIs(lexer.NUMBER) {
			p.nextToken() // consume length
			col.Type += p.curToken.Literal
		}
		if !p.expectPeek(lexer.RPAREN) {
			return nil
		}
		col.Type += ")"
	}
	columns = append(columns, col)

	for p.peekTokenIs(lexer.COMMA) {
		p.nextToken()
		p.nextToken()

		col := ast.Column{}
		if p.curToken.Type != lexer.IDENT {
			p.errors = append(p.errors, fmt.Sprintf("expected identifier for column name, got %s", p.curToken.Literal))
			return nil
		}
		col.Name = p.curToken.Literal

		if !p.expectPeek(lexer.IDENT) { // for type
			return nil
		}
		col.Type = p.curToken.Literal

		if p.peekTokenIs(lexer.LPAREN) {
			p.nextToken() // consume '('
			col.Type += "("
			if p.peekTokenIs(lexer.IDENT) || p.peekTokenIs(lexer.NUMBER) {
				p.nextToken() // consume length
				col.Type += p.curToken.Literal
			}
			if !p.expectPeek(lexer.RPAREN) {
				return nil
			}
			col.Type += ")"
		}
		columns = append(columns, col)
	}

	return columns
}

func (p *Parser) parseDropStatement() *ast.DropTableStmt {
	stmt := &ast.DropTableStmt{}
	if !p.expectPeek(lexer.TABLE) {
		return nil
	}
	if !p.expectPeek(lexer.IDENT) {
		return nil
	}
	stmt.Name = p.curToken.Literal
	if p.peekTokenIs(lexer.SEMICOLON) {
		p.nextToken()
	}
	return stmt
}

func (p *Parser) parseSelectStatement() *ast.SelectStmt {
	stmt := &ast.SelectStmt{}

	if p.peekTokenIs(lexer.ASTERISK) {
		p.nextToken()
		stmt.Columns = []string{"*"} // Use a special value to indicate all columns
	} else {
		stmt.Columns = p.parseIdentifierList()
	}

	if !p.expectPeek(lexer.FROM) {
		return nil
	}
	if !p.expectPeek(lexer.IDENT) {
		return nil	}
	stmt.Table = p.curToken.Literal

	if p.peekTokenIs(lexer.WHERE) {
		p.nextToken()
		stmt.Where = p.parseWhereClause()
	}

	if p.peekTokenIs(lexer.SEMICOLON) {
		p.nextToken()
	}

	return stmt
}

func (p *Parser) parseIdentifierList() []string {
	list := []string{}

	if p.peekTokenIs(lexer.FROM) {
		p.errors = append(p.errors, "expected identifier or '*' after SELECT")
		return nil
	}

	p.nextToken()
	if p.curToken.Type != lexer.IDENT {
		p.errors = append(p.errors, "expected identifier in column list")
		return nil
	}
	list = append(list, p.curToken.Literal)

	for p.peekTokenIs(lexer.COMMA) {
		p.nextToken()
		p.nextToken()
		if p.curToken.Type != lexer.IDENT {
			p.errors = append(p.errors, "expected identifier in column list")
			return nil
		}
		list = append(list, p.curToken.Literal)
	}

	return list
}

func (p *Parser) parseWhereClause() ast.Expression {
	p.nextToken() // consume WHERE
	return p.parseExpression(LOWEST)
}

func (p *Parser) parseInsertStatement() *ast.InsertStmt {
	stmt := &ast.InsertStmt{}
	if !p.expectPeek(lexer.INTO) {
		return nil
	}
	if !p.expectPeek(lexer.IDENT) {
		return nil
	}
	stmt.Table = p.curToken.Literal

	if !p.expectPeek(lexer.VALUES) {
		return nil
	}
	if !p.expectPeek(lexer.LPAREN) {
		return nil
	}

	stmt.Values = p.parseExpressionList(lexer.RPAREN)

	// The old parser didn't require a closing paren, but a proper parser should.
	// Let's assume it's required.
	if !p.curTokenIs(lexer.RPAREN) {
		if !p.expectPeek(lexer.RPAREN) {
			return nil
		}
	}

	if p.peekTokenIs(lexer.SEMICOLON) {
		p.nextToken()
	}

	return stmt
}

func (p *Parser) parseExpressionList(end lexer.TokenType) []string {
	list := []string{}

	if p.peekTokenIs(end) {
		p.nextToken()
		return list
	}

	p.nextToken()
	if p.curToken.Type != lexer.STRING {
		p.errors = append(p.errors, "expected string literal in values list")
		return nil
	}
	list = append(list, p.curToken.Literal)

	for p.peekTokenIs(lexer.COMMA) {
		p.nextToken()
		p.nextToken()
		if p.curToken.Type != lexer.STRING {
			p.errors = append(p.errors, "expected string literal in values list")
			return nil
		}
		list = append(list, p.curToken.Literal)
	}

	return list
}

func (p *Parser) parseUpdateStatement() *ast.UpdateStmt {
	stmt := &ast.UpdateStmt{}
	if !p.expectPeek(lexer.IDENT) {
		return nil
	}
	stmt.Table = p.curToken.Literal

	if !p.expectPeek(lexer.SET) {
		return nil
	}
	p.nextToken() // consume SET

	setClause := ast.UpdateClause{}
	if p.curToken.Type != lexer.IDENT {
		p.errors = append(p.errors, "expected column name in SET clause")
		return nil
	}
	setClause.Column = p.curToken.Literal

	if !p.expectPeek(lexer.ASSIGN) {
		return nil
	}
	if !p.expectPeek(lexer.STRING) {
		return nil
	}
	setClause.Value = p.curToken.Literal
	stmt.Set = setClause

	if !p.expectPeek(lexer.WHERE) {
		return nil
	}
	stmt.Where = p.parseWhereClause()

	if p.peekTokenIs(lexer.SEMICOLON) {
		p.nextToken()
	}

	return stmt
}

func (p *Parser) parseDeleteStatement() *ast.DeleteStmt {
	stmt := &ast.DeleteStmt{}
	if !p.expectPeek(lexer.FROM) {
		return nil
	}
	if !p.expectPeek(lexer.IDENT) {
		return nil
	}
	stmt.Table = p.curToken.Literal

	if !p.expectPeek(lexer.WHERE) {
		return nil
	}
	stmt.Where = p.parseWhereClause()

	if p.peekTokenIs(lexer.SEMICOLON) {
		p.nextToken()
	}

	return stmt
}

func (p *Parser) curTokenIs(t lexer.TokenType) bool {
	return p.curToken.Type == t
}

func (p *Parser) peekTokenIs(t lexer.TokenType) bool {
	return p.peekToken.Type == t
}

func (p *Parser) expectPeek(t lexer.TokenType) bool {
	if p.peekTokenIs(t) {
		p.nextToken()
		return true
	} else {
		p.peekError(t)
		return false
	}
}

func (p *Parser) peekError(t lexer.TokenType) {
	msg := fmt.Sprintf("expected next token to be %s, got %s instead",
		t, p.peekToken.Type)
	p.errors = append(p.errors, msg)
}

func (p *Parser) ParseProgram() []ast.Statement {
	var statements []ast.Statement

	for !p.curTokenIs(lexer.EOF) {
		stmt := p.ParseStatement()
		if stmt != nil {
			statements = append(statements, stmt)
		}
		p.nextToken()
	}
	return statements
}

// parse is the main entry point for parsing a SQL string.
func parse(sql string) (ast.Statement, error) {
	// The old regex parser was very lenient with missing semicolons.
	// To maintain compatibility for single statements, we can append one if it's missing.
	if !strings.HasSuffix(strings.TrimSpace(sql), ";") {
		sql += ";"
	}
	l := lexer.New(sql)
	p := NewParser(l)
	stmt := p.ParseStatement()
	if len(p.errors) > 0 {
		return nil, fmt.Errorf("parser errors: %v", p.errors)
	}
	return stmt, nil
}

// parseMultiple is for parsing a script with multiple statements.
func parseMultiple(sql string) ([]ast.Statement, error) {
	l := lexer.New(sql)
	p := NewParser(l)
	stmts := p.ParseProgram()
	if len(p.errors) > 0 {
		return nil, fmt.Errorf("parser errors: %v", p.errors)
	}
	return stmts, nil
}
