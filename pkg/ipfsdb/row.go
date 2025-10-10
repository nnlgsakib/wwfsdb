package ipfsdb

import (
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

	db, err := LoadDatabase(sh, dbName)
	if err != nil {
		return err
	}

	newDb, err := InsertDB(sh, db, tableName, values)
	if err != nil {
		return err
	}

	newDbCID, err := AddObject(sh, newDb)
	if err != nil {
		return err
	}

	UpdateCache(dbName, newDbCID)
	PublishAsync(sh, dbName, newDbCID)
	return nil
}

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

// InsertDB adds a new row to an in-memory database object using paged storage.
func InsertDB(sh *shell.Shell, db *pb.Database, tableName string, values []ast.Expression) (*pb.Database, error) {
	newDb := proto.Clone(db).(*pb.Database)

	tableCID, ok := newDb.Tables[tableName]
	if !ok {
		return nil, fmt.Errorf("table %s not found in database", tableName)
	}

	table, err := LoadTable(sh, tableCID)
	if err != nil {
		return nil, err
	}

	schema, err := LoadSchema(sh, table.SchemaCid)
	if err != nil {
		return nil, err
	}

	if len(values) != len(schema.Columns) {
		return nil, fmt.Errorf("incorrect number of values for insert statement")
	}

	// Create the new row object
	row := &pb.Row{Values: make(map[string]*anypb.Any)}
	for i, col := range schema.Columns {
		expr := values[i]
		rawValue, err := evaluateInsertExpression(expr)
		if err != nil {
			return nil, err
		}
		valueStr := fmt.Sprintf("%v", rawValue)
		val, err := ValidateAndCastValue(valueStr, col.Type)
		if err != nil {
			return nil, fmt.Errorf("validation error for column '%s': %w", col.Name, err)
		}
		row.Values[col.Name] = val
	}

	var lastPage *pb.Page
	var lastPageCID string
	isNewPage := false

	if len(table.PageCids) > 0 {
		lastPageCID = table.PageCids[len(table.PageCids)-1]
		lastPage, err = LoadPage(sh, lastPageCID)
		if err != nil {
			return nil, fmt.Errorf("failed to load last page %s: %w", lastPageCID, err)
		}
	}

	if lastPage == nil || len(lastPage.Rows) >= MaxRowsPerPage {
		lastPage = &pb.Page{Rows: []*pb.Row{}}
		isNewPage = true
	}

	// Add the new row to the page
	lastPage.Rows = append(lastPage.Rows, row)

	// Save the updated page to get its new CID
	newPageCID, err := AddObject(sh, lastPage)
	if err != nil {
		return nil, err
	}

	// Update indexes with the new row data and page CID
	table, err = UpdateIndexesOnInsert(sh, table, newPageCID, row)
	if err != nil {
		return nil, fmt.Errorf("failed to update indexes on insert: %w", err)
	}

	// Update the table's page list
	if isNewPage {
		table.PageCids = append(table.PageCids, newPageCID)
	} else {
		table.PageCids[len(table.PageCids)-1] = newPageCID
	}

	// Save the final table structure
	newTableCID, err := AddObject(sh, table)
	if err != nil {
		return nil, err
	}

	newDb.Tables[tableName] = newTableCID
	return newDb, nil
}

// Update modifies rows in a table
func Update(ipfsAPI, dbName, tableName, setColumn string, setValue ast.Expression, where ast.Expression) (int, error) {
	sh := shell.NewShell(ipfsAPI)

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

	newDbCID, err := AddObject(sh, newDb)
	if err != nil {
		return 0, err
	}

	UpdateCache(dbName, newDbCID)
	PublishAsync(sh, dbName, newDbCID)
	return updatedCount, nil
}

