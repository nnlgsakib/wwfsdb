package ssql

import (
	"fmt"

	"github.com/nnlgsakib/wwfsdb/pkg/ssql/ast"
	"github.com/nnlgsakib/wwfsdb/pkg/ssql/lexer"
)

// Type aliases re-exported at the facade level.
type (
	Column       = ast.Column
	Schema       = ast.Schema
	Expression   = ast.Expression
	UpdateClause = ast.UpdateClause
	Statement    = ast.Statement
)

// Parse is the main entry point for parsing a single SQL statement.
func Parse(sql string) (ast.Statement, error) {
	return parse(sql)
}

// ParseMultiple is for parsing a script containing multiple SQL statements.
func ParseMultiple(sql string) ([]ast.Statement, error) {
	return parseMultiple(sql)
}

// ParseWithFormatting parses SQL with formatting preservation
func ParseWithFormatting(sql string, keepComments, keepWhitespace bool) ([]ast.Statement, error) {
	l := lexer.NewWithFormatting(sql, keepComments, keepWhitespace)
	p := NewParser(l)
	stmts := p.ParseProgram()
	if len(p.errors) > 0 {
		return nil, fmt.Errorf("parser errors: %v", p.errors)
	}
	return stmts, nil
}
