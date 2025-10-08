package ssql

import (
	"fmt"
	"github.com/nnlgsakib/wwfsdb/pkg/ssql/ast"
	"github.com/nnlgsakib/wwfsdb/pkg/ssql/lexer"
	"strings"
)

type Parser struct {
	l      *lexer.Lexer
	errors []string

	curToken  lexer.Token
	peekToken lexer.Token
}

func NewParser(l *lexer.Lexer) *Parser {
	p := &Parser{
		l:      l,
		errors: []string{},
	}

	// Read two tokens, so curToken and peekToken are both set
	p.nextToken()
	p.nextToken()

	return p
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
	if p.peekTokenIs(lexer.DATABASE) {
		p.nextToken() // consume CREATE
		return p.parseCreateDatabaseStatement()
	}
	if p.peekTokenIs(lexer.TABLE) {
		p.nextToken() // consume CREATE
		return p.parseCreateTableStatement()
	}
	p.errors = append(p.errors, "expected DATABASE or TABLE after CREATE")
	return nil
}

func (p *Parser) parseCreateDatabaseStatement() *ast.CreateDatabaseStmt {
	p.nextToken() // consume DATABASE
	stmt := &ast.CreateDatabaseStmt{}

	if !p.curTokenIs(lexer.IDENT) {
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
	p.nextToken() // consume TABLE
	stmt := &ast.CreateTableStmt{}

	if !p.curTokenIs(lexer.IDENT) {
		p.errors = append(p.errors, fmt.Sprintf("expected identifier for table name, got %s", p.curToken.Literal))
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
		if p.peekTokenIs(lexer.IDENT) {
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
			if p.peekTokenIs(lexer.IDENT) {
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
	if !p.expectPeek(lexer.ASTERISK) {
		return nil
	}
	if !p.expectPeek(lexer.FROM) {
		return nil
	}
	if !p.expectPeek(lexer.IDENT) {
		return nil
	}
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

func (p *Parser) parseWhereClause() *ast.WhereClause {
	p.nextToken() // consume WHERE
	where := &ast.WhereClause{}

	if p.curToken.Type != lexer.IDENT {
		p.errors = append(p.errors, "expected column name in WHERE clause")
		return nil
	}
	where.Column = p.curToken.Literal

	if !p.expectPeek(lexer.ASSIGN) {
		return nil
	}

	if !p.expectPeek(lexer.STRING) {
		return nil
	}
	where.Value = p.curToken.Literal

	return where
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
	stmt.Where = *p.parseWhereClause()

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
	stmt.Where = *p.parseWhereClause()

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
