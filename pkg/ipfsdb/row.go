package ipfsdb

import (
	"bytes"
	"fmt"
	"strconv"

	shell "github.com/ipfs/go-ipfs-api"
	pb "github.com/nnlgsakib/wwfsdb/pkg/ipfsdb/proto"
	"github.com/nnlgsakib/wwfsdb/pkg/ssql/ast"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

// Insert adds a new row to a table
func Insert(ipfsAPI, dbName, tableName string, values []ast.Expression) error {
	sh := shell.NewShell(ipfsAPI)

	// 1. Load the database
	db, err := LoadDatabase(sh, dbName)
	if err != nil {
		return err
	}

	newDb, err := InsertDB(sh, db, tableName, values)
	if err != nil {
		return err
	}

	// 9. Update the database in IPFS
	newDbCID, err := AddObject(sh, newDb)
	if err != nil {
		return err
	}

	// 10. Update the cache
	UpdateCache(dbName, newDbCID)

	// 11. Update the IPNS record in the background
	PublishAsync(sh, dbName, newDbCID)
	return nil
}

// InsertDB adds a new row to an in-memory database object

func evaluateInsertExpression(expr ast.Expression) (interface{}, error) {
	switch e := expr.(type) {
	case *ast.Literal:
		return e.Value, nil
	case *ast.NumberLiteral:
		return e.Value, nil
	case *ast.BooleanLiteral:
		return e.Value, nil
	case *ast.PrefixExpression:
		right, err := evaluateInsertExpression(e.Right)
		if err != nil {
			return nil, err
		}

		if e.Operator == "-" {
			if v, ok := right.(float64); ok {
				return -v, nil
			}
			return nil, fmt.Errorf("unary minus operator can only be applied to numbers, got %T", right)
		}
		return nil, fmt.Errorf("unsupported prefix operator in INSERT: %s", e.Operator)
	default:
		return nil, fmt.Errorf("unsupported expression type in INSERT VALUES: %T", expr)
	}
}

func InsertDB(sh *shell.Shell, db *pb.Database, tableName string, values []ast.Expression) (*pb.Database, error) {
	newDb := proto.Clone(db).(*pb.Database)

	// 2. Get the table CID from the database
	tableCID, ok := newDb.Tables[tableName]
	if !ok {
		return nil, fmt.Errorf("table %s not found in database", tableName)
	}

	// 3. Load the table
	table, err := LoadTable(sh, tableCID)
	if err != nil {
		return nil, err
	}

	// 4. Create the new row
	row := &pb.Row{Values: make(map[string]*anypb.Any)}
	schema, err := LoadSchema(sh, table.SchemaCid)
	if err != nil {
		return nil, err
	}

	if len(values) != len(schema.Columns) {
		return nil, fmt.Errorf("incorrect number of values for insert statement")
	}

	for i, col := range schema.Columns {
		expr := values[i]

		rawValue, err := evaluateInsertExpression(expr)
		if err != nil {
			return nil, err
		}
		valueStr := fmt.Sprintf("%v", rawValue)

		// Validate and cast the value based on the column type
		val, err := ValidateAndCastValue(valueStr, col.Type)
		if err != nil {
			return nil, fmt.Errorf("validation error for column '%s': %w", col.Name, err)
		}
		row.Values[col.Name] = val
	}

	// 5. Add the new row to IPFS
	rowCID, err := AddObject(sh, row)
	if err != nil {
		return nil, err
	}

	// 6. Update the table with the new row CID
	table.Rows = append(table.Rows, rowCID)

	// 7. Update indexes
	table, err = UpdateIndexesOnInsert(sh, table, rowCID, row)
	if err != nil {
		return nil, fmt.Errorf("failed to update indexes on insert: %w", err)
	}

	// 7. Update the table in IPFS
	newTableCID, err := AddObject(sh, table)
	if err != nil {
		return nil, err
	}

	// 8. Update the database with the new table CID
	newDb.Tables[tableName] = newTableCID

	return newDb, nil
}

// Update modifies a row in a table
func Update(ipfsAPI, dbName, tableName, setColumn string, setValue ast.Expression, where ast.Expression) (int, error) {
	sh := shell.NewShell(ipfsAPI)

	// 1. Load the database
	db, err := LoadDatabase(sh, dbName)
	if err != nil {
		return 0, err
	}

	newDb, updatedCount, err := UpdateDB(sh, db, tableName, setColumn, setValue, where)
	if err != nil {
		return 0, err
	}

	if updatedCount == 0 {
		return 0, nil
	}

	// 12. Update the database in IPFS
	newDbCID, err := AddObject(sh, newDb)
	if err != nil {
		return 0, err
	}

	// 13. Update the cache
	UpdateCache(dbName, newDbCID)

	// 14. Update the IPNS record in the background
	PublishAsync(sh, dbName, newDbCID)
	return updatedCount, nil
}

// UpdateDB modifies a row in an in-memory database object
func UpdateDB(sh *shell.Shell, db *pb.Database, tableName, setColumn string, setValue ast.Expression, where ast.Expression) (*pb.Database, int, error) {
	newDb := proto.Clone(db).(*pb.Database)

	// 2. Get the table CID from the database
	tableCID, ok := newDb.Tables[tableName]
	if !ok {
		return nil, 0, fmt.Errorf("table %s not found in database", tableName)
	}

	// 3. Load the table
	table, err := LoadTable(sh, tableCID)
	if err != nil {
		return nil, 0, err
	}

	// 4. Load the schema to get column types
	schema, err := LoadSchema(sh, table.SchemaCid)
	if err != nil {
		return nil, 0, err
	}

	var columnType string
	for _, col := range schema.Columns {
		if col.Name == setColumn {
			columnType = col.Type
			break
		}
	}

	if columnType == "" {
		return nil, 0, fmt.Errorf("column '%s' not found in table '%s'", setColumn, tableName)
	}

	// 5. Validate and cast the new value
	var valueStr string
	switch v := setValue.(type) {
	case *ast.Literal:
		valueStr = v.Value
	case *ast.NumberLiteral:
		valueStr = strconv.FormatFloat(v.Value, 'f', -1, 64)
	case *ast.BooleanLiteral:
		valueStr = strconv.FormatBool(v.Value)
	default:
		return nil, 0, fmt.Errorf("unsupported expression type in SET clause: %T", setValue)
	}

	castedValue, err := ValidateAndCastValue(valueStr, columnType)
	if err != nil {
		return nil, 0, fmt.Errorf("validation error for column '%s': %w", setColumn, err)
	}

	// 6. Find and update rows
			var updatedCount int
			for i, rowCID := range table.Rows {
				row, err := LoadRow(sh, rowCID)
				if err != nil {
					return nil, 0, err
				}
	
				include, err := evaluateExpression(CombinedRow{tableName: row}, where)
				if err != nil {
					return nil, 0, err
				}
	
				if include {			// Capture old state for index update
			oldRow := proto.Clone(row).(*pb.Row)

			// First, update indexes as if the old row is being deleted
			table, err = UpdateIndexesOnDelete(sh, table, rowCID, oldRow)
			if err != nil {
				return nil, 0, fmt.Errorf("failed to update indexes on delete part of update: %w", err)
			}

			// Update the row data with the new value
			row.Values[setColumn] = castedValue

			// Add the updated row to IPFS to get a new CID
			newRowCID, err := AddObject(sh, row)
			if err != nil {
				return nil, 0, err
			}

			// Now, update indexes as if the new row is being inserted
			table, err = UpdateIndexesOnInsert(sh, table, newRowCID, row)
			if err != nil {
				return nil, 0, fmt.Errorf("failed to update indexes on insert part of update: %w", err)
			}

			// Update the table with the new row CID
			table.Rows[i] = newRowCID
			updatedCount++
		}
	}

	if updatedCount == 0 {
		return nil, 0, nil
	}

	// 10. Update the table in IPFS
	newTableCID, err := AddObject(sh, table)
	if err != nil {
		return nil, 0, err
	}

	// 11. Update the database with the new table CID
	newDb.Tables[tableName] = newTableCID

	return newDb, updatedCount, nil
}

// Delete removes a row from a table
func Delete(ipfsAPI, dbName, tableName string, where ast.Expression) (int, error) {
	sh := shell.NewShell(ipfsAPI)

	// 1. Load the database
	db, err := LoadDatabase(sh, dbName)
	if err != nil {
		return 0, err
	}

	newDb, deletedCount, err := DeleteDB(sh, db, tableName, where)
	if err != nil {
		return 0, err
	}

	if deletedCount == 0 {
		return 0, nil
	}

	// 8. Update the database in IPFS
	newDbCID, err := AddObject(sh, newDb)
	if err != nil {
		return 0, err
	}

	// 9. Update the cache
	UpdateCache(dbName, newDbCID)

	// 10. Update the IPNS record in the background
	PublishAsync(sh, dbName, newDbCID)
	return deletedCount, nil
}

// DeleteDB removes a row from an in-memory database object
func DeleteDB(sh *shell.Shell, db *pb.Database, tableName string, where ast.Expression) (*pb.Database, int, error) {
	newDb := proto.Clone(db).(*pb.Database)

	// 2. Get the table CID from the database
	tableCID, ok := newDb.Tables[tableName]
	if !ok {
		return nil, 0, fmt.Errorf("table %s not found in database", tableName)
	}

	// 3. Load the table
	table, err := LoadTable(sh, tableCID)
	if err != nil {
		return nil, 0, err
	}

	// 4. Find the row to delete
	var newRows []string
	var deletedCount int
	for _, rowCID := range table.Rows {
		row, err := LoadRow(sh, rowCID)
		if err != nil {
			return nil, 0, err
		}

		include, err := evaluateExpression(CombinedRow{tableName: row}, where)
		if err != nil {
			return nil, 0, err
		}

		if include {
			deletedCount++
			// Update indexes before deleting the row
			table, err = UpdateIndexesOnDelete(sh, table, rowCID, row)
			if err != nil {
				return nil, 0, fmt.Errorf("failed to update indexes on delete: %w", err)
			}
		} else {
			newRows = append(newRows, rowCID)
		}
	}

	if deletedCount == 0 {
		return newDb, 0, nil
	}

	// 5. Update the table with the new row list
	table.Rows = newRows

	// 6. Update the table in IPFS
	newTableCID, err := AddObject(sh, table)
	if err != nil {
		return nil, 0, err
	}

	// 7. Update the database with the new table CID
	newDb.Tables[tableName] = newTableCID

	return newDb, deletedCount, nil
}

// LoadRow loads a row from IPFS
func LoadRow(sh *shell.Shell, rowCID string) (*pb.Row, error) {
	data, err := sh.Cat(rowCID)
	if err != nil {
		return nil, err
	}
	defer data.Close()

	buf := new(bytes.Buffer)
	buf.ReadFrom(data)

	var row pb.Row
	if err := proto.Unmarshal(buf.Bytes(), &row); err != nil {
		return nil, err
	}

	return &row, nil
}
