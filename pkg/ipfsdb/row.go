package ipfsdb

import (
	"fmt"

	shell "github.com/ipfs/go-ipfs-api"
	pb "github.com/nnlgsakib/wwfsdb/pkg/ipfsdb/proto"
	"github.com/blastrain/vitess-sqlparser/sqlparser"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

// Insert adds a new row to a table
func Insert(ipfsAPI, dbName, tableName string, insert *sqlparser.Insert) error {
	sh := shell.NewShell(ipfsAPI)

	db, err := LoadDatabase(sh, dbName)
	if err != nil {
		return err
	}

	newDb, err := InsertDB(sh, db, tableName, insert)
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

// InsertDB adds new rows to an in-memory database object using paged storage.
func InsertDB(sh *shell.Shell, db *pb.Database, tableName string, insert *sqlparser.Insert) (*pb.Database, error) {
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

	values := insert.Rows.(sqlparser.Values)
	if len(values) == 0 {
		return nil, fmt.Errorf("no rows provided in INSERT statement")
	}

	// Handle column specification in INSERT
	// If insert.Columns is empty, all columns in schema order are expected
	var insertColumnNames []sqlparser.ColIdent
	if len(insert.Columns) > 0 {
		insertColumnNames = insert.Columns
	} else {
		// If no columns specified, expect all columns in schema order
		insertColumnNames = make([]sqlparser.ColIdent, len(schema.Columns))
		for i, col := range schema.Columns {
			insertColumnNames[i] = sqlparser.NewColIdent(col.Name)
		}
	}

	// Process all rows in the insert statement
	for _, rowTuple := range values {
		if len(rowTuple) != len(insertColumnNames) {
			return nil, fmt.Errorf("incorrect number of values for insert statement: got %d values for %d specified columns", len(rowTuple), len(insertColumnNames))
		}

		// Create the new row object
		row := &pb.Row{Values: make(map[string]*anypb.Any)}
		
		// Process each specified column and its corresponding value
		for i, colName := range insertColumnNames {
			colNameStr := colName.String()
			
			// Verify that the specified column exists in the schema
			var colInSchema *pb.Column
			for _, schemaCol := range schema.Columns {
				if schemaCol.Name == colNameStr {
					colInSchema = schemaCol
					break
				}
			}
			if colInSchema == nil {
				return nil, fmt.Errorf("column '%s' does not exist in table '%s'", colNameStr, tableName)
			}

			expr := rowTuple[i]
			lit, ok := expr.(*sqlparser.SQLVal)
			if !ok {
				return nil, fmt.Errorf("unsupported expression type in INSERT VALUES: %T", expr)
			}

			val, err := ValidateAndCastValue(string(lit.Val), colInSchema.Type)
			if err != nil {
				return nil, fmt.Errorf("validation error for column '%s': %w", colNameStr, err)
			}
			row.Values[colNameStr] = val
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
func Update(ipfsAPI, dbName, tableName string, update *sqlparser.Update) (int, error) {
	sh := shell.NewShell(ipfsAPI)

	db, err := LoadDatabase(sh, dbName)
	if err != nil {
		return 0, err
	}

	newDb, updatedCount, err := UpdateDB(sh, db, tableName, update)
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
func UpdateDB(sh *shell.Shell, db *pb.Database, tableName string, update *sqlparser.Update) (*pb.Database, int, error) {
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
	schemas := map[string]*pb.Schema{tableName: schema}

	// TODO: Handle multiple SET clauses
	if len(update.Exprs) != 1 {
		return nil, 0, fmt.Errorf("multiple SET clauses not yet supported")
	}
	setColumn := update.Exprs[0].Name.Name.String()
	setValue := update.Exprs[0].Expr

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

	lit, ok := setValue.(*sqlparser.SQLVal)
	if !ok {
		return nil, 0, fmt.Errorf("unsupported expression type in SET clause: %T", setValue)
	}
	valueStr := string(lit.Val)

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
			include, err := evaluateExpression(CombinedRow{tableName: row}, update.Where.Expr, schemas)
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
func Delete(ipfsAPI, dbName, tableName string, delete *sqlparser.Delete) (int, error) {
	sh := shell.NewShell(ipfsAPI)

	db, err := LoadDatabase(sh, dbName)
	if err != nil {
		return 0, err
	}

	newDb, deletedCount, err := DeleteDB(sh, db, tableName, delete)
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
func DeleteDB(sh *shell.Shell, db *pb.Database, tableName string, delete *sqlparser.Delete) (*pb.Database, int, error) {
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
	schemas := map[string]*pb.Schema{tableName: schema}

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
			include, err := evaluateExpression(CombinedRow{tableName: row}, delete.Where.Expr, schemas)
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
