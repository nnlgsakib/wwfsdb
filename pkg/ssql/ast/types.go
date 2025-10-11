package ast

import "github.com/nnlgsakib/wwfsdb/pkg/ssql/lexer"

// Expression represents a node in the expression tree.

type Expression interface {
	isExpression()
}

// Comment represents a comment in the source code
type Comment struct {
	Value    string
	Position lexer.Position
}

// NodeWithComments represents a node that can have comments associated with it
type NodeWithComments struct {
	LeadingComments  []Comment
	TrailingComments []Comment
}

// BinaryExpr represents a binary operation (e.g., AND, OR).

type BinaryExpr struct {
	Left     Expression
	Operator string
	Right    Expression
	Comments NodeWithComments
}

// ComparisonExpr represents a comparison operation (e.g., =, >, <).

type ComparisonExpr struct {
	Left     Expression
	Operator string
	Right    Expression
	Comments NodeWithComments
}

// LikeExpr represents a LIKE expression.

type LikeExpr struct {
	Left     Expression
	Pattern  Expression
	Comments NodeWithComments
}

// InExpr represents an IN expression.

type InExpr struct {
	Left     Expression
	Values   []Expression
	Comments NodeWithComments
}

// PrefixExpression represents a unary operation (e.g., -5).
type PrefixExpression struct {
	Operator string
	Right    Expression
	Comments NodeWithComments
}

// Literal represents a string or number literal.

type Literal struct {
	Value    string
	Comments NodeWithComments
}

// NumberLiteral represents a numeric literal.

type NumberLiteral struct {
	Value    float64
	Comments NodeWithComments
}

// BooleanLiteral represents a boolean literal.

type BooleanLiteral struct {
	Value    bool
	Comments NodeWithComments
}

// Identifier represents a column name, possibly qualified with a table name.

type Identifier struct {
	Name           string
	TableQualifier string
	Comments       NodeWithComments
}

func (BinaryExpr) isExpression() {}

func (ComparisonExpr) isExpression() {}

func (LikeExpr) isExpression() {}

func (InExpr) isExpression() {}

func (PrefixExpression) isExpression() {}

func (Literal) isExpression() {}

func (NumberLiteral) isExpression() {}

func (BooleanLiteral) isExpression() {}

func (Identifier) isExpression() {}

// FromClause represents a source of rows for a SELECT statement (a table or a join).

type FromClause interface {
	isFromClause()
}

// TableIdentifier represents a single table in a FROM clause.

type TableIdentifier struct {
	Name     string
	Comments NodeWithComments
}

// JoinClause represents an INNER or LEFT JOIN.

type JoinClause struct {
	Type string // "INNER" or "LEFT"

	Left FromClause

	Right FromClause

	On Expression
	Comments NodeWithComments
}

func (TableIdentifier) isFromClause() {}

func (JoinClause) isFromClause() {}

// Column represents a column in a table

type Column struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Comments NodeWithComments
}

// Schema represents the schema of a table

type Schema struct {
	Columns  []Column `json:"columns"`
	Comments NodeWithComments
}

// UpdateClause represents a SET clause in an UPDATE statement

type UpdateClause struct {
	Column   string
	Value    Expression
	Comments NodeWithComments
}

// AlterTableAction represents a sub-command within an ALTER TABLE statement.

type AlterTableAction interface {
	isAlterTableAction()
}

// AddColumnClause represents an ADD COLUMN action.

type AddColumnClause struct {
	Column   Column
	Comments NodeWithComments
}

// DropColumnClause represents a DROP COLUMN action.

type DropColumnClause struct {
	ColumnName string
	Comments   NodeWithComments
}

// RenameColumnClause represents a RENAME COLUMN action.

type RenameColumnClause struct {
	OldName  string
	NewName  string
	Comments NodeWithComments
}

func (AddColumnClause) isAlterTableAction() {}

func (DropColumnClause) isAlterTableAction() {}

func (RenameColumnClause) isAlterTableAction() {}

// Node is a marker interface for AST nodes

type Node interface{ isNode() }

// Statement is a marker interface for SQL statements

type Statement interface {
	Node

	isStatement()
}

// SelectExpr represents an item in a SELECT clause, which can be a column, a wildcard, or a function call.

type SelectExpr interface {
	isSelectExpr()
}

// ColumnExpr represents a standard column selection, e.g., `id` or `users.name`.

type ColumnExpr struct {
	Name           string
	TableQualifier string
	Comments       NodeWithComments
}

// StarExpr represents a `*` selection.

type StarExpr struct {
	Comments NodeWithComments
}

// AggregateFunctionExpr represents an aggregate function call, e.g., `COUNT(*)` or `SUM(price)`.

type AggregateFunctionExpr struct {
	Name     string
	Argument Expression // Can be an Identifier or a StarExpr (represented as an Identifier with Name: "*")
	Comments NodeWithComments
}

func (ColumnExpr) isSelectExpr() {}

func (StarExpr) isSelectExpr() {}

func (AggregateFunctionExpr) isSelectExpr() {}

func (AggregateFunctionExpr) isExpression() {}

// OrderByExpression represents a column and direction in an ORDER BY clause.

type OrderByExpression struct {
	Column    Expression // e.g., an Identifier
	Direction string // "ASC" or "DESC"
	Comments  NodeWithComments
}

// Basic statement node wrappers (placeholders for future rich AST)

type (
	CreateDatabaseStmt struct {
		Name     string
		Comments NodeWithComments
	}

	CreateTableStmt struct {
		Name     string
		Schema   Schema
		Comments NodeWithComments
	}

	AlterTableStmt struct {
		Table    string
		Action   AlterTableAction
		Comments NodeWithComments
	}

	DropTableStmt struct {
		Name     string
		Comments NodeWithComments
	}

	SelectStmt struct {
		Columns []SelectExpr
		From    FromClause
		Where   Expression
		GroupBy []Expression
		OrderBy []*OrderByExpression
		Limit   Expression
		Offset  Expression
		Comments NodeWithComments
	}

	InsertStmt struct {
		Table  string
		Values []Expression
		Comments NodeWithComments
	}

	UpdateStmt struct {
		Table  string
		Set    UpdateClause
		Where  Expression
		Comments NodeWithComments
	}

	DeleteStmt struct {
		Table  string
		Where  Expression
		Comments NodeWithComments
	}

	CreateIndexStmt struct {
		Table    string
		Column   string
		Comments NodeWithComments
	}

	BeginStmt struct {
		Comments NodeWithComments
	}

	CommitStmt struct {
		Comments NodeWithComments
	}

	RollbackStmt struct {
		Comments NodeWithComments
	}
)

func (CreateDatabaseStmt) isNode() {}

func (CreateDatabaseStmt) isStatement() {}

func (CreateTableStmt) isNode() {}

func (CreateTableStmt) isStatement() {}

func (AlterTableStmt) isNode() {}

func (AlterTableStmt) isStatement() {}

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

func (CreateIndexStmt) isNode() {}

func (CreateIndexStmt) isStatement() {}

func (BeginStmt) isNode() {}

func (BeginStmt) isStatement() {}

func (CommitStmt) isNode() {}

func (CommitStmt) isStatement() {}

func (RollbackStmt) isNode() {}

func (RollbackStmt) isStatement() {}
