package sql

import (
	"fmt"

	"github.com/blastrain/vitess-sqlparser/sqlparser"
	pb "github.com/nnlgsakib/wwfsdb/pkg/ipfsdb/proto"
)

func ExtractSchemaFromDDL(stmt sqlparser.Statement) (*pb.Schema, error) {
	createTable, ok := stmt.(*sqlparser.CreateTable)
	if !ok {
		return nil, fmt.Errorf("not a create table statement")
	}

	schema := &pb.Schema{}
	for _, colDef := range createTable.Columns {
		col := &pb.Column{
			Name: colDef.Name,
			Type: colDef.Type,
		}
		for _, opt := range colDef.Options {
			switch opt.Type {
			case sqlparser.ColumnOptionNotNull:
				col.IsNotNull = true
			case sqlparser.ColumnOptionUniqKey:
				col.IsUnique = true
			}
		}
		schema.Columns = append(schema.Columns, col)
	}
	return schema, nil
}

func ExtractTableName(tableExpr sqlparser.TableExpr) (string, error) {
	switch expr := tableExpr.(type) {
	case *sqlparser.AliasedTableExpr:
		tableName, ok := expr.Expr.(sqlparser.TableName)
		if !ok {
			return "", fmt.Errorf("unsupported table expression type: %T", expr.Expr)
		}
		return tableName.Name.String(), nil
	case *sqlparser.JoinTableExpr:
		// For JOIN expressions, we need to handle both sides of the join
		// This is a simplified approach - in practice, you might need more complex handling
		leftName, err := ExtractTableName(expr.LeftExpr)
		if err != nil {
			return "", err
		}
		// We may also need the right table name depending on the context
		// For now, return the left table name as the primary table name for this function
		return leftName, nil
	default:
		return "", fmt.Errorf("unsupported table expression type: %T", tableExpr)
	}
}