// UpdateDB modifies rows in an in-memory database object using paged storage.
func UpdateDB(sh *shell.Shell, db *pb.Database, tableName, setColumn string, setValue ast.Expression, where ast.Expression) (*pb.Database, int, error) {
	newDb := proto.Clone(db).(*pb.Database)

	tableCID, ok := newDb.Tables[tableName]
	if !ok {
		return nil, 0, fmt.Errorf("table %s not found in database", tableName)
	}

	table, err := LoadTable(sh, tableCID)
	if err != nil {
		return nil, 0, err
	}

	schema, err := LoadSchema(sh, table.SchemaCid)
	if err != nil {
		return nil, 0, err
	}

	var columnType string
	columnExists := false
	for _, col := range schema.Columns {
		if col.Name == setColumn {
			columnType = col.Type
			columnExists = true
			break
		}
	}
	if !columnExists {
		return nil, 0, fmt.Errorf("column '%s' not found in table '%s'", setColumn, tableName)
	}

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

	var updatedCount int
	for i, pageCID := range table.PageCids {
		page, err := LoadPage(sh, pageCID)
		if err != nil {
			return nil, 0, err
		}

		pageModified := false
		for _, row := range page.Rows {
			include, err := evaluateExpression(CombinedRow{tableName: row}, where)
			if err != nil {
				return nil, 0, err
			}

			if include {
				// Capture old row state for index update
				oldRow := proto.Clone(row).(*pb.Row)

				// Update the row data with the new value
				row.Values[setColumn] = castedValue
				pageModified = true
				updatedCount++

				// Treat update as a delete then an insert for indexing purposes
				table, err = UpdateIndexesOnDelete(sh, table, pageCID, page, oldRow)
				if err != nil {
					return nil, 0, fmt.Errorf("failed to update indexes on delete part of update: %w", err)
				}
				table, err = UpdateIndexesOnInsert(sh, table, pageCID, row)
				if err != nil {
					return nil, 0, fmt.Errorf("failed to update indexes on insert part of update: %w", err)
				}
			}
		}

		if pageModified {
			newPageCID, err := AddObject(sh, page)
			if err != nil {
				return nil, 0, err
			}
			table.PageCids[i] = newPageCID
		}
	}

	if updatedCount == 0 {
		return newDb, 0, nil
	}

	newTableCID, err := AddObject(sh, table)
	if err != nil {
		return nil, 0, err
	}

	newDb.Tables[tableName] = newTableCID
	return newDb, updatedCount, nil
}

// Delete removes rows from a table
func Delete(ipfsAPI, dbName, tableName string, where ast.Expression) (int, error) {
	sh := shell.NewShell(ipfsAPI)

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

	newDbCID, err := AddObject(sh, newDb)
	if err != nil {
		return 0, err
	}

	UpdateCache(dbName, newDbCID)
	PublishAsync(sh, dbName, newDbCID)
	return deletedCount, nil
}

// DeleteDB removes rows from an in-memory database object using paged storage.
func DeleteDB(sh *shell.Shell, db *pb.Database, tableName string, where ast.Expression) (*pb.Database, int, error) {
	newDb := proto.Clone(db).(*pb.Database)

	tableCID, ok := newDb.Tables[tableName]
	if !ok {
		return nil, 0, fmt.Errorf("table %s not found in database", tableName)
	}

	table, err := LoadTable(sh, tableCID)
	if err != nil {
		return nil, 0, err
	}

	var deletedCount int
	var newPageCIDs []string
	for _, pageCID := range table.PageCids {
		page, err := LoadPage(sh, pageCID)
		if err != nil {
			return nil, 0, err
		}

		var newRows []*pb.Row
		pageModified := false
		for _, row := range page.Rows {
			include, err := evaluateExpression(CombinedRow{tableName: row}, where)
			if err != nil {
				return nil, 0, err
			}

			if include {
				deletedCount++
				pageModified = true
				// Update indexes before "deleting" the row from its page
				table, err = UpdateIndexesOnDelete(sh, table, pageCID, page, row)
				if err != nil {
					return nil, 0, fmt.Errorf("failed to update indexes on delete: %w", err)
				}
			} else {
				newRows = append(newRows, row)
			}
		}

		if pageModified {
			if len(newRows) > 0 {
				page.Rows = newRows
				newPageCID, err := AddObject(sh, page)
				if err != nil {
					return nil, 0, err
				}
				newPageCIDs = append(newPageCIDs, newPageCID)
			}
			// If the page is now empty, we simply don't add it to the new list of page CIDs.
		} else {
			newPageCIDs = append(newPageCIDs, pageCID) // Page was not modified, keep original CID.
		}
	}

	if deletedCount == 0 {
		return newDb, 0, nil
	}

	table.PageCids = newPageCIDs
	newTableCID, err := AddObject(sh, table)
	if err != nil {
		return nil, 0, err
	}

	newDb.Tables[tableName] = newTableCID
	return newDb, deletedCount, nil
}
