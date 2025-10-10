package ssql

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/nnlgsakib/wwfsdb/pkg/ssql/ast"
	"github.com/nnlgsakib/wwfsdb/pkg/ssql/lexer"
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

	lexer.JOIN:     CALL, // Not used as infix, but for precedence context

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
	p.registerPrefix(lexer.MINUS, p.parsePrefixExpression)

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
	ident := &ast.Identifier{Name: p.curToken.Literal}
	if p.peekTokenIs(lexer.DOT) {
		p.nextToken() // consume '.'
		if p.expectPeek(lexer.IDENT) {
			ident.TableQualifier = ident.Name
			ident.Name = p.curToken.Literal
		}
	}
	return ident
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

func (p *Parser) parsePrefixExpression() ast.Expression {
	expression := &ast.PrefixExpression{
		Operator: p.curToken.Literal,
	}
	p.nextToken()
	expression.Right = p.parseExpression(PREFIX)
	return expression
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
	case lexer.ALTER:
		return p.parseAlterTableStatement()
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
	case lexer.BEGIN:
		return p.parseBeginStatement()
	case lexer.COMMIT:
		return p.parseCommitStatement()
	case lexer.ROLLBACK:
		return p.parseRollbackStatement()
	default:
		return nil
	}
}

func (p *Parser) parseAlterTableStatement() *ast.AlterTableStmt {
	stmt := &ast.AlterTableStmt{}

	if !p.expectPeek(lexer.TABLE) {
		return nil
	}

	if !p.expectPeek(lexer.IDENT) {
		return nil
	}
	stmt.Table = p.curToken.Literal

	switch p.peekToken.Type {
	case lexer.ADD:
		p.nextToken() // consume table name
		stmt.Action = p.parseAddColumn()
	case lexer.DROP:
		p.nextToken()
		stmt.Action = p.parseDropColumn()
	case lexer.RENAME:
		p.nextToken()
		stmt.Action = p.parseRenameColumn()
	default:
		msg := fmt.Sprintf("expected next token to be ADD, DROP, or RENAME, got %s instead", p.peekToken.Type)
		p.errors = append(p.errors, msg)
		return nil
	}

	if p.peekTokenIs(lexer.SEMICOLON) {
		p.nextToken()
	}

	return stmt
}

func (p *Parser) parseAddColumn() ast.AlterTableAction {
	if !p.expectPeek(lexer.COLUMN) {
		return nil
	}

	addColumn := &ast.AddColumnClause{}

	if !p.expectPeek(lexer.IDENT) {
		return nil
	}
	addColumn.Column.Name = p.curToken.Literal

	if !p.expectPeek(lexer.IDENT) { // Type
		return nil
	}
	addColumn.Column.Type = p.curToken.Literal

	return addColumn
}

func (p *Parser) parseDropColumn() ast.AlterTableAction {
	if !p.expectPeek(lexer.COLUMN) {
		return nil
	}

	dropColumn := &ast.DropColumnClause{}

	if !p.expectPeek(lexer.IDENT) {
		return nil
	}
	dropColumn.ColumnName = p.curToken.Literal

	return dropColumn
}

func (p *Parser) parseRenameColumn() ast.AlterTableAction {
	if !p.expectPeek(lexer.COLUMN) {
		return nil
	}

	renameColumn := &ast.RenameColumnClause{}

	if !p.expectPeek(lexer.IDENT) {
		return nil
	}
	renameColumn.OldName = p.curToken.Literal

	if !p.expectPeek(lexer.TO) {
		return nil
	}

	if !p.expectPeek(lexer.IDENT) {
		return nil
	}
	renameColumn.NewName = p.curToken.Literal

	return renameColumn
}

func (p *Parser) parseBeginStatement() *ast.BeginStmt {
	stmt := &ast.BeginStmt{}
	if p.peekTokenIs(lexer.SEMICOLON) {
		p.nextToken()
	}
	return stmt
}

func (p *Parser) parseCommitStatement() *ast.CommitStmt {
	stmt := &ast.CommitStmt{}
	if p.peekTokenIs(lexer.SEMICOLON) {
		p.nextToken()
	}
	return stmt
}

