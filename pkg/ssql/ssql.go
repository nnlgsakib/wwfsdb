package ssql

import (
	"github.com/nnlgsakib/wwfsdb/pkg/ssql/ast"
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
