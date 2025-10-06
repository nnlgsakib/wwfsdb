package ast

// Column represents a column in a table
type Column struct {
    Name string `json:"name"`
    Type string `json:"type"`
}

// Schema represents the schema of a table
type Schema struct {
    Columns []Column `json:"columns"`
}

// WhereClause represents a WHERE clause in a SQL statement
type WhereClause struct {
    Column string
    Value  string
}

// UpdateClause represents a SET clause in an UPDATE statement
type UpdateClause struct {
    Column string
    Value  string
}

// Node is a marker interface for AST nodes
type Node interface{ isNode() }

// Statement is a marker interface for SQL statements
type Statement interface{ Node; isStatement() }

// Basic statement node wrappers (placeholders for future rich AST)
type (
    CreateDatabaseStmt struct{ Name string }
    CreateTableStmt struct{ Name string; Schema Schema }
    DropTableStmt struct{ Name string }
    SelectStmt struct{ Table string; Where *WhereClause }
    InsertStmt struct{ Table string; Values []string }
    UpdateStmt struct{ Table string; Set UpdateClause; Where WhereClause }
    DeleteStmt struct{ Table string; Where WhereClause }
)

func (CreateDatabaseStmt) isNode() {}
func (CreateDatabaseStmt) isStatement() {}
func (CreateTableStmt) isNode() {}
func (CreateTableStmt) isStatement() {}
func (DropTableStmt) isNode() {}
func (DropTableStmt) isStatement() {}
func (SelectStmt) isNode() {}
func (SelectStmt) isStatement() {}
func (InsertStmt) isNode() {}
func (InsertStmt) isStatement() {}
func (UpdateStmt) isNode() {}
func (UpdateStmt) isStatement() {}
func (DeleteStmt) isNode() {}
func (DeleteStmt) isStatement() {}