func (p *Parser) parseRollbackStatement() *ast.RollbackStmt {
	stmt := &ast.RollbackStmt{}
	if p.peekTokenIs(lexer.SEMICOLON) {
		p.nextToken()
	}
	return stmt
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

	stmt.Columns = p.parseSelectList()

	if !p.expectPeek(lexer.FROM) {
		return nil
	}

	stmt.From = p.parseFromClause()

	if p.peekTokenIs(lexer.WHERE) {
		p.nextToken()
		stmt.Where = p.parseWhereClause()
	}

	if p.peekTokenIs(lexer.SEMICOLON) {
		p.nextToken()
	}

	return stmt
}

func (p *Parser) parseFromClause() ast.FromClause {
	if !p.expectPeek(lexer.IDENT) {
		return nil
	}

	left := ast.FromClause(&ast.TableIdentifier{Name: p.curToken.Literal})

	for p.peekTokenIs(lexer.JOIN) || p.peekTokenIs(lexer.INNER) || p.peekTokenIs(lexer.LEFT) {
		joinType := "INNER"
		p.nextToken() // consume JOIN, INNER, or LEFT

		if p.curToken.Type == lexer.LEFT {
			joinType = "LEFT"
			if !p.expectPeek(lexer.JOIN) {
				return nil
			}
		} else if p.curToken.Type == lexer.INNER {
			if !p.expectPeek(lexer.JOIN) {
				return nil
			}
		}

		jc := &ast.JoinClause{
			Type: joinType,
			Left: left,
		}

		if !p.expectPeek(lexer.IDENT) {
			return nil
		}
		jc.Right = &ast.TableIdentifier{Name: p.curToken.Literal}

		if !p.expectPeek(lexer.ON) {
			return nil
		}
		p.nextToken() // consume ON
		jc.On = p.parseExpression(LOWEST)

		left = jc
	}

	return left
}

func (p *Parser) parseSelectList() []ast.SelectColumn {
	list := []ast.SelectColumn{}

	if p.peekTokenIs(lexer.ASTERISK) {
		p.nextToken()
		list = append(list, ast.SelectColumn{Name: "*"})
		return list
	}

	p.nextToken()

	// Helper to parse a single select column (e.g., id, users.id, users.*)
	parseCol := func() ast.SelectColumn {
		col := ast.SelectColumn{}
		if p.curToken.Type != lexer.IDENT {
			p.errors = append(p.errors, "expected identifier in column list")
			return col
		}

		// Check for table.* syntax
		if p.peekTokenIs(lexer.DOT) {
			tableQualifier := p.curToken.Literal
			p.nextToken() // consume table name
			p.nextToken() // consume .

			if p.curTokenIs(lexer.ASTERISK) {
				return ast.SelectColumn{Name: "*", TableQualifier: tableQualifier}
			} else if p.curTokenIs(lexer.IDENT) {
				return ast.SelectColumn{Name: p.curToken.Literal, TableQualifier: tableQualifier}
			} else {
				p.errors = append(p.errors, "expected '*' or identifier after 'table.'")
				return col
			}
		} else {
			return ast.SelectColumn{Name: p.curToken.Literal}
		}
	}

	list = append(list, parseCol())

	for p.peekTokenIs(lexer.COMMA) {
		p.nextToken()
		p.nextToken()
		list = append(list, parseCol())
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

	if p.peekTokenIs(lexer.SEMICOLON) {
		p.nextToken()
	}

	return stmt
}

func (p *Parser) parseExpressionList(end lexer.TokenType) []ast.Expression {
	list := []ast.Expression{}

	if p.peekTokenIs(end) {
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

	if !p.expectPeek(end) {
		return nil
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

	setClause := ast.UpdateClause{}
	if !p.expectPeek(lexer.IDENT) {
		p.errors = append(p.errors, "expected column name in SET clause")
		return nil
	}
	setClause.Column = p.curToken.Literal

	if !p.expectPeek(lexer.ASSIGN) {
		return nil
	}

	p.nextToken()
	setClause.Value = p.parseExpression(LOWEST)
	stmt.Set = setClause

	if p.peekTokenIs(lexer.WHERE) {
		p.nextToken()
		stmt.Where = p.parseWhereClause()
	} else {
		p.peekError(lexer.WHERE)
		return nil
	}

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
