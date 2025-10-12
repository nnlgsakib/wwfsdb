package ipfsdb

import (
	"fmt"

	"github.com/blastrain/vitess-sqlparser/sqlparser"
	pb "github.com/nnlgsakib/wwfsdb/pkg/ipfsdb/proto"
)

func extractSchemaFromDDL(stmt sqlparser.Statement) (*pb.Schema, error) {
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

func extractTableName(tableExpr sqlparser.TableExpr) (string, error) {
	aliased, ok := tableExpr.(*sqlparser.AliasedTableExpr)
	if !ok {
		return "", fmt.Errorf("unsupported table expression type: %T", tableExpr)
	}
	tableName, ok := aliased.Expr.(sqlparser.TableName)
	if !ok {
		return "", fmt.Errorf("unsupported table expression type: %T", aliased.Expr)
	}
	return tableName.Name.String(), nil
}
