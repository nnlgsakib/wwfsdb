package ast

// Expression represents a node in the expression tree.	

type Expression interface {
	isExpression()
}

// BinaryExpr represents a binary operation (e.g., AND, OR).	

type BinaryExpr struct {
	Left     Expression
	Operator string
	Right    Expression
}

// ComparisonExpr represents a comparison operation (e.g., =, >, <).	

type ComparisonExpr struct {
	Left     Expression
	Operator string
	Right    Expression
}

// LikeExpr represents a LIKE expression.	

type LikeExpr struct {
	Left    Expression
	Pattern Expression
}

// InExpr represents an IN expression.	

type InExpr struct {
	Left   Expression
	Values []Expression
}

// Literal represents a string or number literal.	

type Literal struct {
	Value string
}

// NumberLiteral represents a numeric literal.

type NumberLiteral struct {
	Value float64
}

// BooleanLiteral represents a boolean literal.

type BooleanLiteral struct {
	Value bool
}

// Identifier represents a column name.	

type Identifier struct {
	Name string
}

func (BinaryExpr) isExpression()     {}
func (ComparisonExpr) isExpression() {}
func (LikeExpr) isExpression()       {}
func (InExpr) isExpression()         {}
func (Literal) isExpression()        {}
func (NumberLiteral) isExpression()  {}
func (BooleanLiteral) isExpression() {}
func (Identifier) isExpression()      {}

// Column represents a column in a table
type Column struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// Schema represents the schema of a table
type Schema struct {
	Columns []Column `json:"columns"`
}

// UpdateClause represents a SET clause in an UPDATE statement
type UpdateClause struct {
	Column string
	Value  Expression
}

// Node is a marker interface for AST nodes
type Node interface{ isNode() }

// Statement is a marker interface for SQL statements
type Statement interface {
	Node
	isStatement()
}

// Basic statement node wrappers (placeholders for future rich AST)
type (
	CreateDatabaseStmt struct{ Name string }
	CreateTableStmt    struct{ Name string; Schema Schema }
	DropTableStmt      struct{ Name string }
	SelectStmt         struct {
		Table   string
		Columns []string // New field for selected columns
		Where   Expression
	}
	InsertStmt struct{ Table string; Values []Expression }
	UpdateStmt struct{ Table string; Set UpdateClause; Where Expression }
	DeleteStmt struct{ Table string; Where Expression }
	CreateIndexStmt struct{ Table string; Column string }
)

func (CreateDatabaseStmt) isNode() {}
func (CreateDatabaseStmt) isStatement() {}
func (CreateTableStmt) isNode()    {}
func (CreateTableStmt) isStatement() {}
func (DropTableStmt) isNode()      {}
func (DropTableStmt) isStatement() {}
func (SelectStmt) isNode()        {}
func (SelectStmt) isStatement()     {}
func (InsertStmt) isNode()        {}
func (InsertStmt) isStatement()     {}
func (UpdateStmt) isNode()        {}
func (UpdateStmt) isStatement()     {}
func (DeleteStmt) isNode()        {}
func (DeleteStmt) isStatement()     {}
func (CreateIndexStmt) isNode() {}
func (CreateIndexStmt) isStatement() {}


